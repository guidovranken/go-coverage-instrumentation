package fuzz_helper
import (
    "runtime"
    "encoding/binary"
    "encoding/json"
    "fmt"
    "bytes"
    "crypto/sha1"
    "strings"
    "io/ioutil"
)

type symcov struct {
    CoveredPoints []string `json:"covered-points"`
    BinaryHash string `json:"binary-hash"`
    PointSymbolInfo map[string]map[string]map[string]string `json:"point-symbol-info"`
}

const (
	CoverSize       = 64 << 10
)

var covered = make([]int, CoverSize)
var total_coverage int;

var instrumentation_type int;

var maxAlloc uint64;

var stackDepth int;
var maxStackDepth int

var mergeMode int;

/* For symcov serialization */
var g_locations []string
var g_symcov bool

func getCaller() (string) {
    pc, file, line, ok := runtime.Caller(2)
    if ok == false {
        panic("Unable to retrieve caller information")
    }
    funcname := runtime.FuncForPC(pc)
    if funcname == nil {
        panic("Unable to resolve caller function name")
    }
    return fmt.Sprintf("%v %v %v", funcname.Name(), file, line)
}


func addToLocations(location string) {
    for _, s := range g_locations {
        if s == location {
            return
        }
    }
    g_locations = append(g_locations, location)
}

func formatSymcov() string {
    hashes := make([]string, 0)
    locmap := make(map[string]map[string]map[string]string, 0)
    i := 0
    for _, loc := range g_locations {
        parts := strings.Split(loc, " ")
        funcname := parts[0]
        file := parts[1]
        line := fmt.Sprintf("%v:0", parts[2])
        hash := fmt.Sprintf("%06d", i)
        hashes = append(hashes, hash)
        if _, ok := locmap[file]; !ok {
            locmap[file] = make(map[string]map[string]string)
        }
        if _, ok := locmap[file][funcname]; !ok {
            locmap[file][funcname] = make(map[string]string)
        }
        locmap[file][funcname][hash] = line
        i++
    }
    s := &symcov{
        hashes,
        "x",
        locmap}

    j, err := json.Marshal(s)
    if err != nil {
        panic("Error encoding to JSON")
    }

    return string(j)
}

func EnableSymcovWriter() {
    g_symcov = true
}

func WriteSymcov(filename string) {
    if g_symcov == true {
        ioutil.WriteFile(filename, []byte(formatSymcov()), 0644)
    }
}

/* instrumentation type 0: code coverage guided
   instrumentation type 1: stack depth guided
   instrumentation type 2: memory usage guided
*/
func SetInstrumentationType(t int) {
    switch t {
        case    1:
            instrumentation_type = 1
        case    2:
            instrumentation_type = 2
        default:
            instrumentation_type = 0
    }
}

/* Enable a specific way of reporting code coverage, required to make
   libFuzzer's corpus minimizing feature work for custom guided code
*/
func MergeMode() {
    mergeMode = 1
}

func ResetCoverage() {
    if mergeMode == 1 {
        covered = make([]int, CoverSize)
        total_coverage = 0
    }
    stackDepth = 0
}

func AddCoverage(idx int) {
    switch instrumentation_type {
        case    2:
            /* Heap alloc guided */
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            if m.Alloc > maxAlloc {
                maxAlloc = m.Alloc
            }
        default:
            /* Code coverage guided */
            if covered[idx] == 0 {
                covered[idx] = 1
                total_coverage += 1
            }

            if g_symcov == true {
                addToLocations(getCaller())
            }
    }
}

func CalcCoverage() uint64 {
    switch instrumentation_type {
        case    1:
            return uint64(maxStackDepth)
        case    2:
            /* This currently allows for up to 2GB of allocations */
            return maxAlloc
        default:
            if mergeMode == 1 {
                /* The purpose of the following lines is to derive a unique
                   value from the combination of code points hit
                   (eg. the items in covered[] set to 1 by AddCoverage).

                   The reason for having this unique value is so
                   libFuzzer can decide which inputs lead to new code
                   coverage in corpus minimizing mode.

                   The code below stores the index of the code points
                   accessed by AddCoverage() as a string, computes
                   the sha1 hash over this string, and compresses it
                   into a uint64.

                   This is not an ideal way to do it, but for now it works,
                   and speed is not an issue because minimizing a corpus
                   is a one-off operation, rarely performed.
                */
                var buffer bytes.Buffer
                for i:= 0; i < CoverSize; i++ {
                    if covered[i] != 0 {
                        buffer.WriteString( fmt.Sprintf("%v ", i) )
                    }
                }
                h := sha1.New()
                h.Write(buffer.Bytes())
                sha := h.Sum(nil)
                sha64 := binary.LittleEndian.Uint64(sha)
                return sha64
            }
            return uint64(total_coverage)
        }
}


func IncrementStack() {
    stackDepth += 1
    if stackDepth > maxStackDepth {
        maxStackDepth = stackDepth
    }
}
func DecrementStack() {
    stackDepth -= 1
}
func CalcStackDepth() int {
    return maxStackDepth
}
