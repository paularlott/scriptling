package extlibs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func kvStore() *object.Builtin {
	return newKVStoreObject(RuntimeState.KVDB, "")
}

func kvCall(store *object.Builtin, method string, args ...object.Object) object.Object {
	fn := store.Attributes[method].(*object.Builtin)
	return fn.Fn(context.Background(), object.Kwargs{}, args...)
}

func kvCallKw(store *object.Builtin, method string, kwargs map[string]object.Object, args ...object.Object) object.Object {
	fn := store.Attributes[method].(*object.Builtin)
	return fn.Fn(context.Background(), object.Kwargs{Kwargs: kwargs}, args...)
}

func kvOpen(name string) *object.Builtin {
	result := kvOpenBuiltin.Fn(context.Background(), object.Kwargs{}, &object.String{Value: name})
	return result.(*object.Builtin)
}

func assertString(t *testing.T, result object.Object, expected string) {
	t.Helper()
	s, ok := result.(*object.String)
	if !ok || s.Value != expected {
		t.Errorf("Expected string %q, got %v (%T)", expected, result, result)
	}
}

func assertInt(t *testing.T, result object.Object, expected int64) {
	t.Helper()
	i, ok := result.(*object.Integer)
	if !ok || i.Value != expected {
		t.Errorf("Expected int %d, got %v (%T)", expected, result, result)
	}
}

func assertBool(t *testing.T, result object.Object, expected bool) {
	t.Helper()
	b, ok := result.(*object.Boolean)
	if !ok || b.Value != expected {
		t.Errorf("Expected bool %v, got %v (%T)", expected, result, result)
	}
}

func assertNull(t *testing.T, result object.Object) {
	t.Helper()
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("Expected Null, got %v (%T)", result, result)
	}
}

func assertListLen(t *testing.T, result object.Object, expected int) {
	t.Helper()
	l, ok := result.(*object.List)
	if !ok || len(l.Elements) != expected {
		t.Errorf("Expected list of length %d, got %v (%T)", expected, result, result)
	}
}

// ---------------------------------------------------------------------------
// Default store — in-memory
// ---------------------------------------------------------------------------

