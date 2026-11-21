package scriptling

import (
	"runtime"
	"testing"
	"time"

	"github.com/paularlott/scriptling/stdlib"
)

// A generator dropped while paused inside a try block is closed when it is
// collected, so its finally block still runs.
func TestAbandonedGeneratorIsClosedWhenCollected(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)
	if _, err := p.Eval(`
log = []
def numbers():
    try:
        yield 1
        yield 2
    finally:
        log.append("closed")

def use():
    g = numbers()
    return next(g)

first = use()
`); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
		result, err := p.Eval(`len(log)`)
		if err != nil {
			t.Fatal(err)
		}
		if n, _ := result.AsInt(); n == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the abandoned generator's finally block never ran")
		}
	}
}

// A generator with no try or with blocks needs no finalizer, and one that
// finished is left alone.
func TestFinishedGeneratorIsNotClosedAgain(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)
	if _, err := p.Eval(`
log = []
def numbers():
    try:
        yield 1
    finally:
        log.append("closed")
for n in numbers():
    pass
`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		runtime.GC()
		time.Sleep(5 * time.Millisecond)
	}
	result, err := p.Eval(`len(log)`)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := result.AsInt(); n != 1 {
		t.Fatalf("finally ran %d times, want 1", n)
	}
}
