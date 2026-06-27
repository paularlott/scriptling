package scriptling

import (
	"sync"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// Many goroutines evaluating against ONE shared instance must be serialized by
// the interpreter lock: the script-level mutations (env map writes, list
// appends) would otherwise race/corrupt. Eval goes through EvalWithContext,
// which acquires the lock. Run under -race to prove safety.
func TestGILSharedEnvConcurrent(t *testing.T) {
	p := New()
	if _, err := p.Eval(`
counter = 0
log = []
def bump(n):
    global counter
    counter = counter + n
    log.append(n)
    return counter
`); err != nil {
		t.Fatal(err)
	}

	const goroutines = 16
	const each = 100

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				if _, err := p.Eval("bump(1)"); err != nil {
					t.Errorf("call failed: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	got, gErr := p.GetVarAsInt("counter")
	if gErr != nil {
		t.Fatal(gErr)
	}
	if want := int64(goroutines * each); got != want {
		t.Fatalf("counter = %d, want %d (lost updates => GIL not protecting shared env)", got, want)
	}

	// The list must have exactly one entry per call (no lost/torn appends).
	logObj, _ := p.GetVarAsObject("log")
	if l, ok := logObj.(*object.List); !ok || len(l.Elements) != goroutines*each {
		n := -1
		if l, ok := logObj.(*object.List); ok {
			n = len(l.Elements)
		}
		t.Fatalf("log length = %d, want %d", n, goroutines*each)
	}
}

// Separate instances have separate roots/locks and must run fully in parallel
// (no shared GIL contention) while remaining correct.
func TestGILSeparateInstancesParallel(t *testing.T) {
	var wg sync.WaitGroup
	errs := make(chan string, 16)
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := New()
			out, err := p.Eval(`
def fib(n):
    if n < 2:
        return n
    return fib(n-1) + fib(n-2)
fib(18)
`)
			if err != nil {
				errs <- err.Error()
				return
			}
			if out.Inspect() != "2584" {
				errs <- "got " + out.Inspect()
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}
