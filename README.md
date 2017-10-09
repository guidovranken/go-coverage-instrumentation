# go-coverage-instrumentation

## Intro

This program injects code into Go source code files with the purpose of extracting run-time behavior that is useful for fuzzing. It is being developed for instrumenting go-ethereum's EVM code (see also: https://github.com/guidovranken/ethereum-vm-fuzzer ), and in the future its scope may be expanded to additional components of go-ethereum.

Three instrumentation types are supported:
    - Code coverage
    - Stack depth
    - Heap allocation size

Code coverage instrumentation is insert at every branch.
Stack depth instrumentation is inserted into every function.
Heap allocation size instrumentation is inserted at every branch.

## API

In ```helper/helper.go``` a number of instructions are implemented that the fuzzing application can call:

```go
func SetInstrumentationType(t int)
```

Call with parameter 0 to enable code coverage instrumentation.
Call with parameter 1 to enable stack depth instrumentation.
Call with parameter 2 to enable heap allocation instrumentation.


```go
func ResetCoverage()
```

This needs to be called before each run.

```go
func CalcCoverage() int
```

After each run this can be called to retrieve the instrumentation measurements.

If code coverage instrumentation was enabled, this returns the number of unique points in the code executed during the last run.

If stack depth instrumentation was enabled, this returns the deepest call stack depth since the last reset. So if function A calls function B calls function C during a run, this function will return ```3```.

If heap allocation size instrumentation was enabled, this returns the maximum heap allocation at a single instant (in number of bytes).

Other functions and variables should not be called or altered.

## Usage

Compile ```main.go``` in ```instrument/``` and run it like:

```sh
./main in_directory out_directory
```

Where ```in_directory``` is where the uninstrumented Go files reside, and ```out_directory``` is where the instrumented versions will be stored.
