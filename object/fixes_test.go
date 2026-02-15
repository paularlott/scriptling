package object

import (
	"testing"
)

func TestDictKeyFormat(t *testing.T) {
	// Test that DictKey produces type-prefixed keys
	tests := []struct {
		name     string
		obj      Object
		expected string
	}{
		{"string key", &String{Value: "hello"}, "s:hello"},
		{"empty string", &String{Value: ""}, "s:"},
		{"integer key", &Integer{Value: 42}, "n:42"},
		{"float key", &Float{Value: 3.14}, "f:3.14"},
		{"boolean true", &Boolean{Value: true}, "n:1"},
		{"boolean false", &Boolean{Value: false}, "n:0"},
		{"null key", &Null{}, "null:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DictKey(tt.obj)
			if result != tt.expected {
				t.Errorf("DictKey(%s) = %q, want %q", tt.obj.Inspect(), result, tt.expected)
			}
		})
	}
}

func TestNewStringDict(t *testing.T) {
	// Test that NewStringDict creates properly keyed dict
	d := NewStringDict(map[string]Object{
		"name": &String{Value: "test"},
		"age":  &Integer{Value: 30},
	})

	// Verify keys are stored with DictKey format
	nameKey := DictKey(&String{Value: "name"})
	if _, ok := d.Pairs[nameKey]; !ok {
		t.Errorf("Expected key %q in Pairs map", nameKey)
	}

	ageKey := DictKey(&String{Value: "age"})
	if _, ok := d.Pairs[ageKey]; !ok {
		t.Errorf("Expected key %q in Pairs map", ageKey)
	}

	// Test GetByString
	if pair, ok := d.GetByString("name"); !ok {
		t.Error("GetByString('name') should find the key")
	} else if str, ok := pair.Value.(*String); !ok || str.Value != "test" {
		t.Errorf("Expected 'test', got %v", pair.Value)
	}

	// Test HasByString
	if !d.HasByString("age") {
		t.Error("HasByString('age') should return true")
	}
	if d.HasByString("missing") {
		t.Error("HasByString('missing') should return false")
	}

	// Test SetByString
	d.SetByString("email", &String{Value: "test@example.com"})
	if !d.HasByString("email") {
		t.Error("Should have 'email' after SetByString")
	}

	// Test DeleteByString
	d.DeleteByString("email")
	if d.HasByString("email") {
		t.Error("Should not have 'email' after DeleteByString")
	}
}

func TestDictAsDict(t *testing.T) {
	// Test that AsDict returns human-readable keys (not DictKey format)
	d := NewStringDict(map[string]Object{
		"hello": &String{Value: "world"},
		"count": &Integer{Value: 5},
	})

	result, err := d.AsDict()
	if err != nil {
		t.Fatalf("AsDict() returned error: %v", err)
	}

	// Keys should be human-readable, not DictKey format
	for key := range result {
		if len(key) > 2 && key[1] == ':' {
			t.Errorf("AsDict() key %q looks like DictKey format (should be human-readable)", key)
		}
	}

	if _, ok := result["hello"]; !ok {
		t.Error("AsDict() should have key 'hello' (human-readable)")
	}
	if _, ok := result["count"]; !ok {
		t.Error("AsDict() should have key 'count' (human-readable)")
	}
}

