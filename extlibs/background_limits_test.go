package extlibs

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

func backgroundInterp(t *testing.T, opts ...scriptling.Option) *scriptling.Scriptling {
	t.Helper()
	ResetRuntime()
	SetSandboxFactory(nil)
	p := scriptling.New(opts...)
	stdlib.RegisterAll(p)
	RegisterRuntimeLibraryAll(p, nil)
	if _, err := p.Eval(`
import scriptling.runtime as runtime
import time
def nap(s):
    time.sleep(s)
    return s
`); err != nil {
		t.Fatal(err)
	}
	return p
}

// waitTasks waits for the interpreter's background tasks to drain.
func waitTasks(t *testing.T, p *scriptling.Scriptling) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for p.ResourceUsage().BackgroundTasks != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d background tasks still accounted", p.ResourceUsage().BackgroundTasks)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Each background task holds about 45 KB (its environment and goroutine)
// that no allocation point reported, so a loop starting tasks ran a process
// under an 8 MiB limit to gigabytes and a fatal, uncatchable out-of-memory
// (CWE-770, strix 4250 vuln-0016). Live tasks are now charged against the
// limit: the loop is stopped with the memory-limit error after a few hundred
// tasks. Tasks already started stay charged until they end (a task blocked in
// a sleep notices the cancellation when it wakes), and are then released.
func TestBackgroundTasksChargedToMemoryLimit(t *testing.T) {
	p := backgroundInterp(t, scriptling.WithMemoryLimit(8<<20), scriptling.WithMaxValueBytes(0))
	goroutines := runtime.NumGoroutine()
	_, err := p.Eval(`
i = 0
while i < 20000:
    runtime.background("t" + str(i), "nap", 1, daemon=True)
    i += 1
`)
	if err == nil || !strings.Contains(err.Error(), "memory limit exceeded") {
		t.Fatalf("want the memory-limit error, got %v", err)
	}
	started, _ := p.GetVar("i")
	n, _ := started.(int64)
	if n < 50 || n > 8<<20/object.TaskBytes {
		t.Fatalf("%d tasks started under an 8 MiB limit", n)
	}
	if live := p.ResourceUsage().BackgroundTasks; live != n {
		t.Fatalf("%d tasks accounted, %d started", live, n)
	}
	if used := p.ResourceUsage().MemoryBytes; used < n*object.TaskBytes {
		t.Fatalf("%d bytes charged for %d live tasks", used, n)
	}
	waitTasks(t, p)
	if n := runtime.NumGoroutine() - goroutines; n > 20 {
		t.Fatalf("%d goroutines left behind", n)
	}
}

// Only live tasks are charged: many short tasks run one after another under
// the same limit, and the charge returns to nothing.
func TestBackgroundSequentialTasksWithinMemoryLimit(t *testing.T) {
	p := backgroundInterp(t, scriptling.WithMemoryLimit(8<<20), scriptling.WithMaxValueBytes(0))
	if _, err := p.Eval(`
for i in range(500):
    runtime.background("seq", "nap", 0).get()
`); err != nil {
		t.Fatal(err)
	}
	waitTasks(t, p)
	if used := p.ResourceUsage().MemoryBytes; used > 4<<20 {
		t.Fatalf("%d bytes still charged after the tasks finished", used)
	}
}

// WithMaxBackgroundTasks caps live tasks: one more raises a catchable
// RuntimeError naming the limit, a re-registered name is not counted twice,
// shared tasks count, and a finished task frees its slot.
func TestBackgroundTaskLimit(t *testing.T) {
	p := backgroundInterp(t, scriptling.WithMaxBackgroundTasks(3))
	result, err := p.Eval(`
a = runtime.background("a", "nap", 0.5)
b = runtime.background("b", "nap", 0.5, shared=True)
again = runtime.background("a", "nap", 0.5)
c = runtime.background("c", "nap", 0.5)
try:
    runtime.background("d", "nap", 0.5)
    r = "started"
except RuntimeError as e:
    r = str(e)
r`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); !strings.Contains(got, "background task limit reached: 3 tasks are queued or running, the limit is 3") {
		t.Fatalf("fourth task: %s", got)
	}
	if n := p.ResourceUsage().BackgroundTasks; n != 3 {
		t.Fatalf("%d tasks accounted, want 3", n)
	}
	if _, err := p.Eval(`a.get()
d = runtime.background("d", "nap", 0)
d.get()`); err != nil {
		t.Fatalf("after a task finished: %v", err)
	}
	if _, err := p.Eval("b.get()\nc.get()"); err != nil {
		t.Fatal(err)
	}
	waitTasks(t, p)
}

// The process default applies to interpreters without their own limit, and
// an interpreter's own 0 lifts it.
func TestBackgroundTaskLimitDefault(t *testing.T) {
	scriptling.SetMaxBackgroundTasks(1)
	defer scriptling.SetMaxBackgroundTasks(0)

	p := backgroundInterp(t)
	if _, err := p.Eval(`
x = runtime.background("x", "nap", 0.3)
runtime.background("y", "nap", 0)`); err == nil || !strings.Contains(err.Error(), "the limit is 1") {
		t.Fatalf("default limit: %v", err)
	}
	p.Eval("x.get()")
	waitTasks(t, p)

	p = backgroundInterp(t, scriptling.WithMaxBackgroundTasks(0))
	if _, err := p.Eval(`
x = runtime.background("x", "nap", 0.1)
y = runtime.background("y", "nap", 0.1)
x.get()
y.get()`); err != nil {
		t.Fatalf("own limit of 0: %v", err)
	}
	waitTasks(t, p)
}

// A queued task (a setup script's, before the server releases tasks) counts
// from the moment it is queued, and its slot is freed when it finishes, or
// when ResetRuntime drops it unrun.
func TestBackgroundTaskLimitCountsQueued(t *testing.T) {
	for _, release := range []bool{true, false} {
		ResetRuntime()
		SetBackgroundFactory(func() SandboxInstance {
			p2 := scriptling.New()
			stdlib.RegisterAll(p2)
			RegisterRuntimeLibraryAll(p2, nil)
			return p2
		})
		p := scriptling.New(scriptling.WithMaxBackgroundTasks(2))
		stdlib.RegisterAll(p)
		RegisterRuntimeLibraryAll(p, nil)
		result, err := p.Eval(`
import scriptling.runtime as runtime
def quick():
    return 1
runtime.background("q1", "quick")
runtime.background("q2", "quick")
try:
    runtime.background("q3", "quick")
    r = "queued"
except RuntimeError as e:
    r = str(e)
r`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Inspect(), "the limit is 2") {
			t.Fatalf("third queued task: %s", result.Inspect())
		}
		if release {
			ReleaseBackgroundTasks()
		} else {
			ResetRuntime()
		}
		waitTasks(t, p)
	}
	ResetRuntime()
}