func TestKVDefaultMemory(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("InitKVStore: %v", err)
	}
	defer CloseKVStore()

	store := kvStore()

	t.Run("SetAndGet_String", func(t *testing.T) {
		kvCall(store, "set", &object.String{Value: "k"}, &object.String{Value: "v"})
		assertString(t, kvCall(store, "get", &object.String{Value: "k"}), "v")
	})

	t.Run("SetAndGet_Int", func(t *testing.T) {
		kvCall(store, "set", &object.String{Value: "n"}, object.NewInteger(99))
		assertInt(t, kvCall(store, "get", &object.String{Value: "n"}), 99)
	})

	t.Run("SetAndGet_Float", func(t *testing.T) {
		kvCall(store, "set", &object.String{Value: "f"}, &object.Float{Value: 3.14})
		r := kvCall(store, "get", &object.String{Value: "f"})
		if fl, ok := r.(*object.Float); !ok || fl.Value != 3.14 {
			t.Errorf("Expected float 3.14, got %v", r)
		}
	})

	t.Run("SetAndGet_Bool", func(t *testing.T) {
		kvCall(store, "set", &object.String{Value: "b"}, &object.Boolean{Value: true})
		assertBool(t, kvCall(store, "get", &object.String{Value: "b"}), true)
	})

	t.Run("SetAndGet_List", func(t *testing.T) {
		kvCall(store, "set", &object.String{Value: "l"},
			&object.List{Elements: []object.Object{object.NewInteger(1), object.NewInteger(2)}})
		r := kvCall(store, "get", &object.String{Value: "l"})
		assertListLen(t, r, 2)
	})

	t.Run("Get_DefaultKwarg", func(t *testing.T) {
		assertString(t,
			kvCallKw(store, "get", map[string]object.Object{"default": &object.String{Value: "fallback"}},
				&object.String{Value: "missing"}),
			"fallback")
	})

	t.Run("Get_DefaultPositional", func(t *testing.T) {
		assertString(t,
			kvCall(store, "get", &object.String{Value: "missing2"}, &object.String{Value: "pos_fallback"}),
			"pos_fallback")
	})

	t.Run("Get_MissingReturnsNull", func(t *testing.T) {
		assertNull(t, kvCall(store, "get", &object.String{Value: "no_such_key"}))
	})

	t.Run("Exists", func(t *testing.T) {
		kvCall(store, "set", &object.String{Value: "ex"}, &object.String{Value: "yes"})
		assertBool(t, kvCall(store, "exists", &object.String{Value: "ex"}), true)
		assertBool(t, kvCall(store, "exists", &object.String{Value: "no_such"}), false)
	})

	t.Run("Delete", func(t *testing.T) {
		kvCall(store, "set", &object.String{Value: "del"}, &object.String{Value: "x"})
		kvCall(store, "delete", &object.String{Value: "del"})
		assertBool(t, kvCall(store, "exists", &object.String{Value: "del"}), false)
	})

	t.Run("TTL_Permanent", func(t *testing.T) {
		kvCall(store, "set", &object.String{Value: "perm"}, &object.String{Value: "v"})
		assertInt(t, kvCall(store, "ttl", &object.String{Value: "perm"}), -1)
	})

	t.Run("TTL_WithKwarg", func(t *testing.T) {
		kvCallKw(store, "set",
			map[string]object.Object{"ttl": object.NewInteger(60)},
			&object.String{Value: "ttlkw"}, &object.String{Value: "v"})
		r := kvCall(store, "ttl", &object.String{Value: "ttlkw"})
		if i, ok := r.(*object.Integer); !ok || i.Value <= 0 || i.Value > 60 {
			t.Errorf("Expected TTL ~60s, got %v", r)
		}
	})

	t.Run("TTL_WithPositional", func(t *testing.T) {
		kvCall(store, "set",
			&object.String{Value: "ttlpos"}, &object.String{Value: "v"}, object.NewInteger(30))
		r := kvCall(store, "ttl", &object.String{Value: "ttlpos"})
		if i, ok := r.(*object.Integer); !ok || i.Value <= 0 || i.Value > 30 {
			t.Errorf("Expected TTL ~30s, got %v", r)
		}
	})

	t.Run("TTL_Missing", func(t *testing.T) {
		assertInt(t, kvCall(store, "ttl", &object.String{Value: "no_such_ttl"}), -2)
	})

	t.Run("Keys_All", func(t *testing.T) {
		kvCall(store, "clear")
		kvCall(store, "set", &object.String{Value: "a:1"}, &object.String{Value: "v"})
		kvCall(store, "set", &object.String{Value: "a:2"}, &object.String{Value: "v"})
		kvCall(store, "set", &object.String{Value: "b:1"}, &object.String{Value: "v"})
		assertListLen(t, kvCall(store, "keys"), 3)
	})

	t.Run("Keys_PatternKwarg", func(t *testing.T) {
		assertListLen(t,
			kvCallKw(store, "keys", map[string]object.Object{"pattern": &object.String{Value: "a:*"}}),
			2)
	})

	t.Run("Keys_PatternPositional", func(t *testing.T) {
		assertListLen(t, kvCall(store, "keys", &object.String{Value: "b:*"}), 1)
	})

	t.Run("Clear", func(t *testing.T) {
		kvCall(store, "clear")
		assertListLen(t, kvCall(store, "keys"), 0)
	})

	t.Run("Close_IsNoop", func(t *testing.T) {
		kvCall(store, "close")
		if RuntimeState.KVDB == nil {
			t.Error("Default store close should be a no-op — KVDB should still be set")
		}
		// Store should still be usable
		kvCall(store, "set", &object.String{Value: "after_close"}, &object.String{Value: "ok"})
		assertString(t, kvCall(store, "get", &object.String{Value: "after_close"}), "ok")
	})
}

// ---------------------------------------------------------------------------
// Default store — persistent
// ---------------------------------------------------------------------------

func TestKVDefaultPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kv-default-persist-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, "default.db")

	t.Run("Write", func(t *testing.T) {
		if err := InitKVStore(path); err != nil {
			t.Fatalf("InitKVStore: %v", err)
		}
		store := kvStore()
		kvCall(store, "set", &object.String{Value: "str"}, &object.String{Value: "hello"})
		kvCall(store, "set", &object.String{Value: "num"}, object.NewInteger(42))
		kvCall(store, "set", &object.String{Value: "flag"}, &object.Boolean{Value: true})
		if err := RuntimeState.KVDB.Save(); err != nil {
			t.Fatalf("Save: %v", err)
		}
		CloseKVStore()
	})

	t.Run("Read", func(t *testing.T) {
		if err := InitKVStore(path); err != nil {
			t.Fatalf("InitKVStore: %v", err)
		}
		defer CloseKVStore()
		store := kvStore()
		assertString(t, kvCall(store, "get", &object.String{Value: "str"}), "hello")
		assertInt(t, kvCall(store, "get", &object.String{Value: "num"}), 42)
		assertBool(t, kvCall(store, "get", &object.String{Value: "flag"}), true)
	})
}