func TestDictInspectNoDictKeyLeak(t *testing.T) {
	d := NewStringDict(map[string]Object{
		"key": &String{Value: "val"},
	})

	inspect := d.Inspect()
	// Should contain human-readable key, not DictKey format
	if contains(inspect, "s:key") {
		t.Errorf("Inspect() should not contain DictKey format: %s", inspect)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDictIterators(t *testing.T) {
	d := NewStringDict(map[string]Object{
		"a": &Integer{Value: 1},
		"b": &Integer{Value: 2},
	})

	t.Run("DictKeys iterator", func(t *testing.T) {
		iter := (&DictKeys{Dict: d}).CreateIterator()
		var keys []string
		for {
			obj, ok := iter.Next()
			if !ok {
				break
			}
			str, ok := obj.(*String)
			if !ok {
				t.Fatalf("DictKeys iterator should return String, got %T", obj)
			}
			// Should NOT have DictKey prefix
			if len(str.Value) > 2 && str.Value[1] == ':' {
				t.Errorf("DictKeys iterator returned DictKey-formatted key: %q", str.Value)
			}
			keys = append(keys, str.Value)
		}
		if len(keys) != 2 {
			t.Errorf("Expected 2 keys, got %d", len(keys))
		}
	})

	t.Run("DictItems iterator", func(t *testing.T) {
		iter := (&DictItems{Dict: d}).CreateIterator()
		count := 0
		for {
			obj, ok := iter.Next()
			if !ok {
				break
			}
			tuple, ok := obj.(*Tuple)
			if !ok {
				t.Fatalf("DictItems iterator should return Tuple, got %T", obj)
			}
			key, ok := tuple.Elements[0].(*String)
			if !ok {
				t.Fatalf("Item key should be String, got %T", tuple.Elements[0])
			}
			if len(key.Value) > 2 && key.Value[1] == ':' {
				t.Errorf("DictItems iterator returned DictKey-formatted key: %q", key.Value)
			}
			count++
		}
		if count != 2 {
			t.Errorf("Expected 2 items, got %d", count)
		}
	})
}

func TestEnvironmentConcurrentSafety(t *testing.T) {
	// Test that Environment operations are safe with RWMutex
	env := NewEnvironment()
	env.Set("x", &Integer{Value: 1})

	// Concurrent reads should not panic
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				env.Get("x")
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	val, ok := env.Get("x")
	if !ok {
		t.Error("Expected to find 'x'")
	}
	if integer, ok := val.(*Integer); !ok || integer.Value != 1 {
		t.Errorf("Expected Integer(1), got %v", val)
	}
}

func TestListAsList(t *testing.T) {
	// Test that AsList returns a copy, not the original slice
	original := &List{Elements: []Object{
		&Integer{Value: 1},
		&Integer{Value: 2},
		&Integer{Value: 3},
	}}

	copy, err := original.AsList()
	if err != nil {
		t.Fatalf("AsList() returned error: %v", err)
	}

	// Modify the copy
	copy = append(copy, &Integer{Value: 4})

	// Original should be unchanged
	if len(original.Elements) != 3 {
		t.Errorf("Original list should still have 3 elements, got %d", len(original.Elements))
	}
}

func TestTupleAsList(t *testing.T) {
	// Test that Tuple.AsList returns a copy
	original := &Tuple{Elements: []Object{
		&Integer{Value: 1},
		&Integer{Value: 2},
	}}

	copy, err := original.AsList()
	if err != nil {
		t.Fatalf("AsList() returned error: %v", err)
	}

	// Modify the copy
	copy = append(copy, &Integer{Value: 3})

	// Original should be unchanged
	if len(original.Elements) != 2 {
		t.Errorf("Original tuple should still have 2 elements, got %d", len(original.Elements))
	}
}

func TestSetOperationsWithDictKey(t *testing.T) {
	s := &Set{Elements: make(map[string]Object)}

	// Add various types
	s.Add(&String{Value: "hello"})
	s.Add(&Integer{Value: 42})
	s.Add(&Boolean{Value: true})

	// Check contains
	if !s.Contains(&String{Value: "hello"}) {
		t.Error("Set should contain 'hello'")
	}
	if !s.Contains(&Integer{Value: 42}) {
		t.Error("Set should contain 42")
	}
	if s.Contains(&String{Value: "missing"}) {
		t.Error("Set should not contain 'missing'")
	}

	// Remove
	s.Remove(&String{Value: "hello"})
	if s.Contains(&String{Value: "hello"}) {
		t.Error("Set should not contain 'hello' after remove")
	}
}

func TestLibraryGetDict(t *testing.T) {
	lib := NewLibrary("test", map[string]*Builtin{
		"test_func": {
			Fn: nil,
		},
	}, map[string]Object{
		"PI": &Float{Value: 3.14},
	}, "test library")

	dict := lib.GetDict()

	// All keys should use DictKey format in the Pairs map
	for mapKey, pair := range dict.Pairs {
		keyStr, _ := pair.Key.AsString()
		expectedMapKey := DictKey(&String{Value: keyStr})
		if mapKey != expectedMapKey {
			t.Errorf("Map key %q does not match expected DictKey %q for key %q", mapKey, expectedMapKey, keyStr)
		}
	}

	// Should be findable via GetByString
	if _, ok := dict.GetByString("test_func"); !ok {
		t.Error("Should find 'test_func' via GetByString")
	}
	if _, ok := dict.GetByString("PI"); !ok {
		t.Error("Should find 'PI' via GetByString")
	}
}
