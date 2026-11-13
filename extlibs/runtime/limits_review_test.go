package runtime

import (
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

// A fire-and-forget task outlives the call that started it, even with a
// memory limit (which wraps each call in a cancellable run).
func TestBackgroundTaskOutlivesItsCall(t *testing.T) {
	for _, opts := range [][]scriptling.Option{nil, {scriptling.WithMemoryLimit(1 << 30)}} {
		ResetRuntime()
		SetSandboxFactory(nil)
		p := scriptling.New(opts...)
		stdlib.RegisterAll(p)
		registerAll(p, nil)
		if _, err := p.Eval(`
import scriptling.runtime as runtime
import time
def task():
    time.sleep(0.1)
    n = 0
    for i in range(1000):
        n += 1
    return n
pr = runtime.background("outlives", "task")
`); err != nil {
			t.Fatal(err)
		}
		result, err := p.Eval("pr.get()")
		if err != nil || result.Inspect() != "1000" {
			t.Fatalf("memory limit %v: task result %v, error %v", opts != nil, result, err)
		}
	}
}

// A detached task still counts against the shared memory budget, and is
// stopped when it goes over it.
func TestBackgroundTaskStoppedByMemoryLimit(t *testing.T) {
	ResetRuntime()
	SetSandboxFactory(nil)
	p := scriptling.New(scriptling.WithMemoryLimit(8<<20), scriptling.WithMaxValueBytes(0))
	stdlib.RegisterAll(p)
	registerAll(p, nil)
	if _, err := p.Eval(`
import scriptling.runtime as runtime
def hog():
    held = []
    n = 1000
    i = 0
    while True:
        held.append(str(i) * n)
        i += 1
pr = runtime.background("hog", "hog")
`); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err := p.Eval("pr.get()")
	if err == nil {
		t.Fatal("task holding more than the memory limit ran to completion")
	}
	if time.Since(start) > 20*time.Second {
		t.Fatal("task was slow to stop")
	}
	if p.ResourceUsage().MemoryLimitCancellations == 0 {
		t.Fatalf("no cancellation counted: %v", err)
	}
}

// A shared task, like an isolated one, outlives the call that started it.
func TestSharedTaskOutlivesItsCall(t *testing.T) {
	for _, opts := range [][]scriptling.Option{nil, {scriptling.WithMemoryLimit(1 << 30)}} {
		ResetRuntime()
		SetSandboxFactory(nil)
		p := scriptling.New(opts...)
		stdlib.RegisterAll(p)
		registerAll(p, nil)
		if _, err := p.Eval(`
import scriptling.runtime as runtime
import time
def task():
    time.sleep(0.1)
    return 42
pr = runtime.background("shared-outlives", "task", shared=True)
`); err != nil {
			t.Fatal(err)
		}
		result, err := p.Eval("pr.get()")
		if err != nil || result.Inspect() != "42" {
			t.Fatalf("memory limit %v: result %v, error %v", opts != nil, result, err)
		}
	}
}
