package extlibs

import (
	"context"
	"testing"
	"time"

	"github.com/paularlott/scriptling/object"
)

func TestKVSetAndGet(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear the store first
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	// Test set
	setFn := KVLibrary.Functions()["set"]
	result := setFn.Fn(ctx, kwargs,
		&object.String{Value: "test_key"},
		&object.String{Value: "test_value"},
	)

	if _, isNull := result.(*object.Null); !isNull {
		t.Errorf("set() should return None, got %T", result)
	}

	// Test get
	getFn := KVLibrary.Functions()["get"]
	result = getFn.Fn(ctx, kwargs, &object.String{Value: "test_key"})

	str, ok := result.(*object.String)
	if !ok {
		t.Errorf("get() should return String, got %T", result)
	}
	if str.Value != "test_value" {
		t.Errorf("get() = %s, want test_value", str.Value)
	}
}

func TestKVGetWithDefault(t *testing.T) {
	ctx := context.Background()

	// Clear the store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	getFn := KVLibrary.Functions()["get"]

	// Test with default value
	kwargs := object.NewKwargs(map[string]object.Object{
		"default": &object.String{Value: "default_value"},
	})
	result := getFn.Fn(ctx, kwargs, &object.String{Value: "nonexistent_key"})

	str, ok := result.(*object.String)
	if !ok {
		t.Errorf("get() should return String, got %T", result)
	}
	if str.Value != "default_value" {
		t.Errorf("get() = %s, want default_value", str.Value)
	}

	// Test with positional default
	result = getFn.Fn(ctx, object.NewKwargs(nil),
		&object.String{Value: "another_nonexistent"},
		&object.Integer{Value: 42},
	)

	num, ok := result.(*object.Integer)
	if !ok {
		t.Errorf("get() should return Integer, got %T", result)
	}
	if num.Value != 42 {
		t.Errorf("get() = %d, want 42", num.Value)
	}
}

func TestKVDelete(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear and set up
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.data["to_delete"] = &kvEntry{value: "value"}
	kvStore.Unlock()

	// Delete
	deleteFn := KVLibrary.Functions()["delete"]
	result := deleteFn.Fn(ctx, kwargs, &object.String{Value: "to_delete"})

	if _, isNull := result.(*object.Null); !isNull {
		t.Errorf("delete() should return None, got %T", result)
	}

	// Verify deleted
	getFn := KVLibrary.Functions()["get"]
	result = getFn.Fn(ctx, kwargs, &object.String{Value: "to_delete"})

	if _, isNull := result.(*object.Null); !isNull {
		t.Errorf("get() after delete should return None, got %T", result)
	}
}

func TestKVExists(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear and set up
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.data["existing_key"] = &kvEntry{value: "value"}
	kvStore.Unlock()

	existsFn := KVLibrary.Functions()["exists"]

	// Test existing key
	result := existsFn.Fn(ctx, kwargs, &object.String{Value: "existing_key"})
	b, ok := result.(*object.Boolean)
	if !ok || !b.Value {
		t.Errorf("exists() for existing key should return true")
	}

	// Test non-existing key
	result = existsFn.Fn(ctx, kwargs, &object.String{Value: "nonexistent_key"})
	b, ok = result.(*object.Boolean)
	if !ok || b.Value {
		t.Errorf("exists() for non-existing key should return false")
	}
}

func TestKVIncr(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	incrFn := KVLibrary.Functions()["incr"]

	// Test incrementing non-existent key (should initialize)
	result := incrFn.Fn(ctx, kwargs, &object.String{Value: "counter"})
	num, ok := result.(*object.Integer)
	if !ok || num.Value != 1 {
		t.Errorf("incr() on new key = %d, want 1", num.Value)
	}

	// Test incrementing existing key
	result = incrFn.Fn(ctx, kwargs, &object.String{Value: "counter"})
	num, ok = result.(*object.Integer)
	if !ok || num.Value != 2 {
		t.Errorf("incr() = %d, want 2", num.Value)
	}

	// Test incrementing by amount
	result = incrFn.Fn(ctx, kwargs,
		&object.String{Value: "counter"},
		&object.Integer{Value: 5},
	)
	num, ok = result.(*object.Integer)
	if !ok || num.Value != 7 {
		t.Errorf("incr(5) = %d, want 7", num.Value)
	}
}

