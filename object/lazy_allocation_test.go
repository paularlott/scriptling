package object

import (
	"testing"
)

func TestLazyMapAllocation(t *testing.T) {
	t.Run("Environment without global/nonlocal", func(t *testing.T) {
		env := NewEnvironment()
		
		// Maps should be nil initially
		if env.globals != nil {
			t.Error("globals map should be nil initially")
		}
		if env.nonlocals != nil {
			t.Error("nonlocals map should be nil initially")
		}
		
		// Setting a regular variable shouldn't allocate maps
		env.Set("x", &Integer{Value: 42})
		if env.globals != nil {
			t.Error("globals map should still be nil after Set")
		}
		if env.nonlocals != nil {
			t.Error("nonlocals map should still be nil after Set")
		}
	})
	
	t.Run("MarkGlobal allocates map", func(t *testing.T) {
		env := NewEnvironment()
		
		// Mark a variable as global
		env.MarkGlobal("x")
		
		// globals map should now be allocated
		if env.globals == nil {
			t.Error("globals map should be allocated after MarkGlobal")
		}
		if !env.IsGlobal("x") {
			t.Error("x should be marked as global")
		}
		
		// nonlocals should still be nil
		if env.nonlocals != nil {
			t.Error("nonlocals map should still be nil")
		}
	})
	
	t.Run("MarkNonlocal allocates map", func(t *testing.T) {
		env := NewEnvironment()
		
		// Mark a variable as nonlocal
		env.MarkNonlocal("y")
		
		// nonlocals map should now be allocated
		if env.nonlocals == nil {
			t.Error("nonlocals map should be allocated after MarkNonlocal")
		}
		if !env.IsNonlocal("y") {
			t.Error("y should be marked as nonlocal")
		}
		
		// globals should still be nil
		if env.globals != nil {
			t.Error("globals map should still be nil")
		}
	})
	
	t.Run("IsGlobal with nil map", func(t *testing.T) {
		env := NewEnvironment()
		
		// Should return false, not panic
		if env.IsGlobal("x") {
			t.Error("IsGlobal should return false for nil map")
		}
	})
	
	t.Run("IsNonlocal with nil map", func(t *testing.T) {
		env := NewEnvironment()
		
		// Should return false, not panic
		if env.IsNonlocal("x") {
			t.Error("IsNonlocal should return false for nil map")
		}
	})
	
	t.Run("Set with nil maps", func(t *testing.T) {
		env := NewEnvironment()
		
		// Should work without panicking
		result := env.Set("x", &Integer{Value: 42})
		if result == nil {
			t.Error("Set should return a value")
		}
		
		val, ok := env.Get("x")
		if !ok {
			t.Error("Variable should be set")
		}
		if intVal, ok := val.(*Integer); !ok || intVal.Value != 42 {
			t.Error("Variable should have correct value")
		}
	})
}

func TestGlobalVariableBehavior(t *testing.T) {
	t.Run("Global variable modification", func(t *testing.T) {
		// Create root environment
		root := NewEnvironment()
		root.Set("x", &Integer{Value: 10})
		
		// Create nested environment
		nested := NewEnclosedEnvironment(root)
		nested.MarkGlobal("x")
		
		// Modify global variable from nested scope
		nested.Set("x", &Integer{Value: 20})
		
		// Check that root was modified
		val, ok := root.Get("x")
		if !ok {
			t.Fatal("Global variable should exist in root")
		}
		if intVal, ok := val.(*Integer); !ok || intVal.Value != 20 {
			t.Errorf("Global variable should be 20, got %v", intVal.Value)
		}
	})
}

func TestNonlocalVariableBehavior(t *testing.T) {
	t.Run("Nonlocal variable modification", func(t *testing.T) {
		// Create root environment
		root := NewEnvironment()
		
		// Create middle environment
		middle := NewEnclosedEnvironment(root)
		middle.Set("x", &Integer{Value: 10})
		
		// Create nested environment
		nested := NewEnclosedEnvironment(middle)
		nested.MarkNonlocal("x")
		
		// Modify nonlocal variable
		nested.Set("x", &Integer{Value: 20})
		
		// Check that middle was modified, not nested
		val, ok := middle.Get("x")
		if !ok {
			t.Fatal("Nonlocal variable should exist in middle")
		}
		if intVal, ok := val.(*Integer); !ok || intVal.Value != 20 {
			t.Errorf("Nonlocal variable should be 20, got %v", intVal.Value)
		}
		
		// Check that nested doesn't have it locally
		if _, ok := nested.store["x"]; ok {
			t.Error("Nested should not have x in local store")
		}
	})
}

func BenchmarkEnvironmentAllocation(b *testing.B) {
	b.Run("Without global/nonlocal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			env := NewEnvironment()
			env.Set("x", &Integer{Value: 42})
			env.Set("y", &Integer{Value: 43})
		}
	})
	
	b.Run("With global", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			env := NewEnvironment()
			env.MarkGlobal("x")
			env.Set("x", &Integer{Value: 42})
		}
	})
	
	b.Run("With nonlocal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			env := NewEnvironment()
			env.MarkNonlocal("x")
			env.Set("x", &Integer{Value: 42})
		}
	})
}
