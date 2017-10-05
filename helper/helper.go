package fuzz_helper

const (
	CoverSize       = 64 << 10
)

var covered = make([]int, CoverSize)

var total_coverage int;

var stackDepth int;

func ResetCoverage() {
    //total_coverage = 0
}

func AddCoverage(idx int) {
    if covered[idx] == 0 {
        covered[idx] = 1
        total_coverage += 1
    }
}

func CalcCoverage() int {
    return total_coverage
}


func IncrementStack() {
    stackDepth += 1
}
func DecrementStack() {
    stackDepth += 1
}
func CalcStackDepth() int {
    return stackDepth
}
