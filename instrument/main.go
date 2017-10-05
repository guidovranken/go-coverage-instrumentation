/* Per-file Go coverage instrumentation

   Portions taken from https://github.com/dvyukov/go-fuzz
*/

package main

import (
	"crypto/sha1"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
    "bytes"
    "fmt"
    "os"
    "io/ioutil"
    "path"
    "path/filepath"
)

var counterGen uint32

type CoverBlock struct {
	ID        int
	File      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	NumStmt   int
}

type File struct {
	fset      *token.FileSet
	astFile   *ast.File
}

func failf(str string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, str+"\n", args...)
	os.Exit(1)
}

func unquote(s string) string {
	t, err := strconv.Unquote(s)
	if err != nil {
		failf("cover: improperly quoted string %q\n", s)
	}
	return t
}

func trimComments(file *ast.File, fset *token.FileSet) []*ast.CommentGroup {
	var comments []*ast.CommentGroup
	for _, group := range file.Comments {
		var list []*ast.Comment
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:") && fset.Position(comment.Slash).Column == 1 {
				list = append(list, comment)
			}
		}
		if list != nil {
			comments = append(comments, &ast.CommentGroup{List: list})
		}
	}
	return comments
}

type Literal struct {
	Val   string
	IsStr bool
}

type LiteralCollector struct {
	lits map[Literal]struct{}
}

func genCounter() int {
	counterGen++
	id := counterGen
	buf := []byte{byte(id), byte(id >> 8), byte(id >> 16), byte(id >> 24)}
	hash := sha1.Sum(buf)
	return int(uint16(hash[0]) | uint16(hash[1])<<8)
}

func (f *File) newCounter(start, end token.Pos, numStmt int) ast.Stmt {
	cnt := genCounter()

    /*
	if f.blocks != nil {
		s := f.fset.Position(start)
		e := f.fset.Position(end)
		*f.blocks = append(*f.blocks, CoverBlock{cnt, f.fullName, s.Line, s.Column, e.Line, e.Column, numStmt})
	}
    */

	idx := &ast.BasicLit{
		Kind:  token.INT,
		Value: strconv.Itoa(cnt),
	}
    return &ast.ExprStmt{
        X : &ast.CallExpr{
            Fun: &ast.SelectorExpr{
                X:   ast.NewIdent("fuzz_helper"),
                Sel: ast.NewIdent("AddCoverage"),
            },
            Args: []ast.Expr { idx },
        },
    }
}

func (f *File) addCounters(pos, blockEnd token.Pos, list []ast.Stmt, extendToClosingBrace bool) []ast.Stmt {
	// Special case: make sure we add a counter to an empty block. Can't do this below
	// or we will add a counter to an empty statement list after, say, a return statement.
	if len(list) == 0 {
		return []ast.Stmt{f.newCounter(pos, blockEnd, 0)}
	}
	// We have a block (statement list), but it may have several basic blocks due to the
	// appearance of statements that affect the flow of control.
	var newList []ast.Stmt
	for {
		// Find first statement that affects flow of control (break, continue, if, etc.).
		// It will be the last statement of this basic block.
		var last int
		end := blockEnd
		for last = 0; last < len(list); last++ {
			end = f.statementBoundary(list[last])
			if f.endsBasicSourceBlock(list[last]) {
				extendToClosingBrace = false // Block is broken up now.
				last++
				break
			}
		}
		if extendToClosingBrace {
			end = blockEnd
		}
		if pos != end { // Can have no source to cover if e.g. blocks abut.
			newList = append(newList, f.newCounter(pos, end, last))
		}
		newList = append(newList, list[0:last]...)
		list = list[last:]
		if len(list) == 0 {
			break
		}
		pos = list[0].Pos()
	}
	return newList
}

func (f *File) endsBasicSourceBlock(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.BlockStmt:
		// Treat blocks like basic blocks to avoid overlapping counters.
		return true
	case *ast.BranchStmt:
		return true
	case *ast.ForStmt:
		return true
	case *ast.IfStmt:
		return true
	case *ast.LabeledStmt:
		return f.endsBasicSourceBlock(s.Stmt)
	case *ast.RangeStmt:
		return true
	case *ast.SwitchStmt:
		return true
	case *ast.SelectStmt:
		return true
	case *ast.TypeSwitchStmt:
		return true
	case *ast.ExprStmt:
		// Calls to panic change the flow.
		// We really should verify that "panic" is the predefined function,
		// but without type checking we can't and the likelihood of it being
		// an actual problem is vanishingly small.
		if call, ok := s.X.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "panic" && len(call.Args) == 1 {
				return true
			}
		}
	}
	found, _ := hasFuncLiteral(s)
	return found
}