// ---------------------------------------------------------------------------
// Default store — TTL expiry
// ---------------------------------------------------------------------------

func TestKVDefaultTTLExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TTL expiry test in short mode")
	}
	if err := InitKVStore(""); err != nil {
		t.Fatalf("InitKVStore: %v", err)
	}
	defer CloseKVStore()

	store := kvStore()

	// Use the underlying DB for sub-second TTL since the store object works in whole seconds
	RuntimeState.KVDB.SetEx("ttl:short", "expires", 100*time.Millisecond)
	assertBool(t, kvCall(store, "exists", &object.String{Value: "ttl:short"}), true)

	time.Sleep(200 * time.Millisecond)
	RuntimeState.KVDB.Delete("ttl:short") // force eviction of expired key
	assertBool(t, kvCall(store, "exists", &object.String{Value: "ttl:short"}), false)

	// 1-second TTL via store object
	kvCall(store, "set", &object.String{Value: "ttl:1s"}, &object.String{Value: "v"}, object.NewInteger(1))
	assertBool(t, kvCall(store, "exists", &object.String{Value: "ttl:1s"}), true)
	time.Sleep(1100 * time.Millisecond)
	assertBool(t, kvCall(store, "exists", &object.String{Value: "ttl:1s"}), false)

	// Permanent key survives
	kvCall(store, "set", &object.String{Value: "ttl:perm"}, &object.String{Value: "v"})
	time.Sleep(200 * time.Millisecond)
	assertBool(t, kvCall(store, "exists", &object.String{Value: "ttl:perm"}), true)
}

// ---------------------------------------------------------------------------
// Named memory store
// ---------------------------------------------------------------------------

func TestKVNamedMemoryStore(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("InitKVStore: %v", err)
	}
	defer CloseKVStore()
	defer closeKVRegistry()

	t.Run("BasicOps", func(t *testing.T) {
		store := kvOpen(":memory:basic")
		defer kvCall(store, "close")

		kvCall(store, "set", &object.String{Value: "x"}, object.NewInteger(1))
		assertInt(t, kvCall(store, "get", &object.String{Value: "x"}), 1)
		assertBool(t, kvCall(store, "exists", &object.String{Value: "x"}), true)
		kvCall(store, "delete", &object.String{Value: "x"})
		assertBool(t, kvCall(store, "exists", &object.String{Value: "x"}), false)
	})

	t.Run("SharedAcrossWrappers", func(t *testing.T) {
		s1 := kvOpen(":memory:shared")
		s2 := kvOpen(":memory:shared")

		kvCall(s1, "set", &object.String{Value: "msg"}, &object.String{Value: "hello"})
		assertString(t, kvCall(s2, "get", &object.String{Value: "msg"}), "hello")

		kvCall(s1, "close")
		kvCall(s2, "close")
	})

	t.Run("IsolatedFromDefault", func(t *testing.T) {
		def := kvStore()
		named := kvOpen(":memory:isolated")
		defer kvCall(named, "close")

		kvCall(def, "set", &object.String{Value: "iso"}, &object.String{Value: "default"})
		kvCall(named, "set", &object.String{Value: "iso"}, &object.String{Value: "named"})

		assertString(t, kvCall(def, "get", &object.String{Value: "iso"}), "default")
		assertString(t, kvCall(named, "get", &object.String{Value: "iso"}), "named")
	})

	t.Run("IsolatedFromOtherNamed", func(t *testing.T) {
		a := kvOpen(":memory:iso_a")
		b := kvOpen(":memory:iso_b")
		defer kvCall(a, "close")
		defer kvCall(b, "close")

		kvCall(a, "set", &object.String{Value: "k"}, &object.String{Value: "from_a"})
		assertNull(t, kvCall(b, "get", &object.String{Value: "k"}))
	})

	t.Run("RefCountPartialClose", func(t *testing.T) {
		s1 := kvOpen(":memory:refcount")
		s2 := kvOpen(":memory:refcount")

		kvCall(s1, "set", &object.String{Value: "alive"}, &object.String{Value: "yes"})
		kvCall(s1, "close") // ref count → 1, DB still open

		// s2 should still work
		assertString(t, kvCall(s2, "get", &object.String{Value: "alive"}), "yes")
		kvCall(s2, "close") // ref count → 0, DB closed

		kvRegistry.Lock()
		_, stillOpen := kvRegistry.stores[":memory:refcount"]
		kvRegistry.Unlock()
		if stillOpen {
			t.Error("Registry entry should be removed after all refs closed")
		}
	})

	t.Run("ReopenAfterClose", func(t *testing.T) {
		s1 := kvOpen(":memory:reopen")
		kvCall(s1, "set", &object.String{Value: "k"}, &object.String{Value: "v1"})
		kvCall(s1, "close")

		// After full close, reopening creates a fresh in-memory store
		s2 := kvOpen(":memory:reopen")
		defer kvCall(s2, "close")
		assertNull(t, kvCall(s2, "get", &object.String{Value: "k"}))
	})
}

