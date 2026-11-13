//go:build race

package filesystem

// raceDetector is true when the race detector is on; heap-size assertions
// are skipped then, as its shadow memory and slower collections inflate the
// process heap.
const raceDetector = true
