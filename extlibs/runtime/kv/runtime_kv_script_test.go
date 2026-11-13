package kv

// Script-level tests of scriptling.runtime.kv reached through the parent
// library (`import scriptling.runtime as runtime; runtime.kv...`), moved here
// from extlibs/runtime/runtime_test.go with the library.

import (
	"context"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/runtime"
	"github.com/paularlott/scriptling/object"
)

func TestRuntimeKV(t *testing.T) {
	runtime.ResetRuntime()
	InitStore("", nil)
	p := scriptling.New()
	registerAll(p, nil)

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
	runtime.ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

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
	runtime.ResetRuntime()
	InitStore("", nil)
	p := scriptling.New()
	registerAll(p, nil)

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

func BenchmarkRuntimeKVSet(b *testing.B) {
	runtime.ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	ctx := context.Background()
	key := object.NewString("bench_key")
	value := object.NewString("bench_value")

	db, err := openRegisteredStore(":memory:kv_bench_set")
	if err != nil {
		b.Fatal(err)
	}
	store := newKVStoreObject(db, ":memory:kv_bench_set")
	setFn := store.Attributes["set"].(*object.Builtin)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		setFn.Fn(ctx, object.Kwargs{}, key, value)
	}
}

func BenchmarkRuntimeKVGet(b *testing.B) {
	runtime.ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	ctx := context.Background()
	key := object.NewString("bench_key")
	value := object.NewString("bench_value")

	db, err := openRegisteredStore(":memory:kv_bench_get")
	if err != nil {
		b.Fatal(err)
	}
	store := newKVStoreObject(db, ":memory:kv_bench_get")
	setFn := store.Attributes["set"].(*object.Builtin)
	getFn := store.Attributes["get"].(*object.Builtin)

	setFn.Fn(ctx, object.Kwargs{}, key, value)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getFn.Fn(ctx, object.Kwargs{}, key)
	}
}