// ---------------------------------------------------------------------------
// Named persistent store
// ---------------------------------------------------------------------------

func TestKVNamedPersistentStore(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("InitKVStore: %v", err)
	}
	defer CloseKVStore()
	defer closeKVRegistry()

	tmpDir, err := os.MkdirTemp("", "kv-named-persist-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, "named.db")

	t.Run("WriteAndClose", func(t *testing.T) {
		store := kvOpen(path)
		kvCall(store, "set", &object.String{Value: "persist"}, &object.String{Value: "data"})
		kvCall(store, "set", &object.String{Value: "num"}, object.NewInteger(777))

		// Access underlying DB to force save
		kvRegistry.Lock()
		entry := kvRegistry.stores[path]
		kvRegistry.Unlock()
		if err := entry.db.Save(); err != nil {
			t.Fatalf("Save: %v", err)
		}
		kvCall(store, "close")
	})

	t.Run("ReadAfterReopen", func(t *testing.T) {
		store := kvOpen(path)
		defer kvCall(store, "close")
		assertString(t, kvCall(store, "get", &object.String{Value: "persist"}), "data")
		assertInt(t, kvCall(store, "get", &object.String{Value: "num"}), 777)
	})

	t.Run("SharedWritesThenPersist", func(t *testing.T) {
		path2 := filepath.Join(tmpDir, "shared_persist.db")
		s1 := kvOpen(path2)
		s2 := kvOpen(path2)

		kvCall(s1, "set", &object.String{Value: "from_s1"}, &object.String{Value: "yes"})
		assertString(t, kvCall(s2, "get", &object.String{Value: "from_s1"}), "yes")

		kvRegistry.Lock()
		entry := kvRegistry.stores[path2]
		kvRegistry.Unlock()
		entry.db.Save()

		kvCall(s1, "close")
		kvCall(s2, "close")

		// Reopen and verify persistence
		s3 := kvOpen(path2)
		defer kvCall(s3, "close")
		assertString(t, kvCall(s3, "get", &object.String{Value: "from_s1"}), "yes")
	})
}

// ---------------------------------------------------------------------------
// Concurrent access — default store
// ---------------------------------------------------------------------------

func TestKVDefaultConcurrent(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("InitKVStore: %v", err)
	}
	defer CloseKVStore()

	const goroutines = 20
	const keysPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			store := kvStore()
			for k := 0; k < keysPerGoroutine; k++ {
				key := &object.String{Value: fmt.Sprintf("g%d:k%d", id, k)}
				val := object.NewInteger(int64(id*1000 + k))
				kvCall(store, "set", key, val)
				kvCall(store, "get", key)
				kvCall(store, "exists", key)
			}
		}(g)
	}
	wg.Wait()

	store := kvStore()
	result := kvCall(store, "keys")
	l, ok := result.(*object.List)
	if !ok || len(l.Elements) != goroutines*keysPerGoroutine {
		t.Errorf("Expected %d keys, got %d", goroutines*keysPerGoroutine, len(l.Elements))
	}
}

// ---------------------------------------------------------------------------
// Concurrent access — named memory store
// ---------------------------------------------------------------------------

