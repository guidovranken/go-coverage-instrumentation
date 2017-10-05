package fuzz_helper
import (
    "runtime"
)
const (
	CoverSize       = 64 << 10
)

var covered = make([]int, CoverSize)
var total_coverage int;

var instrumentation_type int;

var maxAlloc uint64;

var stackDepth int;
var maxStackDepth int

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

func ResetCoverage() {
    stackDepth = 0
    //total_coverage = 0
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
    }
}

func CalcCoverage() int {
    switch instrumentation_type {
        case    1:
            return maxStackDepth
        case    2:
            /* This currently allows for up to 2GB of allocations */
            return int(maxAlloc)
        default:
            return total_coverage
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