func TestKVIncrWithKwargs(t *testing.T) {
	ctx := context.Background()

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.data["counter2"] = &kvEntry{value: int64(10)}
	kvStore.Unlock()

	incrFn := KVLibrary.Functions()["incr"]

	kwargs := object.NewKwargs(map[string]object.Object{
		"amount": &object.Integer{Value: 3},
	})
	result := incrFn.Fn(ctx, kwargs, &object.String{Value: "counter2"})
	num, ok := result.(*object.Integer)
	if !ok || num.Value != 13 {
		t.Errorf("incr(amount=3) = %d, want 13", num.Value)
	}
}

func TestKVKeys(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear and set up
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.data["user:1"] = &kvEntry{value: "alice"}
	kvStore.data["user:2"] = &kvEntry{value: "bob"}
	kvStore.data["session:abc"] = &kvEntry{value: "data"}
	kvStore.Unlock()

	keysFn := KVLibrary.Functions()["keys"]

	// Get all keys
	result := keysFn.Fn(ctx, kwargs)
	list, ok := result.(*object.List)
	if !ok {
		t.Errorf("keys() should return List, got %T", result)
	}
	if len(list.Elements) != 3 {
		t.Errorf("keys() returned %d keys, want 3", len(list.Elements))
	}

	// Get keys matching pattern
	kwargs = object.NewKwargs(map[string]object.Object{
		"pattern": &object.String{Value: "user:*"},
	})
	result = keysFn.Fn(ctx, kwargs)
	list, ok = result.(*object.List)
	if !ok {
		t.Errorf("keys(pattern) should return List, got %T", result)
	}
	if len(list.Elements) != 2 {
		t.Errorf("keys('user:*') returned %d keys, want 2", len(list.Elements))
	}
}

func TestKVClear(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Set up
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.data["key1"] = &kvEntry{value: "value1"}
	kvStore.data["key2"] = &kvEntry{value: "value2"}
	kvStore.Unlock()

	// Clear
	clearFn := KVLibrary.Functions()["clear"]
	result := clearFn.Fn(ctx, kwargs)

	if _, isNull := result.(*object.Null); !isNull {
		t.Errorf("clear() should return None, got %T", result)
	}

	// Verify cleared
	kvStore.RLock()
	if len(kvStore.data) != 0 {
		t.Errorf("clear() should empty the store, has %d keys", len(kvStore.data))
	}
	kvStore.RUnlock()
}

func TestKVStoreComplexTypes(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	setFn := KVLibrary.Functions()["set"]
	getFn := KVLibrary.Functions()["get"]

	// Test storing a list
	list := &object.List{Elements: []object.Object{
		&object.String{Value: "item1"},
		&object.String{Value: "item2"},
	}}
	setFn.Fn(ctx, kwargs, &object.String{Value: "my_list"}, list)

	result := getFn.Fn(ctx, kwargs, &object.String{Value: "my_list"})
	retrievedList, ok := result.(*object.List)
	if !ok {
		t.Errorf("get() for list should return List, got %T", result)
	}
	if len(retrievedList.Elements) != 2 {
		t.Errorf("list has %d elements, want 2", len(retrievedList.Elements))
	}

	// Test storing a dict
	dict := &object.Dict{Pairs: map[string]object.DictPair{
		"name":  {Key: &object.String{Value: "name"}, Value: &object.String{Value: "test"}},
		"count": {Key: &object.String{Value: "count"}, Value: &object.Integer{Value: 42}},
	}}
	setFn.Fn(ctx, kwargs, &object.String{Value: "my_dict"}, dict)

	result = getFn.Fn(ctx, kwargs, &object.String{Value: "my_dict"})
	retrievedDict, ok := result.(*object.Dict)
	if !ok {
		t.Errorf("get() for dict should return Dict, got %T", result)
	}
	if len(retrievedDict.Pairs) != 2 {
		t.Errorf("dict has %d pairs, want 2", len(retrievedDict.Pairs))
	}
}

