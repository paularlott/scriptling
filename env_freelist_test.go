package scriptling

import (
	"fmt"
	"sync"
	"testing"
)

// Concurrent instances, each its own env tree / root free-list, under heavy
// recursion + varied arities. This is the designed-safe case (1 env : 1
// goroutine) and must be race-clean.
func TestFreeListConcurrentInstances(t *testing.T) {
	const src = `
def fib(n):
    if n < 2:
        return n
    return fib(n - 1) + fib(n - 2)
def add3(a, b, c):
    return a + b + c
def go():
    total = 0
    i = 0
    while i < 50:
        total = add3(total, fib(12), i)
        i = i + 1
    return total
go()
`
	var wg sync.WaitGroup
	errs := make(chan string, 128)
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := New()
			var want string
			for it := 0; it < 20; it++ {
				out, err := p.Eval(src)
				if err != nil {
					errs <- err.Error()
					return
				}
				if want == "" {
					want = out.Inspect()
				} else if out.Inspect() != want {
					errs <- fmt.Sprintf("nondeterministic: %s != %s", out.Inspect(), want)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}

// Warm free-list correctness: repeated evals on one persistent instance must not
// leak frame state between calls (different functions, same arity reuse a frame).
func TestFreeListWarmReuseCorrectness(t *testing.T) {
	p := New()
	if _, err := p.Eval(`
def f(x):
    y = x + 1
    return y
def g(x):
    return x * 100
`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		out, err := p.Eval(fmt.Sprintf("f(%d) + g(%d)", i, i))
		if err != nil {
			t.Fatal(err)
		}
		want := fmt.Sprintf("%d", (i+1)+(i*100))
		if out.Inspect() != want {
			t.Fatalf("iter %d: got %s want %s", i, out.Inspect(), want)
		}
	}
}
