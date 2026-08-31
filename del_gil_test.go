package scriptling

import (
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/scriptling/object"
)

// TestDelFinalizerSerializesWithEvaluation is the production scenario the
// GIL-context work exists for: a user __del__ runs from the runtime finalizer
// goroutine while another goroutine keeps evaluating in the same environment
// tree. The destructor must serialize against those evaluations — never run
// concurrently — and must actually run. A data race fails under -race; a lost
// wakeup fails the deadline.
func TestDelFinalizerSerializesWithEvaluation(t *testing.T) {
	sl := New()

	var mu sync.Mutex
	var events []string
	noteFn := object.NewFunctionBuilder()
	noteFn.Function(func(s string) bool {
		mu.Lock()
		events = append(events, s)
		mu.Unlock()
		return true
	})
	note := &object.Builtin{Fn: noteFn.Build()}
	if err := sl.SetObjectVar("note", note); err != nil {
		t.Fatalf("set note: %v", err)
	}

	if _, err := sl.Eval(`
class Noisy:
    def __init__(self, tag):
        self.tag = tag
    def __del__(self):
        note("del:" + self.tag)

def spin(n):
    total = 0
    i = 0
    while i < n:
        total = total + i
        i = i + 1
    return total

n = Noisy("one")
spin(20000)
n = None
spin(20000)
"done"
`); err != nil {
		t.Fatalf("eval: %v", err)
	}

	// A concurrent evaluator hammers the same tree so the finalizer, when it
	// fires, must contend for the interpreter lock via the context-aware
	// path (the finalizer's 5s context is cancellable, Done() is non-nil).
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := sl.Eval("x = 1 + 1\nx"); err != nil {
				t.Errorf("concurrent eval: %v", err)
				return
			}
		}
	}()

	// Force collection: the finalizer goroutine races the evaluator for the
	// interpreter lock on every GC cycle.
	deadline := time.Now().Add(5 * time.Second)
	sawDel := false
	for time.Now().Before(deadline) {
		runtime.GC()
		runtime.GC() // finalizers from the previous cycle run now
		mu.Lock()
		n := len(events)
		mu.Unlock()
		if n > 0 {
			sawDel = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(stop)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if !sawDel {
		t.Fatal("destructor never ran while evaluation was in flight: finalizer lost, deadlocked, or cancelled")
	}
	for _, e := range events {
		if !strings.HasPrefix(e, "del:") {
			t.Fatalf("unexpected event %q", e)
		}
	}
}
