package fuzz_helper

const (
	CoverSize       = 64 << 10
)

var covered = make([]int, CoverSize)

var new_coverage int;
func ResetCoverage() {
    new_coverage = 0
}

func AddCoverage(idx int) {
    if covered[idx] == 0 {
        covered[idx] = 1
        new_coverage += 1
    }
}

func CalcCoverage() int {
    return new_coverage
}