func TestKVTTLBasic(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	setFn := KVLibrary.Functions()["set"]
	ttlFn := KVLibrary.Functions()["ttl"]

	// Set with TTL using positional argument
	setFn.Fn(ctx, kwargs,
		&object.String{Value: "temp_key"},
		&object.String{Value: "temp_value"},
		&object.Integer{Value: 10}, // 10 seconds TTL
	)

	// Check TTL
	result := ttlFn.Fn(ctx, kwargs, &object.String{Value: "temp_key"})
	num, ok := result.(*object.Integer)
	if !ok {
		t.Errorf("ttl() should return Integer, got %T", result)
	}
	if num.Value < 9 || num.Value > 10 {
		t.Errorf("ttl() = %d, want ~10", num.Value)
	}

	// Set with TTL using kwargs
	kwargsWithTTL := object.NewKwargs(map[string]object.Object{
		"ttl": &object.Integer{Value: 5},
	})
	setFn.Fn(ctx, kwargsWithTTL,
		&object.String{Value: "temp_key2"},
		&object.String{Value: "temp_value2"},
	)

	result = ttlFn.Fn(ctx, kwargs, &object.String{Value: "temp_key2"})
	num, ok = result.(*object.Integer)
	if !ok || num.Value < 4 || num.Value > 5 {
		t.Errorf("ttl() with kwargs = %d, want ~5", num.Value)
	}
}

func TestKVTTLNoExpiration(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	setFn := KVLibrary.Functions()["set"]
	ttlFn := KVLibrary.Functions()["ttl"]

	// Set without TTL
	setFn.Fn(ctx, kwargs,
		&object.String{Value: "permanent_key"},
		&object.String{Value: "permanent_value"},
	)

	// Check TTL - should return -1 for no expiration
	result := ttlFn.Fn(ctx, kwargs, &object.String{Value: "permanent_key"})
	num, ok := result.(*object.Integer)
	if !ok || num.Value != -1 {
		t.Errorf("ttl() for permanent key = %d, want -1", num.Value)
	}
}

func TestKVTTLNonExistent(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	ttlFn := KVLibrary.Functions()["ttl"]

	// Check TTL for non-existent key - should return -2
	result := ttlFn.Fn(ctx, kwargs, &object.String{Value: "nonexistent"})
	num, ok := result.(*object.Integer)
	if !ok || num.Value != -2 {
		t.Errorf("ttl() for non-existent key = %d, want -2", num.Value)
	}
}

func TestKVExpiryGet(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	setFn := KVLibrary.Functions()["set"]
	getFn := KVLibrary.Functions()["get"]

	// Set with very short TTL (1 second)
	setFn.Fn(ctx, kwargs,
		&object.String{Value: "expiring_key"},
		&object.String{Value: "expiring_value"},
		&object.Integer{Value: 1},
	)

	// Get immediately - should work
	result := getFn.Fn(ctx, kwargs, &object.String{Value: "expiring_key"})
	str, ok := result.(*object.String)
	if !ok || str.Value != "expiring_value" {
		t.Errorf("get() before expiry should return value")
	}

	// Wait for expiry
	time.Sleep(1100 * time.Millisecond)

	// Get after expiry - should return None
	result = getFn.Fn(ctx, kwargs, &object.String{Value: "expiring_key"})
	if _, isNull := result.(*object.Null); !isNull {
		t.Errorf("get() after expiry should return None, got %T", result)
	}
}

