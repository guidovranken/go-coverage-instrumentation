package fuzz_helper

const (
	CoverSize       = 64 << 10
)

var covered = make([]int, CoverSize)

var total_coverage int;

func ResetCoverage() {
    stackDepth = 0
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

var stackDepth int;
var maxStackDepth int;
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
