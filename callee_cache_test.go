package scriptling

import (
	"fmt"
	"sync"
	"testing"
)

// Reassignment must be reflected (cache stores a location, not a value).
func TestCalleeCacheReassignment(t *testing.T) {
	p := New()
	out, err := p.Eval(`
def a():
    return 1
def b():
    return 2
def pick(f):
    return f()
r1 = pick(a)
a = b          # rebind a -> b's body via name
def call_a():
    return a()
r2 = call_a()
r3 = call_a()
[r1, r2, r3]
`)
	if err != nil {
		t.Fatal(err)
	}
	if out.Inspect() != "[1, 2, 2]" {
		t.Fatalf("reassignment not reflected: got %s", out.Inspect())
	}
}

// Closures: same call node, different captured envs across instances.
func TestCalleeCacheClosure(t *testing.T) {
	p := New()
	out, err := p.Eval(`
def make(n):
    def helper():
        return n
    def run():
        return helper()      # callee 'helper' lives in enclosing frame
    return run
r = make(10)() + make(20)() + make(10)()
r
`)
	if err != nil {
		t.Fatal(err)
	}
	if out.Inspect() != "40" {
		t.Fatalf("closure callee wrong: got %s", out.Inspect())
	}
}

// Cross-instance concurrency over shared (parse-cached) AST exercising recursion.
func TestCalleeCacheConcurrentSharedAST(t *testing.T) {
	const src = `
def fib(n):
    if n < 2:
        return n
    return fib(n - 1) + fib(n - 2)
def helper(x):
    return x * 2
helper(fib(15))
`
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := New()
			out, err := p.Eval(src)
			if err != nil {
				errs <- err.Error()
				return
			}
			// fib(15)=610, *2 = 1220
			if out.Inspect() != "1220" {
				errs <- fmt.Sprintf("got %s", out.Inspect())
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("concurrent mismatch: %s", e)
	}
}