func (f *File) statementBoundary(s ast.Stmt) token.Pos {
	// Control flow statements are easy.
	switch s := s.(type) {
	case *ast.BlockStmt:
		// Treat blocks like basic blocks to avoid overlapping counters.
		return s.Lbrace
	case *ast.IfStmt:
		found, pos := hasFuncLiteral(s.Init)
		if found {
			return pos
		}
		found, pos = hasFuncLiteral(s.Cond)
		if found {
			return pos
		}
		return s.Body.Lbrace
	case *ast.ForStmt:
		found, pos := hasFuncLiteral(s.Init)
		if found {
			return pos
		}
		found, pos = hasFuncLiteral(s.Cond)
		if found {
			return pos
		}
		found, pos = hasFuncLiteral(s.Post)
		if found {
			return pos
		}
		return s.Body.Lbrace
	case *ast.LabeledStmt:
		return f.statementBoundary(s.Stmt)
	case *ast.RangeStmt:
		found, pos := hasFuncLiteral(s.X)
		if found {
			return pos
		}
		return s.Body.Lbrace
	case *ast.SwitchStmt:
		found, pos := hasFuncLiteral(s.Init)
		if found {
			return pos
		}
		found, pos = hasFuncLiteral(s.Tag)
		if found {
			return pos
		}
		return s.Body.Lbrace
	case *ast.SelectStmt:
		return s.Body.Lbrace
	case *ast.TypeSwitchStmt:
		found, pos := hasFuncLiteral(s.Init)
		if found {
			return pos
		}
		return s.Body.Lbrace
	}
	found, pos := hasFuncLiteral(s)
	if found {
		return pos
	}
	return s.End()
}

type funcLitFinder token.Pos

func (f *funcLitFinder) Visit(node ast.Node) (w ast.Visitor) {
	if f.found() {
		return nil // Prune search.
	}
	switch n := node.(type) {
	case *ast.FuncLit:
		*f = funcLitFinder(n.Body.Lbrace)
		return nil // Prune search.
	}
	return f
}

func (f *funcLitFinder) found() bool {
	return token.Pos(*f) != token.NoPos
}
func hasFuncLiteral(n ast.Node) (bool, token.Pos) {
	if n == nil {
		return false, 0
	}
	var literal funcLitFinder
	ast.Walk(&literal, n)
	return literal.found(), token.Pos(literal)
}
func (f *File) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.GenDecl:
		if n.Tok != token.VAR {
			return nil // constants and types are not interesting
		}

	case *ast.BlockStmt:
		// If it's a switch or select, the body is a list of case clauses; don't tag the block itself.
		if len(n.List) > 0 {
			switch n.List[0].(type) {
			case *ast.CaseClause: // switch
				for _, n := range n.List {
					clause := n.(*ast.CaseClause)
					clause.Body = f.addCounters(clause.Pos(), clause.End(), clause.Body, false)
				}
				return f
			case *ast.CommClause: // select
				for _, n := range n.List {
					clause := n.(*ast.CommClause)
					clause.Body = f.addCounters(clause.Pos(), clause.End(), clause.Body, false)
				}
				return f
			}
		}
		n.List = f.addCounters(n.Lbrace, n.Rbrace+1, n.List, true) // +1 to step past closing brace.
	case *ast.IfStmt:
		if n.Init != nil {
			ast.Walk(f, n.Init)
		}
		if n.Cond != nil {
			ast.Walk(f, n.Cond)
		}
		ast.Walk(f, n.Body)
		if n.Else == nil {
			// Add else because we want coverage for "not taken".
			n.Else = &ast.BlockStmt{
				Lbrace: n.Body.End(),
				Rbrace: n.Body.End(),
			}
		}
		// The elses are special, because if we have
		//	if x {
		//	} else if y {
		//	}
		// we want to cover the "if y". To do this, we need a place to drop the counter,
		// so we add a hidden block:
		//	if x {
		//	} else {
		//		if y {
		//		}
		//	}
		switch stmt := n.Else.(type) {
		case *ast.IfStmt:
			block := &ast.BlockStmt{
				Lbrace: n.Body.End(), // Start at end of the "if" block so the covered part looks like it starts at the "else".
				List:   []ast.Stmt{stmt},
				Rbrace: stmt.End(),
			}
			n.Else = block
		case *ast.BlockStmt:
			stmt.Lbrace = n.Body.End() // Start at end of the "if" block so the covered part looks like it starts at the "else".
		default:
			panic("unexpected node type in if")
		}
		ast.Walk(f, n.Else)
		return nil
	case *ast.ForStmt:
		// TODO: handle increment statement
	case *ast.SelectStmt:
		// Don't annotate an empty select - creates a syntax error.
		if n.Body == nil || len(n.Body.List) == 0 {
			return nil
		}
	case *ast.SwitchStmt:
		hasDefault := false
		if n.Body == nil {
			n.Body = new(ast.BlockStmt)
		}
		for _, s := range n.Body.List {
			if cas, ok := s.(*ast.CaseClause); ok && cas.List == nil {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			// Add default case to get additional coverage.
			n.Body.List = append(n.Body.List, &ast.CaseClause{})
		}

		// Don't annotate an empty switch - creates a syntax error.
		if n.Body == nil || len(n.Body.List) == 0 {
			return nil
		}
	case *ast.TypeSwitchStmt:
		// Don't annotate an empty type switch - creates a syntax error.
		// TODO: add default case
		if n.Body == nil || len(n.Body.List) == 0 {
			return nil
		}
	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			// Replace:
			//	x && y
			// with:
			//	x && func() bool { return y }

            /*
            Temporarily disabled

			typ := f.info.Types[n].Type.String()
			if strings.HasPrefix(typ, f.pkg+".") {
				typ = typ[len(f.pkg)+1:]
			}
			if typ == "untyped bool" {
				typ = "bool"
			}
			n.Y = &ast.CallExpr{
				Fun: &ast.FuncLit{
					Type: &ast.FuncType{Results: &ast.FieldList{List: []*ast.Field{{Type: &ast.Ident{Name: typ}}}}},
					Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{n.Y}}}},
				},
			}
            */
		}
	}
	return f
}

