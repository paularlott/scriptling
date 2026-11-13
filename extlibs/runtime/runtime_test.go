package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/runtime/kv"
	"github.com/paularlott/scriptling/object"
)

func TestRuntimeSync(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

# Test Atomic
counter = runtime.sync.Atomic("test_counter", initial=0)
counter.add(1)
counter.add(5)
v1 = counter.get()
counter.set(100)
v2 = counter.get()

# Test Shared
shared = runtime.sync.Shared("test_shared", initial=[])
shared.set([1, 2, 3])
v3 = shared.get()

# Test Queue
queue = runtime.sync.Queue("test_queue", maxsize=10)
queue.put("item1")
queue.put("item2")
v4 = queue.size()
v5 = queue.get()

# Test WaitGroup
wg = runtime.sync.WaitGroup("test_wg")
wg.add(1)
wg.done()

[v1, v2, v3, v4, v5]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, _ := result.(*object.List)

	if i, _ := list.Elements[0].AsInt(); i != 6 {
		t.Errorf("Expected 6, got %d", i)
	}

	if i, _ := list.Elements[1].AsInt(); i != 100 {
		t.Errorf("Expected 100, got %d", i)
	}

	if i, _ := list.Elements[3].AsInt(); i != 2 {
		t.Errorf("Expected queue size 2, got %d", i)
	}

	if s, _ := list.Elements[4].AsString(); s != "item1" {
		t.Errorf("Expected 'item1', got %s", s)
	}
}

func TestRuntimeBackground(t *testing.T) {
	ResetRuntime()
	SetBackgroundFactory(func() SandboxInstance {
		p2 := scriptling.New()
		registerAll(p2, nil)
		return p2
	})
	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

runtime.background("task1", "worker.run")
runtime.background("task2", "worker.cleanup")
`

	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if len(RuntimeState.Backgrounds) != 2 {
		t.Errorf("Expected 2 background tasks, got %d", len(RuntimeState.Backgrounds))
	}

	if handler, ok := RuntimeState.Backgrounds["task1"]; !ok || handler != "worker.run" {
		t.Error("Background task1 not registered correctly")
	}
}

func TestRuntimeCrossEnvironmentSync(t *testing.T) {
	ResetRuntime()
	kv.InitStore("", nil)

	// Create two separate scriptling instances
	p1 := scriptling.New()
	registerAll(p1, nil)

	p2 := scriptling.New()
	registerAll(p2, nil)

	// Set value in p1
	_, err := p1.Eval(`
import scriptling.runtime as runtime
runtime.kv.default.set("shared_key", "shared_value")
counter = runtime.sync.Atomic("shared_counter", initial=0)
counter.add(10)
`)
	if err != nil {
		t.Fatalf("P1 script error: %v", err)
	}

	// Read value in p2
	result, err := p2.Eval(`
import scriptling.runtime as runtime
v1 = runtime.kv.default.get("shared_key")
counter = runtime.sync.Atomic("shared_counter")
v2 = counter.get()
[v1, v2]
`)
	if err != nil {
		t.Fatalf("P2 script error: %v", err)
	}

	list, _ := result.(*object.List)
	if s, _ := list.Elements[0].AsString(); s != "shared_value" {
		t.Errorf("Expected 'shared_value', got %s", s)
	}

	if i, _ := list.Elements[1].AsInt(); i != 10 {
		t.Errorf("Expected 10, got %d", i)
	}
}

func TestRuntimeSyncSharedUpdate(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

shared = runtime.sync.Shared("test_update", initial=0)

# Test update with a callback
def increment(current):
    return current + 10

result = shared.update(increment)
v1 = shared.get()

# Chain multiple updates
shared.update(lambda x: x + 5)
shared.update(lambda x: x * 2)
v2 = shared.get()

[result, v1, v2]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, _ := result.(*object.List)

	if i, _ := list.Elements[0].AsInt(); i != 10 {
		t.Errorf("Expected update result 10, got %d", i)
	}

	if i, _ := list.Elements[1].AsInt(); i != 10 {
		t.Errorf("Expected v1=10, got %d", i)
	}

	// (10 + 5) * 2 = 30
	if i, _ := list.Elements[2].AsInt(); i != 30 {
		t.Errorf("Expected v2=30, got %d", i)
	}
}

func TestRuntimeSyncQueueClose(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	// Test basic close and size
	script := `
import scriptling.runtime as runtime

queue = runtime.sync.Queue("test_close_queue", maxsize=5)
queue.put("item1")
queue.put("item2")
s1 = queue.size()

queue.close()
s2 = queue.size()

[s1, s2]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, _ := result.(*object.List)

	if i, _ := list.Elements[0].AsInt(); i != 2 {
		t.Errorf("Expected size 2, got %d", i)
	}

	if i, _ := list.Elements[1].AsInt(); i != 2 {
		t.Errorf("Expected size still 2 after close, got %d", i)
	}

	// Test that putting after close returns an error
	ctx := context.Background()
	queueName := object.NewString("test_close_queue")
	queueFn := SyncSubLibrary.Functions()["Queue"]
	queueObj := queueFn.Fn(ctx, object.Kwargs{}, queueName)
	putFn := queueObj.(*object.Builtin).Attributes["put"].(*object.Builtin)

	putResult := putFn.Fn(ctx, object.Kwargs{}, object.NewString("item3"))
	if _, ok := putResult.(*object.Error); !ok {
		t.Errorf("Expected error when putting to closed queue, got %T", putResult)
	}
}

