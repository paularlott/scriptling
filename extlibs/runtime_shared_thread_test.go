package extlibs

import (
	"testing"

	"github.com/paularlott/scriptling"
)

// runtime.background(shared=True) runs the handler in the caller's own
// environment on a goroutine. Multiple shared threads mutating one global must
// be serialized by the interpreter lock (GIL) — run under -race to prove safety.
func TestRuntimeSharedThreads(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	out, err := p.Eval(`
import scriptling.runtime as runtime

counter = 0
def worker(n):
    global counter
    i = 0
    while i < n:
        counter = counter + 1   # read-modify-write on shared global
        i = i + 1
    return counter

promises = []
i = 0
while i < 8:
    promises.append(runtime.background("w" + str(i), "worker", 200, shared=True))
    i = i + 1

# wait() releases the GIL so the workers can run, then they serialize on it.
for pr in promises:
    pr.wait()

counter
`)
	if err != nil {
		t.Fatal(err)
	}
	if out.Inspect() != "1600" {
		t.Fatalf("counter = %s, want 1600 (lost updates => GIL not protecting shared threads)", out.Inspect())
	}
}

// Shared threads coordinating via a runtime.sync Queue: a producer thread and a
// consumer thread share the env. Queue.get/put release the GIL so they make
// progress concurrently. Proves the sync primitives don't deadlock shared threads.
func TestRuntimeSharedThreadsQueue(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	out, err := p.Eval(`
import scriptling.runtime as runtime

q = runtime.sync.Queue("jobs", maxsize=4)
total = 0

def producer(n):
    i = 0
    while i < n:
        q.put(i)
        i = i + 1
    q.put(-1)   # sentinel

def consumer():
    global total
    while True:
        v = q.get()
        if v < 0:
            break
        total = total + v
    return total

pp = runtime.background("prod", "producer", 50, shared=True)
cp = runtime.background("cons", "consumer", shared=True)
pp.wait()
cp.wait()
total
`)
	if err != nil {
		t.Fatal(err)
	}
	// sum 0..49 = 1225
	if out.Inspect() != "1225" {
		t.Fatalf("total = %s, want 1225", out.Inspect())
	}
}

// yield_now() releases the GIL inside a busy loop so a shared-env thread can
// run. Without it the main loop would hold the lock and the worker would never
// set the flag (the loop would spin to the cap). It is a global builtin.
func TestRuntimeYield(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	out, err := p.Eval(`
import scriptling.runtime as runtime

done = [False]
def worker():
    done[0] = True

runtime.background("y", "worker", shared=True)

i = 0
while not done[0] and i < 5000000:
    yield_now()
    i = i + 1

done[0]
`)
	if err != nil {
		t.Fatal(err)
	}
	if out.Inspect() != "true" {
		t.Fatalf("yield did not let the shared thread run (got %s)", out.Inspect())
	}
}
