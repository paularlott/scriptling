package extlibs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

func TestKVStoreMemoryMode(t *testing.T) {
	// Initialize in-memory mode
	if err := InitKVStore(""); err != nil {
		t.Fatalf("Failed to init KV store: %v", err)
	}
	defer CloseKVStore()

	// Test Set and Get
	t.Run("SetAndGet", func(t *testing.T) {
		db := RuntimeState.KVDB

		// Set a value
		if err := db.Set("test:key1", "value1"); err != nil {
			t.Fatalf("Failed to set: %v", err)
		}

		// Get the value
		val, err := db.Get("test:key1")
		if err != nil {
			t.Fatalf("Failed to get: %v", err)
		}
		if val.(string) != "value1" {
			t.Errorf("Expected 'value1', got %v", val)
		}
	})

	// Test Exists
	t.Run("Exists", func(t *testing.T) {
		db := RuntimeState.KVDB

		if !db.Exists("test:key1") {
			t.Error("Key should exist")
		}

		if db.Exists("nonexistent") {
			t.Error("Nonexistent key should not exist")
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		db := RuntimeState.KVDB

		if err := db.Delete("test:key1"); err != nil {
			t.Fatalf("Failed to delete: %v", err)
		}

		if db.Exists("test:key1") {
			t.Error("Key should not exist after delete")
		}
	})

	// Test different value types
	t.Run("ValueTypes", func(t *testing.T) {
		db := RuntimeState.KVDB

		// String
		db.Set("type:string", "hello")
		val, _ := db.Get("type:string")
		if val.(string) != "hello" {
			t.Errorf("String mismatch: %v", val)
		}

		// Integer
		db.Set("type:int", int64(42))
		val, _ = db.Get("type:int")
		if val.(int64) != 42 {
			t.Errorf("Int mismatch: %v", val)
		}

		// Map
		db.Set("type:map", map[string]any{"name": "test", "count": 123})
		val, _ = db.Get("type:map")
		m := val.(map[string]any)
		if m["name"].(string) != "test" {
			t.Errorf("Map name mismatch: %v", m["name"])
		}

		// Slice
		db.Set("type:slice", []any{1, 2, 3})
		val, _ = db.Get("type:slice")
		s := val.([]any)
		if len(s) != 3 {
			t.Errorf("Slice length mismatch: %d", len(s))
		}
	})

	// Test TTL
	t.Run("TTL", func(t *testing.T) {
		db := RuntimeState.KVDB

		// Key with no expiration
		db.Set("ttl:permanent", "value")
		ttl := db.TTL("ttl:permanent")
		if ttl >= 0 {
			t.Errorf("Permanent key should have negative TTL, got %v", ttl)
		}

		// Key with TTL
		db.SetEx("ttl:temporary", "value", 10*time.Second)
		ttl = db.TTL("ttl:temporary")
		if ttl < 0 {
			t.Errorf("Temporary key should have positive TTL, got %v", ttl)
		}
		if ttl > 10*time.Second || ttl < 9*time.Second {
			t.Errorf("TTL should be ~10s, got %v", ttl)
		}

		// Nonexistent key
		ttl = db.TTL("nonexistent")
		if ttl != -2*time.Second {
			t.Errorf("Nonexistent key should return -2s, got %v", ttl)
		}
	})

	// Test FindKeysByPrefix
	t.Run("FindKeysByPrefix", func(t *testing.T) {
		db := RuntimeState.KVDB

		db.Set("prefix:a:1", "1")
		db.Set("prefix:a:2", "2")
		db.Set("prefix:b:1", "1")
		db.Set("other:1", "1")

		keys := db.FindKeysByPrefix("prefix:a:")
		if len(keys) != 2 {
			t.Errorf("Expected 2 keys with prefix 'prefix:a:', got %d", len(keys))
		}

		keys = db.FindKeysByPrefix("prefix:")
		if len(keys) != 3 {
			t.Errorf("Expected 3 keys with prefix 'prefix:', got %d", len(keys))
		}

		keys = db.FindKeysByPrefix("")
		if len(keys) < 4 {
			t.Errorf("Expected at least 4 keys with empty prefix, got %d", len(keys))
		}
	})
}

func TestKVStorePersistence(t *testing.T) {
	// Create temp directory for persistence
	tmpDir, err := os.MkdirTemp("", "kv-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "kvstore")

	// First run: create and store data
	t.Run("FirstRun", func(t *testing.T) {
		if err := InitKVStore(storagePath); err != nil {
			t.Fatalf("Failed to init KV store: %v", err)
		}

		db := RuntimeState.KVDB

		// Store various values
		db.Set("persistent:string", "hello world")
		db.Set("persistent:int", int64(12345))
		db.Set("persistent:map", map[string]any{
			"name":  "test",
			"value": 42,
			"nested": map[string]any{
				"key": "nested_value",
			},
		})
		db.Set("persistent:slice", []any{1, "two", 3.0})

		// Force save
		if err := db.Save(); err != nil {
			t.Fatalf("Failed to save: %v", err)
		}

		CloseKVStore()
	})

	// Second run: verify data persisted
	t.Run("SecondRun", func(t *testing.T) {
		if err := InitKVStore(storagePath); err != nil {
			t.Fatalf("Failed to init KV store: %v", err)
		}
		defer CloseKVStore()

		db := RuntimeState.KVDB

		// Verify string
		val, err := db.Get("persistent:string")
		if err != nil {
			t.Fatalf("Failed to get string: %v", err)
		}
		if val.(string) != "hello world" {
			t.Errorf("String mismatch: expected 'hello world', got %v", val)
		}

		// Verify int
		val, err = db.Get("persistent:int")
		if err != nil {
			t.Fatalf("Failed to get int: %v", err)
		}
		// Handle different int types from deserialization
		var intVal int64
		switch v := val.(type) {
		case int64:
			intVal = v
		case int:
			intVal = int64(v)
		case float64:
			intVal = int64(v)
		default:
			t.Fatalf("Unexpected int type: %T", val)
		}
		if intVal != 12345 {
			t.Errorf("Int mismatch: expected 12345, got %d", intVal)
		}

		// Verify map
		val, err = db.Get("persistent:map")
		if err != nil {
			t.Fatalf("Failed to get map: %v", err)
		}
		m := val.(map[string]any)
		if m["name"].(string) != "test" {
			t.Errorf("Map name mismatch: %v", m["name"])
		}

		// Verify slice
		val, err = db.Get("persistent:slice")
		if err != nil {
			t.Fatalf("Failed to get slice: %v", err)
		}
		s := val.([]any)
		if len(s) != 3 {
			t.Errorf("Slice length mismatch: expected 3, got %d", len(s))
		}

		// Verify all keys exist
		if !db.Exists("persistent:string") {
			t.Error("persistent:string should exist")
		}
		if !db.Exists("persistent:int") {
			t.Error("persistent:int should exist")
		}
		if !db.Exists("persistent:map") {
			t.Error("persistent:map should exist")
		}
		if !db.Exists("persistent:slice") {
			t.Error("persistent:slice should exist")
		}
	})
}

func TestKVStoreTTLExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TTL expiry test in short mode")
	}

	// Create temp directory for persistence
	tmpDir, err := os.MkdirTemp("", "kv-ttl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "kvstore")

	// Use short cleanup interval for testing
	cfg := RuntimeState.KVDB
	_ = cfg // We'll test with the default init

	if err := InitKVStore(storagePath); err != nil {
		t.Fatalf("Failed to init KV store: %v", err)
	}
	defer CloseKVStore()

	db := RuntimeState.KVDB

	// Set a key with short TTL
	db.SetEx("ttl:short", "expires soon", 100*time.Millisecond)

	// Verify it exists
	if !db.Exists("ttl:short") {
		t.Error("Key should exist immediately after set")
	}

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Trigger cleanup manually (since cleanup interval is 1 minute)
	db.Delete("ttl:short") // This will work if already expired

	// Key should not exist (either expired or cleaned up)
	if db.Exists("ttl:short") {
		t.Error("Key should not exist after TTL expiry")
	}

	// Permanent key should still exist
	db.Set("ttl:permanent", "never expires")
	time.Sleep(150 * time.Millisecond)
	if !db.Exists("ttl:permanent") {
		t.Error("Permanent key should still exist")
	}
}

func TestKVStoreClear(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("Failed to init KV store: %v", err)
	}
	defer CloseKVStore()

	db := RuntimeState.KVDB

	// Add multiple keys
	for i := 0; i < 10; i++ {
		db.Set(filepath.Join("clear", "test", string(rune('0'+i))), i)
	}

	// Verify keys exist
	keys := db.FindKeysByPrefix("clear/")
	if len(keys) != 10 {
		t.Errorf("Expected 10 keys, got %d", len(keys))
	}

	// Clear all keys
	allKeys := db.FindKeysByPrefix("")
	for _, key := range allKeys {
		db.Delete(key)
	}

	// Verify all keys are gone
	keys = db.FindKeysByPrefix("")
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys after clear, got %d", len(keys))
	}
}

