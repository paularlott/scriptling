package extlibs

import (
	"context"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func TestRuntimeHTTP(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	script := `
import scriptling.runtime as runtime

runtime.http.get("/test", "handler.test")
runtime.http.post("/api", "handler.api")
runtime.http.middleware("auth.check")
runtime.http.static("/assets", "./public")
`

	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if len(RuntimeState.Routes) != 3 {
		t.Errorf("Expected 3 routes, got %d", len(RuntimeState.Routes))
	}

	if route, ok := RuntimeState.Routes["/test"]; !ok || route.Handler != "handler.test" {
		t.Error("GET route not registered correctly")
	}

	if RuntimeState.Middleware != "auth.check" {
		t.Errorf("Middleware not set correctly: %s", RuntimeState.Middleware)
	}
}

func TestRuntimeHTTPResponses(t *testing.T) {
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	tests := []struct {
		name   string
		script string
		check  func(*object.Dict) error
	}{
		{
			name:   "json response",
			script: `import scriptling.runtime as runtime; runtime.http.json(200, {"test": "data"})`,
			check: func(d *object.Dict) error {
				if status, ok := d.GetByString("status"); !ok {
					t.Error("Missing status")
				} else if s, _ := status.Value.AsInt(); s != 200 {
					t.Errorf("Expected status 200, got %d", s)
				}
				return nil
			},
		},
		{
			name:   "redirect response",
			script: `import scriptling.runtime as runtime; runtime.http.redirect("/new")`,
			check: func(d *object.Dict) error {
				if headers, ok := d.GetByString("headers"); ok {
					if hDict, err := headers.Value.AsDict(); err == nil {
						if loc, ok := hDict["Location"]; ok {
							if locStr, err := loc.AsString(); err == nil && locStr != "/new" {
								t.Error("Location header not set correctly")
							}
						} else {
							t.Error("Location header missing")
						}
					}
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Eval(tt.script)
			if err != nil {
				t.Fatalf("Script error: %v", err)
			}
			if dict, ok := result.(*object.Dict); ok {
				tt.check(dict)
			} else {
				t.Error("Expected Dict result")
			}
		})
	}
}

func TestRuntimeKV(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	script := `
import scriptling.runtime as runtime

runtime.kv.default.set("key1", "value1")
runtime.kv.default.set("key2", 42)
runtime.kv.default.set("key3", {"nested": "data"})

v1 = runtime.kv.default.get("key1")
v2 = runtime.kv.default.get("key2")
v3 = runtime.kv.default.get("key3")
v4 = runtime.kv.default.get("missing", default="default")

exists1 = runtime.kv.default.exists("key1")
exists2 = runtime.kv.default.exists("missing")

runtime.kv.default.delete("key1")
v5 = runtime.kv.default.get("key1")

[v1, v2, v3, v4, exists1, exists2, v5]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, ok := result.(*object.List)
	if !ok {
		t.Fatal("Expected list result")
	}

	if s, _ := list.Elements[0].AsString(); s != "value1" {
		t.Errorf("Expected 'value1', got %s", s)
	}

	if i, _ := list.Elements[1].AsInt(); i != 42 {
		t.Errorf("Expected 42, got %d", i)
	}

	if s, _ := list.Elements[3].AsString(); s != "default" {
		t.Errorf("Expected 'default', got %s", s)
	}

	if b, _ := list.Elements[4].AsBool(); !b {
		t.Error("Expected exists to be true")
	}

	if b, _ := list.Elements[5].AsBool(); b {
		t.Error("Expected exists to be false")
	}

	if _, ok := list.Elements[6].(*object.Null); !ok {
		t.Error("Expected null after delete")
	}
}

func TestRuntimeKVOpenStore(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	script := `
import scriptling.runtime as runtime

store = runtime.kv.open(":memory:test_open")
store.set("counter", 1)
v1 = store.get("counter")
store.set("counter", 2)
v2 = store.get("counter")
store.close()

[v1, v2]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, _ := result.(*object.List)
	if i, _ := list.Elements[0].AsInt(); i != 1 {
		t.Errorf("Expected 1, got %d", i)
	}
	if i, _ := list.Elements[1].AsInt(); i != 2 {
		t.Errorf("Expected 2, got %d", i)
	}
}

func TestRuntimeKVTTL(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	script := `
import scriptling.runtime as runtime

runtime.kv.default.set("temp", "data", ttl=1)
exists1 = runtime.kv.default.exists("temp")
ttl1 = runtime.kv.default.ttl("temp")

exists1
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if b, _ := result.AsBool(); !b {
		t.Error("Key should exist before expiration")
	}

	time.Sleep(1100 * time.Millisecond)

	script2 := `
import scriptling.runtime as runtime
runtime.kv.default.exists("temp")
`

	result2, err := p.Eval(script2)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if b, _ := result2.AsBool(); b {
		t.Error("Key should not exist after expiration")
	}
}

func TestRuntimeSync(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

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
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

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

	// Create two separate scriptling instances
	p1 := scriptling.New()
	RegisterRuntimeLibraryAll(p1, nil)

	p2 := scriptling.New()
	RegisterRuntimeLibraryAll(p2, nil)

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
	RegisterRuntimeLibraryAll(p, nil)

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
	RegisterRuntimeLibraryAll(p, nil)

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
	queueName := &object.String{Value: "test_close_queue"}
	queueFn := SyncSubLibrary.Functions()["Queue"]
	queueObj := queueFn.Fn(ctx, object.Kwargs{}, queueName)
	putFn := queueObj.(*object.Builtin).Attributes["put"].(*object.Builtin)

	putResult := putFn.Fn(ctx, object.Kwargs{}, &object.String{Value: "item3"})
	if _, ok := putResult.(*object.Error); !ok {
		t.Errorf("Expected error when putting to closed queue, got %T", putResult)
	}
}

func TestRuntimeSyncWaitGroupWait(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

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
	RegisterRuntimeLibraryAll(p, nil)

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
	RegisterRuntimeLibraryAll(p, nil)

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
	RegisterRuntimeLibraryAll(p, nil)

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
	RegisterRuntimeLibraryAll(p, nil)

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

	// Set BackgroundReady to true so background() returns a promise
	RuntimeState.Lock()
	RuntimeState.BackgroundReady = true
	RuntimeState.Unlock()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	// Set up a factory for background tasks
	SetSandboxFactory(func() SandboxInstance {
		p2 := scriptling.New()
		RegisterRuntimeLibraryAll(p2, nil)
		return p2
	})

	// First define a handler function
	_, err := p.Eval(`
import scriptling.runtime as runtime

def my_handler(a, b):
    return a + b
`)
	if err != nil {
		t.Fatalf("Setup error: %v", err)
	}

	// Test that background returns a promise object with get/wait methods
	script := `
promise = runtime.background("test_promise", "my_handler", 5, 3)
promise
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	// Check that result is a Builtin with get and wait methods
	promise, ok := result.(*object.Builtin)
	if !ok {
		t.Fatalf("Expected Builtin (promise), got %T", result)
	}

	if _, ok := promise.Attributes["get"]; !ok {
		t.Error("Promise should have 'get' method")
	}

	if _, ok := promise.Attributes["wait"]; !ok {
		t.Error("Promise should have 'wait' method")
	}
}

func BenchmarkRuntimeKVSet(b *testing.B) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	ctx := context.Background()
	key := &object.String{Value: "bench_key"}
	value := &object.String{Value: "bench_value"}

	store := newKVStoreObject(RuntimeState.KVDB, "")
	setFn := store.Attributes["set"].(*object.Builtin)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		setFn.Fn(ctx, object.Kwargs{}, key, value)
	}
}

func BenchmarkRuntimeKVGet(b *testing.B) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	ctx := context.Background()
	key := &object.String{Value: "bench_key"}
	value := &object.String{Value: "bench_value"}

	store := newKVStoreObject(RuntimeState.KVDB, "")
	setFn := store.Attributes["set"].(*object.Builtin)
	getFn := store.Attributes["get"].(*object.Builtin)

	setFn.Fn(ctx, object.Kwargs{}, key, value)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getFn.Fn(ctx, object.Kwargs{}, key)
	}
}

func BenchmarkRuntimeAtomicAdd(b *testing.B) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	ctx := context.Background()
	name := &object.String{Value: "bench_counter"}

	atomicFn := SyncSubLibrary.Functions()["Atomic"]
	atomic := atomicFn.Fn(ctx, object.Kwargs{}, name)
	addFn := atomic.(*object.Builtin).Attributes["add"].(*object.Builtin)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addFn.Fn(ctx, object.Kwargs{})
	}
}
