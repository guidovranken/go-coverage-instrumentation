package fuzz_helper

const (
	CoverSize       = 64 << 10
)

var CoverTab    [CoverSize]byte

var covered = make([]int, CoverSize)
var coverage_cached int;

func ResetCoverage() {
    for i := 0; i < CoverSize; i++ {
        CoverTab[i] = 0
    }
}

func CalcCoverage() int {
    new_coverage := 0

    for i := 0; i < CoverSize; i++ {
        if covered[i] == 0 && CoverTab[i] != 0 {
                new_coverage += 1
                covered[i] = 1
        }
    }

    /* For the sake of speed, only calculate new coverage if at least 1
       new branch was accessed
    */
    if new_coverage > 0 {
        coverage_cached += new_coverage
    }

    /* At this point, coverage_cached always reflects the current coverage */
    return coverage_cached
}