func TestKVExpiryExists(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	setFn := KVLibrary.Functions()["set"]
	existsFn := KVLibrary.Functions()["exists"]

	// Set with very short TTL
	setFn.Fn(ctx, kwargs,
		&object.String{Value: "expiring_key2"},
		&object.String{Value: "value"},
		&object.Integer{Value: 1},
	)

	// Check exists before expiry
	result := existsFn.Fn(ctx, kwargs, &object.String{Value: "expiring_key2"})
	b, ok := result.(*object.Boolean)
	if !ok || !b.Value {
		t.Errorf("exists() before expiry should return true")
	}

	// Wait for expiry
	time.Sleep(1100 * time.Millisecond)

	// Check exists after expiry
	result = existsFn.Fn(ctx, kwargs, &object.String{Value: "expiring_key2"})
	b, ok = result.(*object.Boolean)
	if !ok || b.Value {
		t.Errorf("exists() after expiry should return false")
	}
}

func TestKVExpiryKeys(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	setFn := KVLibrary.Functions()["set"]
	keysFn := KVLibrary.Functions()["keys"]

	// Set permanent key
	setFn.Fn(ctx, kwargs,
		&object.String{Value: "permanent"},
		&object.String{Value: "value"},
	)

	// Set expiring key
	setFn.Fn(ctx, kwargs,
		&object.String{Value: "expiring"},
		&object.String{Value: "value"},
		&object.Integer{Value: 1},
	)

	// Get keys before expiry - should have 2
	result := keysFn.Fn(ctx, kwargs)
	list, ok := result.(*object.List)
	if !ok || len(list.Elements) != 2 {
		t.Errorf("keys() before expiry should return 2 keys, got %d", len(list.Elements))
	}

	// Wait for expiry
	time.Sleep(1100 * time.Millisecond)

	// Get keys after expiry - should have 1 (expired key filtered out)
	result = keysFn.Fn(ctx, kwargs)
	list, ok = result.(*object.List)
	if !ok || len(list.Elements) != 1 {
		t.Errorf("keys() after expiry should return 1 key, got %d", len(list.Elements))
	}
}

func TestKVExpiryIncr(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	setFn := KVLibrary.Functions()["set"]
	incrFn := KVLibrary.Functions()["incr"]

	// Set counter with TTL
	setFn.Fn(ctx, kwargs,
		&object.String{Value: "counter"},
		&object.Integer{Value: 10},
		&object.Integer{Value: 1},
	)

	// Wait for expiry
	time.Sleep(1100 * time.Millisecond)

	// Increment after expiry - should reinitialize
	result := incrFn.Fn(ctx, kwargs, &object.String{Value: "counter"})
	num, ok := result.(*object.Integer)
	if !ok || num.Value != 1 {
		t.Errorf("incr() after expiry should reinitialize to 1, got %d", num.Value)
	}
}

func TestKVClearExpired(t *testing.T) {
	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	// Add permanent entry
	kvStore.Lock()
	kvStore.data["permanent"] = &kvEntry{value: "value"}
	kvStore.Unlock()

	// Add expired entry
	kvStore.Lock()
	kvStore.data["expired"] = &kvEntry{
		value:     "value",
		expiresAt: time.Now().Add(-1 * time.Hour), // Already expired
	}
	kvStore.Unlock()

	// Verify both exist in store
	kvStore.RLock()
	if len(kvStore.data) != 2 {
		t.Errorf("Before cleanup: expected 2 entries, got %d", len(kvStore.data))
	}
	kvStore.RUnlock()

	// Run cleanup
	ClearExpired()

	// Verify only permanent entry remains
	kvStore.RLock()
	if len(kvStore.data) != 1 {
		t.Errorf("After cleanup: expected 1 entry, got %d", len(kvStore.data))
	}
	if _, exists := kvStore.data["permanent"]; !exists {
		t.Errorf("Permanent entry should still exist")
	}
	if _, exists := kvStore.data["expired"]; exists {
		t.Errorf("Expired entry should be removed")
	}
	kvStore.RUnlock()
}