func TestRuntimeSyncWaitGroupWait(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

wg = runtime.sync.WaitGroup("test_wg_wait")
counter = runtime.sync.Atomic("wg_counter", initial=0)

# Add 3 workers
wg.add(3)

# Simulate completing workers
counter.add(1)
wg.done()

counter.add(1)
wg.done()

counter.add(1)
wg.done()

# Now wait should return immediately
wg.wait()
final_count = counter.get()

final_count
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if i, _ := result.AsInt(); i != 3 {
		t.Errorf("Expected counter 3, got %d", i)
	}
}

func TestRuntimeSyncQueueMaxsize(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

queue = runtime.sync.Queue("test_maxsize", maxsize=3)
queue.put(1)
queue.put(2)
queue.put(3)
s1 = queue.size()

# Get one item to make room
item = queue.get()
s2 = queue.size()

[item, s1, s2]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, _ := result.(*object.List)

	if i, _ := list.Elements[1].AsInt(); i != 3 {
		t.Errorf("Expected max size 3, got %d", i)
	}

	if i, _ := list.Elements[2].AsInt(); i != 2 {
		t.Errorf("Expected size 2 after get, got %d", i)
	}
}

func TestRuntimeSyncAtomicDefaultDelta(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

counter = runtime.sync.Atomic("test_default_delta", initial=0)

# add() with no args should add 1
v1 = counter.add()
v2 = counter.add()
v3 = counter.get()

[v1, v2, v3]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, _ := result.(*object.List)

	if i, _ := list.Elements[0].AsInt(); i != 1 {
		t.Errorf("Expected first add to return 1, got %d", i)
	}

	if i, _ := list.Elements[1].AsInt(); i != 2 {
		t.Errorf("Expected second add to return 2, got %d", i)
	}

	if i, _ := list.Elements[2].AsInt(); i != 2 {
		t.Errorf("Expected final value 2, got %d", i)
	}
}

func TestRuntimeSyncSharedWithList(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

shared = runtime.sync.Shared("test_shared_list", initial=[])
shared.set([1, 2, 3])
v1 = shared.get()

# Update that appends to the list
def append_item(lst):
    return lst + [4]

shared.update(append_item)
v2 = shared.get()

[len(v1), len(v2)]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, _ := result.(*object.List)

	if i, _ := list.Elements[0].AsInt(); i != 3 {
		t.Errorf("Expected list length 3, got %d", i)
	}

	if i, _ := list.Elements[1].AsInt(); i != 4 {
		t.Errorf("Expected list length 4 after update, got %d", i)
	}
}

func TestRuntimeSyncAtomicNegative(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

counter = runtime.sync.Atomic("test_negative", initial=100)
counter.add(-50)
v1 = counter.get()
counter.add(-100)
v2 = counter.get()

[v1, v2]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, _ := result.(*object.List)

	if i, _ := list.Elements[0].AsInt(); i != 50 {
		t.Errorf("Expected 50, got %d", i)
	}

	if i, _ := list.Elements[1].AsInt(); i != -50 {
		t.Errorf("Expected -50, got %d", i)
	}
}