func (f *File) addImport(path, name, anyIdent string) {
	newImport := &ast.ImportSpec{
		Name: ast.NewIdent(name),
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: fmt.Sprintf("%q", path),
		},
	}
	impDecl := &ast.GenDecl{
		Tok: token.IMPORT,
		Specs: []ast.Spec{
			newImport,
		},
	}
	// Make the new import the first Decl in the file.
	astFile := f.astFile
	astFile.Decls = append(astFile.Decls, nil)
	copy(astFile.Decls[1:], astFile.Decls[0:])
	astFile.Decls[0] = impDecl
	astFile.Imports = append(astFile.Imports, newImport)

	// Now refer to the package, just in case it ends up unused.
	// That is, append to the end of the file the declaration
	//	var _ = _cover_atomic_.AddUint32
	reference := &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{
					ast.NewIdent("_"),
				},
				Values: []ast.Expr{
					&ast.SelectorExpr{
						X:   ast.NewIdent(name),
						Sel: ast.NewIdent(anyIdent),
					},
				},
			},
		},
	}
	astFile.Decls = append(astFile.Decls, reference)
}

func InstrumentFile(filename_in string, filename_out string) {
    fset := token.NewFileSet()
    astFile, err := parser.ParseFile(fset, filename_in, nil, parser.ParseComments)
    if err != nil {
        failf("failed to parse filename %v: %v", filename_in, err)
    }

    astFile.Comments = trimComments(astFile, fset)

	file := &File{
		fset:      fset,
		astFile:   astFile,
	}

    file.addImport("github.com/guidovranken/go-coverage-instrumentation/helper", "fuzz_helper", "AddCoverage")

	ast.Walk(file, file.astFile)

    var buf bytes.Buffer
    if err := format.Node(&buf, fset, astFile); err != nil {
        panic(err)
    }

    d, _ := path.Split(filename_out)
    os.Mkdir(d, 0700)
    err = ioutil.WriteFile(filename_out, buf.Bytes(), 0644)
    if err != nil {
        fmt.Printf("error: %s\n", err)
        panic("")
    }
}

func main() {
    fileList_in := []string{}
    fileList_out := []string{}
    filepath.Walk(os.Args[1], func(path string, f os.FileInfo, err error) error {
        if strings.HasSuffix(path, ".go") {
            fileList_in = append(fileList_in, path)

            if !strings.HasPrefix(path, os.Args[1]) {
                panic("")
            }
            fileList_out = append(fileList_out, os.Args[2] + path[len(os.Args[1]):])
        }
        return nil
    })
	for i := 0 ; i < len(fileList_in); i++ {
        fmt.Printf("Processing %s, output to %s\n", fileList_in[i], fileList_out[i])
        InstrumentFile(fileList_in[i], fileList_out[i])
    }
}