func TestKVDeepCopyIsolation(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	setFn := KVLibrary.Functions()["set"]
	getFn := KVLibrary.Functions()["get"]

	// Store a list
	originalList := &object.List{Elements: []object.Object{
		&object.String{Value: "item1"},
		&object.Integer{Value: 42},
	}}
	setFn.Fn(ctx, kwargs, &object.String{Value: "my_list"}, originalList)

	// Get the list
	result := getFn.Fn(ctx, kwargs, &object.String{Value: "my_list"})
	retrievedList, ok := result.(*object.List)
	if !ok {
		t.Fatalf("Expected List, got %T", result)
	}

	// Modify the retrieved list
	retrievedList.Elements[0] = &object.String{Value: "modified"}

	// Get again and verify original is unchanged
	result2 := getFn.Fn(ctx, kwargs, &object.String{Value: "my_list"})
	retrievedList2, ok := result2.(*object.List)
	if !ok {
		t.Fatalf("Expected List, got %T", result2)
	}

	str, ok := retrievedList2.Elements[0].(*object.String)
	if !ok || str.Value != "item1" {
		t.Errorf("Deep copy failed: original value was modified")
	}
}

func TestKVExportImport(t *testing.T) {
	// Clear and populate store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.data["key1"] = &kvEntry{value: "value1"}
	kvStore.data["key2"] = &kvEntry{value: int64(42)}
	kvStore.data["key3"] = &kvEntry{
		value:     "expiring",
		expiresAt: time.Now().Add(1 * time.Hour),
	}
	kvStore.Unlock()

	// Export
	exported, err := ExportStore()
	if err != nil {
		t.Fatalf("ExportStore() error: %v", err)
	}
	if exported == "" {
		t.Errorf("ExportStore() returned empty string")
	}

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	// Import
	err = ImportStore(exported)
	if err != nil {
		t.Fatalf("ImportStore() error: %v", err)
	}

	// Verify imported data
	kvStore.RLock()
	if len(kvStore.data) != 3 {
		t.Errorf("After import: expected 3 entries, got %d", len(kvStore.data))
	}
	if entry, exists := kvStore.data["key1"]; !exists || entry.value != "value1" {
		t.Errorf("key1 not properly imported")
	}
	if entry, exists := kvStore.data["key2"]; !exists {
		t.Errorf("key2 not properly imported")
	} else {
		// JSON unmarshaling converts numbers to float64
		switch v := entry.value.(type) {
		case int64:
			if v != 42 {
				t.Errorf("key2 value incorrect: got %v, want 42", v)
			}
		case float64:
			if v != 42.0 {
				t.Errorf("key2 value incorrect: got %v, want 42", v)
			}
		default:
			t.Errorf("key2 value has unexpected type: %T", v)
		}
	}
	kvStore.RUnlock()
}

func TestKVTTLAfterExpiry(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	setFn := KVLibrary.Functions()["set"]
	ttlFn := KVLibrary.Functions()["ttl"]

	// Set with short TTL
	setFn.Fn(ctx, kwargs,
		&object.String{Value: "expiring"},
		&object.String{Value: "value"},
		&object.Integer{Value: 1},
	)

	// Wait for expiry
	time.Sleep(1100 * time.Millisecond)

	// Check TTL after expiry - should return -2 (key doesn't exist)
	result := ttlFn.Fn(ctx, kwargs, &object.String{Value: "expiring"})
	num, ok := result.(*object.Integer)
	if !ok || num.Value != -2 {
		t.Errorf("ttl() after expiry = %d, want -2", num.Value)
	}
}

func TestKVConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Clear store
	kvStore.Lock()
	kvStore.data = make(map[string]*kvEntry)
	kvStore.Unlock()

	setFn := KVLibrary.Functions()["set"]
	getFn := KVLibrary.Functions()["get"]
	incrFn := KVLibrary.Functions()["incr"]

	// Initialize counter
	setFn.Fn(ctx, kwargs,
		&object.String{Value: "concurrent_counter"},
		&object.Integer{Value: 0},
	)

	// Concurrent increments
	const numGoroutines = 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			incrFn.Fn(ctx, kwargs, &object.String{Value: "concurrent_counter"})
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final count
	result := getFn.Fn(ctx, kwargs, &object.String{Value: "concurrent_counter"})
	num, ok := result.(*object.Integer)
	if !ok || num.Value != int64(numGoroutines) {
		t.Errorf("Concurrent incr() = %d, want %d", num.Value, numGoroutines)
	}
}