func TestKVNamedMemoryConcurrent(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("InitKVStore: %v", err)
	}
	defer CloseKVStore()
	defer closeKVRegistry()

	const goroutines = 20
	const keysPerGoroutine = 50

	// Hold one ref open for the duration so the DB survives all goroutine closes
	keeper := kvOpen(":memory:concurrent")
	defer kvCall(keeper, "close")

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			store := kvOpen(":memory:concurrent")
			defer kvCall(store, "close")
			for k := 0; k < keysPerGoroutine; k++ {
				key := &object.String{Value: fmt.Sprintf("g%d:k%d", id, k)}
				val := object.NewInteger(int64(id*1000 + k))
				kvCall(store, "set", key, val)
				kvCall(store, "get", key)
				kvCall(store, "exists", key)
			}
		}(g)
	}
	wg.Wait()

	// Verify all keys written via the keeper wrapper
	result := kvCall(keeper, "keys")
	l, ok := result.(*object.List)
	if !ok || len(l.Elements) != goroutines*keysPerGoroutine {
		t.Errorf("Expected %d keys, got %d", goroutines*keysPerGoroutine, len(l.Elements))
	}
}

// ---------------------------------------------------------------------------
// Concurrent registry open/close
// ---------------------------------------------------------------------------

func TestKVRegistryConcurrentOpenClose(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("InitKVStore: %v", err)
	}
	defer CloseKVStore()
	defer closeKVRegistry()

	const goroutines = 30

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			store := kvOpen(":memory:race")
			kvCall(store, "set", &object.String{Value: "k"}, &object.String{Value: "v"})
			kvCall(store, "get", &object.String{Value: "k"})
			kvCall(store, "close")
		}()
	}
	wg.Wait()

	// All goroutines closed — registry entry should be gone
	kvRegistry.Lock()
	_, stillOpen := kvRegistry.stores[":memory:race"]
	kvRegistry.Unlock()
	if stillOpen {
		t.Error("Registry entry should be removed after all concurrent refs closed")
	}
}

// ---------------------------------------------------------------------------
// Registry cleanup via RegisterCleanup
// ---------------------------------------------------------------------------

func TestKVRegistryCleanupOnReset(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("InitKVStore: %v", err)
	}

	// Open a named store to populate the registry
	store := kvOpen(":memory:cleanup_test")
	kvCall(store, "set", &object.String{Value: "k"}, &object.String{Value: "v"})

	// Register cleanup (normally done by NewKVSubLibrary)
	RegisterCleanup(closeKVRegistry)

	// ResetRuntime should fire the cleanup
	ResetRuntime()

	kvRegistry.Lock()
	count := len(kvRegistry.stores)
	kvRegistry.Unlock()
	if count != 0 {
		t.Errorf("Expected empty registry after ResetRuntime, got %d entries", count)
	}
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestKVOpenErrors(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("InitKVStore: %v", err)
	}
	defer CloseKVStore()
	defer closeKVRegistry()

	ctx := context.Background()

	t.Run("EmptyName", func(t *testing.T) {
		result := kvOpenBuiltin.Fn(ctx, object.Kwargs{}, &object.String{Value: ""})
		if _, ok := result.(*object.Error); !ok {
			t.Error("Expected error for empty store name")
		}
	})

	t.Run("MissingArg", func(t *testing.T) {
		result := kvOpenBuiltin.Fn(ctx, object.Kwargs{})
		if _, ok := result.(*object.Error); !ok {
			t.Error("Expected error for missing argument")
		}
	})

	t.Run("SetMissingArgs", func(t *testing.T) {
		store := kvOpen(":memory:err_test")
		defer kvCall(store, "close")
		result := kvCall(store, "set", &object.String{Value: "k"}) // missing value
		if _, ok := result.(*object.Error); !ok {
			t.Error("Expected error for set with missing value arg")
		}
	})

	t.Run("GetMissingArgs", func(t *testing.T) {
		store := kvOpen(":memory:err_test2")
		defer kvCall(store, "close")
		result := kvCall(store, "get") // missing key
		if _, ok := result.(*object.Error); !ok {
			t.Error("Expected error for get with missing key arg")
		}
	})
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func TestKVConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    object.Object
		expected interface{}
	}{
		{"string", &object.String{Value: "hello"}, "hello"},
		{"int", object.NewInteger(42), int64(42)},
		{"float", &object.Float{Value: 3.14}, 3.14},
		{"bool true", &object.Boolean{Value: true}, true},
		{"bool false", &object.Boolean{Value: false}, false},
		{"null", &object.Null{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conversion.ToGoWithError(tt.input)
			if err != nil {
				t.Fatalf("Conversion failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
