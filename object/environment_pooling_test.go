package object

import (
	"testing"
)

func TestEnvironmentPooling(t *testing.T) {
	// Test that environments are properly reused
	env1 := getEnvironment(nil)
	env1.Set("x", &Integer{Value: 10})
	
	// Return to pool
	PutEnvironment(env1)
	
	// Get another environment - should be the same object
	env2 := getEnvironment(nil)
	
	// Should be cleared
	if _, ok := env2.Get("x"); ok {
		t.Error("Environment not properly cleared after pooling")
	}
	
	// Should have nil references
	if env2.outer != nil {
		t.Error("outer not cleared")
	}
	if env2.globals != nil {
		t.Error("globals not cleared")
	}
	if env2.nonlocals != nil {
		t.Error("nonlocals not cleared")
	}
}

func TestEnvironmentPoolingWithOuter(t *testing.T) {
	parent := NewEnvironment()
	parent.Set("parent_var", &Integer{Value: 42})
	
	child := getEnvironment(parent)
	child.Set("child_var", &Integer{Value: 10})
	
	// Child should see parent vars
	if val, ok := child.Get("parent_var"); !ok {
		t.Error("Child can't see parent variable")
	} else if intVal, ok := val.(*Integer); !ok || intVal.Value != 42 {
		t.Error("Wrong parent variable value")
	}
	
	// Return child to pool
	PutEnvironment(child)
	
	// Get new child
	child2 := getEnvironment(parent)
	
	// Should not see old child vars
	if _, ok := child2.Get("child_var"); ok {
		t.Error("New child sees old child variables")
	}
	
	// Should still see parent vars
	if val, ok := child2.Get("parent_var"); !ok {
		t.Error("New child can't see parent variable")
	} else if intVal, ok := val.(*Integer); !ok || intVal.Value != 42 {
		t.Error("Wrong parent variable value in new child")
	}
}

func TestEnvironmentPoolingConcurrent(t *testing.T) {
	// Test that pooling is thread-safe
	done := make(chan bool)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				env := getEnvironment(nil)
				env.Set("test", &Integer{Value: int64(id)})
				
				// Verify we can read what we wrote
				if val, ok := env.Get("test"); !ok {
					t.Error("Can't read variable we just set")
				} else if intVal, ok := val.(*Integer); !ok || intVal.Value != int64(id) {
					t.Error("Wrong variable value")
				}
				
				PutEnvironment(env)
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestEnvironmentPoolingWithGlobalsNonlocals(t *testing.T) {
	env := getEnvironment(nil)
	
	// Mark some variables as global/nonlocal
	env.MarkGlobal("x")
	env.MarkNonlocal("y")
	
	if !env.IsGlobal("x") {
		t.Error("Variable not marked as global")
	}
	if !env.IsNonlocal("y") {
		t.Error("Variable not marked as nonlocal")
	}
	
	// Return to pool
	PutEnvironment(env)
	
	// Get new environment
	env2 := getEnvironment(nil)
	
	// Should not have old global/nonlocal markers
	if env2.IsGlobal("x") {
		t.Error("New environment has old global marker")
	}
	if env2.IsNonlocal("y") {
		t.Error("New environment has old nonlocal marker")
	}
}

func BenchmarkEnvironmentPooling(b *testing.B) {
	b.Run("WithoutPooling", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			env := &Environment{
				store: make(map[string]Object, 4),
			}
			env.Set("x", &Integer{Value: int64(i)})
			_ = env
		}
	})
	
	b.Run("WithPooling", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			env := getEnvironment(nil)
			env.Set("x", &Integer{Value: int64(i)})
			PutEnvironment(env)
		}
	})
}