func TestRuntimeBackgroundReturnsPromise(t *testing.T) {
	ResetRuntime()

	RuntimeState.Lock()
	RuntimeState.BackgroundReady = true
	RuntimeState.Unlock()

	p := scriptling.New()
	registerAll(p, nil)

	SetBackgroundFactory(func() SandboxInstance {
		p2 := scriptling.New()
		registerAll(p2, nil)
		return p2
	})

	_, err := p.Eval(`
import scriptling.runtime as runtime

def my_handler(a, b):
    return a + b
`)
	if err != nil {
		t.Fatalf("Setup error: %v", err)
	}

	// background() should return a Promise (Builtin with get/wait)
	result, err := p.Eval(`
promise = runtime.background("test_promise", "my_handler", 5, 3)
promise.get()
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if i, err := result.AsInt(); err != nil || i != 8 {
		t.Errorf("Expected promise result 8, got %v (%T)", result.Inspect(), result)
	}
}

func TestRuntimeBackgroundFuncNameClean(t *testing.T) {
	ResetRuntime()

	RuntimeState.Lock()
	RuntimeState.BackgroundReady = true
	RuntimeState.Unlock()

	p := scriptling.New()
	registerAll(p, nil)

	SetBackgroundFactory(func() SandboxInstance {
		p2 := scriptling.New()
		registerAll(p2, nil)
		return p2
	})

	// Define a function that writes to shared state, then accesses an undefined variable
	_, err := p.Eval(`
import scriptling.runtime as runtime

# Pre-create the atomic so the background task can look it up by name
runtime.sync.Atomic("clean_test", initial=0)

def my_task():
    import scriptling.runtime as runtime
    counter = runtime.sync.Atomic("clean_test")
    counter.add(1)
    x  # undefined — should error here
`)
	if err != nil {
		t.Fatalf("setup error: %v", err)
	}

	// Call background — should run the function (incrementing counter) then error on x
	_, err = p.Eval(`runtime.background("test_clean", "my_task")`)
	if err != nil {
		t.Fatalf("background call error: %v", err)
	}

	// Wait briefly for the goroutine to run
	time.Sleep(100 * time.Millisecond)

	RuntimeState.RLock()
	atomic := RuntimeState.Atomics["clean_test"]
	RuntimeState.RUnlock()
	if atomic == nil {
		t.Fatal("Atomic not found")
	}
	if v := atomic.get(); v != 1 {
		t.Errorf("Expected counter 1 (ran but errored on x), got %d", v)
	}
}

func TestRuntimeBackgroundFuncNameWithArgs(t *testing.T) {
	ResetRuntime()

	RuntimeState.Lock()
	RuntimeState.BackgroundReady = true
	RuntimeState.Unlock()

	p := scriptling.New()
	registerAll(p, nil)

	SetBackgroundFactory(func() SandboxInstance {
		p2 := scriptling.New()
		registerAll(p2, nil)
		return p2
	})

	// Define a function that writes to shared state via import
	_, err := p.Eval(`
import scriptling.runtime as runtime

# Pre-create the atomic so the background task can look it up by name
runtime.sync.Atomic("args_test", initial=0)

def my_task(x, y):
    import scriptling.runtime as runtime
    counter = runtime.sync.Atomic("args_test")
    counter.set(x + y)
`)
	if err != nil {
		t.Fatalf("setup error: %v", err)
	}

	// Call background with args
	_, err = p.Eval(`runtime.background("test_args", "my_task", 10, 5)`)
	if err != nil {
		t.Fatalf("background call error: %v", err)
	}

	// Wait for the goroutine to complete
	time.Sleep(100 * time.Millisecond)

	RuntimeState.RLock()
	atomic := RuntimeState.Atomics["args_test"]
	RuntimeState.RUnlock()
	if atomic == nil {
		t.Fatal("Atomic not found")
	}
	if v := atomic.get(); v != 15 {
		t.Errorf("Expected counter 15, got %d", v)
	}
}

func TestRuntimeBackgroundFuncNameSiblingCall(t *testing.T) {
	ResetRuntime()

	RuntimeState.Lock()
	RuntimeState.BackgroundReady = true
	RuntimeState.Unlock()

	p := scriptling.New()
	registerAll(p, nil)

	SetBackgroundFactory(func() SandboxInstance {
		p2 := scriptling.New()
		registerAll(p2, nil)
		return p2
	})

	// Define functions that call each other, accessing shared state via import
	_, err := p.Eval(`
import scriptling.runtime as runtime

# Pre-create the atomic so the background task can look it up by name
runtime.sync.Atomic("sibling_test", initial=0)

def helper(x, y):
    return x * y

def my_task():
    import scriptling.runtime as runtime
    counter = runtime.sync.Atomic("sibling_test")
    counter.set(helper(3, 5))
`)
	if err != nil {
		t.Fatalf("setup error: %v", err)
	}

	// Call background — the function calls helper (sibling)
	_, err = p.Eval(`runtime.background("test_sibling", "my_task")`)
	if err != nil {
		t.Fatalf("background call error: %v", err)
	}

	// Wait for the goroutine to complete
	time.Sleep(100 * time.Millisecond)

	RuntimeState.RLock()
	atomic := RuntimeState.Atomics["sibling_test"]
	RuntimeState.RUnlock()
	if atomic == nil {
		t.Fatal("Atomic not found")
	}
	if v := atomic.get(); v != 15 {
		t.Errorf("Expected counter 15, got %d", v)
	}
}
func TestRuntimeBackgroundImportsCopied(t *testing.T) {
	ResetRuntime()

	RuntimeState.Lock()
	RuntimeState.BackgroundReady = true
	RuntimeState.Unlock()

	p := scriptling.New()
	registerAll(p, nil)

	SetBackgroundFactory(func() SandboxInstance {
		p2 := scriptling.New()
		registerAll(p2, nil)
		return p2
	})

	// The function re-imports runtime — the background task gets a clean
	// env with only sibling functions, not the caller's imports.
	_, err := p.Eval(`
import scriptling.runtime as runtime

runtime.sync.Atomic("import_test", initial=0)

def my_task():
    import scriptling.runtime as rt
    counter = rt.sync.Atomic("import_test")
    counter.add(1)
`)
	if err != nil {
		t.Fatalf("setup error: %v", err)
	}

	_, err = p.Eval(`runtime.background("test_imports", "my_task")`)
	if err != nil {
		t.Fatalf("background call error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	RuntimeState.RLock()
	atomic := RuntimeState.Atomics["import_test"]
	RuntimeState.RUnlock()
	if atomic == nil {
		t.Fatal("Atomic not found")
	}
	if v := atomic.get(); v != 1 {
		t.Errorf("Expected counter 1, got %d", v)
	}
}

func BenchmarkRuntimeAtomicAdd(b *testing.B) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	ctx := context.Background()
	name := object.NewString("bench_counter")

	atomicFn := SyncSubLibrary.Functions()["Atomic"]
	atomic := atomicFn.Fn(ctx, object.Kwargs{}, name)
	addFn := atomic.(*object.Builtin).Attributes["add"].(*object.Builtin)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addFn.Fn(ctx, object.Kwargs{})
	}
}

// A background task whose own run was stopped hands the caller an ordinary
// error the caller's script can catch; the caller's run is not stopped.
func TestPromiseGetDowngradesTaskRunStop(t *testing.T) {
	for _, stop := range []object.Object{errors.NewTimeoutError(), errors.NewCancelledError(), errors.NewMemoryLimitStopError()} {
		promise := newPromise("stopped")
		promise.set(stop, nil)
		get, _ := promiseObject(promise).(*object.Builtin).Attributes["get"].(*object.Builtin)
		result := get.Fn(context.Background(), object.Kwargs{})
		if errors.IsRunStop(result) {
			t.Fatalf("%s: get() passed the task's run stop to a live caller", stop.Inspect())
		}
		e, ok := result.(*object.Error)
		if !ok || !strings.Contains(e.Message, "background task stopped") {
			t.Fatalf("%s: got %v, want an ordinary async error", stop.Inspect(), result)
		}
	}

	// A caller whose own run has stopped keeps the stop.
	promise := newPromise("stopped")
	promise.set(errors.NewTimeoutError(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	get, _ := promiseObject(promise).(*object.Builtin).Attributes["get"].(*object.Builtin)
	if result := get.Fn(ctx, object.Kwargs{}); !errors.IsRunStop(result) {
		t.Fatalf("a stopped caller should keep the stop, got %v", result)
	}
}
