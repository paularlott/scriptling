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
	RegisterRuntimeLibrary(p)

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
	RegisterRuntimeLibrary(p)

	tests := []struct {
		name   string
		script string
		check  func(*object.Dict) error
	}{
		{
			name:   "json response",
			script: `import scriptling.runtime as runtime; runtime.http.json(200, {"test": "data"})`,
			check: func(d *object.Dict) error {
				if status, ok := d.Pairs["status"]; !ok {
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
				if headers, ok := d.Pairs["headers"]; ok {
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
	RegisterRuntimeLibrary(p)

	script := `
import scriptling.runtime as runtime

runtime.kv.set("key1", "value1")
runtime.kv.set("key2", 42)
runtime.kv.set("key3", {"nested": "data"})

v1 = runtime.kv.get("key1")
v2 = runtime.kv.get("key2")
v3 = runtime.kv.get("key3")
v4 = runtime.kv.get("missing", default="default")

exists1 = runtime.kv.exists("key1")
exists2 = runtime.kv.exists("missing")

runtime.kv.delete("key1")
v5 = runtime.kv.get("key1")

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

func TestRuntimeKVIncr(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibrary(p)

	script := `
import scriptling.runtime as runtime

runtime.kv.set("counter", 0)
v1 = runtime.kv.incr("counter")
v2 = runtime.kv.incr("counter", 5)
v3 = runtime.kv.incr("new_counter")

[v1, v2, v3]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, _ := result.(*object.List)
	if i, _ := list.Elements[0].AsInt(); i != 1 {
		t.Errorf("Expected 1, got %d", i)
	}
	if i, _ := list.Elements[1].AsInt(); i != 6 {
		t.Errorf("Expected 6, got %d", i)
	}
	if i, _ := list.Elements[2].AsInt(); i != 1 {
		t.Errorf("Expected 1, got %d", i)
	}
}

func TestRuntimeKVTTL(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibrary(p)

	script := `
import scriptling.runtime as runtime

runtime.kv.set("temp", "data", ttl=1)
exists1 = runtime.kv.exists("temp")
ttl1 = runtime.kv.ttl("temp")

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
runtime.kv.exists("temp")
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
	RegisterRuntimeLibrary(p)

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
	RegisterRuntimeLibrary(p)

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

func TestRuntimeRun(t *testing.T) {
	p := scriptling.New()
	RegisterRuntimeLibrary(p)

	script := `
import scriptling.runtime as runtime

def worker(x, y):
    return x + y

promise = runtime.run(worker, 5, 10)
result = promise.get()
result
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if i, _ := result.AsInt(); i != 15 {
		t.Errorf("Expected 15, got %d", i)
	}
}

func TestRuntimeRunWithKwargs(t *testing.T) {
	p := scriptling.New()
	RegisterRuntimeLibrary(p)

	script := `
import scriptling.runtime as runtime

def worker(x, y=10):
    return x + y

promise = runtime.run(worker, 5, y=3)
result = promise.get()
result
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if i, _ := result.AsInt(); i != 8 {
		t.Errorf("Expected 8, got %d", i)
	}
}

func TestRuntimeCrossEnvironmentSync(t *testing.T) {
	ResetRuntime()

	// Create two separate scriptling instances
	p1 := scriptling.New()
	RegisterRuntimeLibrary(p1)

	p2 := scriptling.New()
	RegisterRuntimeLibrary(p2)

	// Set value in p1
	_, err := p1.Eval(`
import scriptling.runtime as runtime
runtime.kv.set("shared_key", "shared_value")
counter = runtime.sync.Atomic("shared_counter", initial=0)
counter.add(10)
`)
	if err != nil {
		t.Fatalf("P1 script error: %v", err)
	}

	// Read value in p2
	result, err := p2.Eval(`
import scriptling.runtime as runtime
v1 = runtime.kv.get("shared_key")
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

func BenchmarkRuntimeKVSet(b *testing.B) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibrary(p)

	ctx := context.Background()
	key := &object.String{Value: "bench_key"}
	value := &object.String{Value: "bench_value"}

	setFn := KVSubLibrary.Functions()["set"]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		setFn.Fn(ctx, object.Kwargs{}, key, value)
	}
}

func BenchmarkRuntimeKVGet(b *testing.B) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibrary(p)

	ctx := context.Background()
	key := &object.String{Value: "bench_key"}
	value := &object.String{Value: "bench_value"}

	setFn := KVSubLibrary.Functions()["set"]
	getFn := KVSubLibrary.Functions()["get"]

	setFn.Fn(ctx, object.Kwargs{}, key, value)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getFn.Fn(ctx, object.Kwargs{}, key)
	}
}

func BenchmarkRuntimeAtomicAdd(b *testing.B) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibrary(p)

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