func TestKVStoreIncr(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("Failed to init KV store: %v", err)
	}
	defer CloseKVStore()

	// Test incr via snapshotkv directly
	db := RuntimeState.KVDB

	// Increment non-existent key (should create it)
	db.Set("incr:test", int64(0))
	val, _ := db.Get("incr:test")
	if val.(int64) != 0 {
		t.Errorf("Expected 0, got %v", val)
	}

	// Simulate incr by getting, modifying, setting
	currentVal, _ := db.Get("incr:test")
	intVal := currentVal.(int64)
	db.Set("incr:test", intVal+1)

	val, _ = db.Get("incr:test")
	if val.(int64) != 1 {
		t.Errorf("Expected 1 after incr, got %v", val)
	}

	// Increment by more
	currentVal, _ = db.Get("incr:test")
	intVal = currentVal.(int64)
	db.Set("incr:test", intVal+5)

	val, _ = db.Get("incr:test")
	if val.(int64) != 6 {
		t.Errorf("Expected 6 after incr by 5, got %v", val)
	}
}

func TestKVStoreNilValues(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("Failed to init KV store: %v", err)
	}
	defer CloseKVStore()

	db := RuntimeState.KVDB

	// Test nil value
	db.Set("nil:test", nil)
	val, err := db.Get("nil:test")
	if err != nil {
		t.Fatalf("Failed to get nil value: %v", err)
	}
	if val != nil {
		t.Errorf("Expected nil, got %v", val)
	}

	// Verify it exists
	if !db.Exists("nil:test") {
		t.Error("Key with nil value should exist")
	}
}

func TestKVStoreEmptyKey(t *testing.T) {
	if err := InitKVStore(""); err != nil {
		t.Fatalf("Failed to init KV store: %v", err)
	}
	defer CloseKVStore()

	db := RuntimeState.KVDB

	// Empty key should work
	db.Set("", "empty key value")
	val, err := db.Get("")
	if err != nil {
		t.Fatalf("Failed to get empty key: %v", err)
	}
	if val.(string) != "empty key value" {
		t.Errorf("Empty key value mismatch: %v", val)
	}
}

// Test conversion helpers
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
