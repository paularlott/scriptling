package extlibs

import (
	"context"
	"testing"
	"time"

	"github.com/paularlott/scriptling/object"
)

func TestCloneEnvironment(t *testing.T) {
	// Create a base environment with some variables
	env := object.NewEnvironment()
	env.Set("test_var", &object.Integer{Value: 42})
	env.Set("test_str", &object.String{Value: "hello"})

	// Add an atomic (builtin) object
	atomic := newAtomicInt64(10)
	atomicObj := &object.Builtin{
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) == 0 {
				return &object.Integer{Value: atomic.get()}
			}
			if len(args) == 1 {
				if val, ok := args[0].(*object.Integer); ok {
					atomic.set(val.Value)
					return &object.Null{}
				}
			}
			return &object.Error{Message: "invalid arguments"}
		},
	}
	env.Set("atomic_var", atomicObj)

	// Clone the environment
	cloned := cloneEnvironment(env)

	// Test that regular variables are deep copied
	originalVal, _ := env.Get("test_var")
	clonedVal, _ := cloned.Get("test_var")

	if clonedVal.Inspect() != originalVal.Inspect() {
		t.Errorf("Expected cloned value %s, got %s", originalVal.Inspect(), clonedVal.Inspect())
	}

	// Modify cloned value
	cloned.Set("test_var", &object.Integer{Value: 100})

	// Original should be unchanged
	if val, _ := env.Get("test_var"); val.Inspect() != "42" {
		t.Errorf("Original value should be 42, got %s", val.Inspect())
	}
}

func TestPoolWithSharedState(t *testing.T) {
	// Save original function
	origApply := ApplyFunctionFunc
	defer func() {
		ApplyFunctionFunc = origApply
	}()

	// Track executions using an atomic counter
	counter := newAtomicInt64(0)
	env := object.NewEnvironment()

	// Create a worker function that increments the counter
	worker := &object.Builtin{
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			counter.add(1)
			return &object.Null{}
		},
	}

	// Mock ApplyFunctionFunc to call the builtin function directly
	ApplyFunctionFunc = func(ctx context.Context, fn object.Object, args []object.Object, kwargs map[string]object.Object, env *object.Environment) object.Object {
		if builtin, ok := fn.(*object.Builtin); ok {
			return builtin.Fn(ctx, kwargs, args...)
		}
		return &object.Error{Message: "not a builtin"}
	}

	// Create a pool
	ctx := context.Background()
	pool := newPool(ctx, worker, env, 2, 10)

	// Submit tasks
	for i := 0; i < 5; i++ {
		pool.submit(&object.Integer{Value: int64(i)})
	}

	// Close and wait
	pool.close()

	// Check that all tasks were executed
	if counter.get() != 5 {
		t.Errorf("Expected 5 tasks executed, got %d", counter.get())
	}
}

func TestEnvironmentSetInParentRaceCondition(t *testing.T) {
	// Test the fixed SetInParent method for race conditions
	parent := object.NewEnvironment()
	child := object.NewEnclosedEnvironment(parent)

	// Set initial value in parent
	parent.Set("test", &object.Integer{Value: 1})

	// Use SetInParent concurrently
	done := make(chan bool, 2)

	go func() {
		child.SetInParent("test", &object.Integer{Value: 2})
		done <- true
	}()

	go func() {
		child.SetInParent("test", &object.Integer{Value: 3})
		done <- true
	}()

	// Wait for both to complete
	<-done
	<-done

	// Value should be either 2 or 3 (no race condition panic)
	val, _ := parent.Get("test")
	if val.Inspect() != "2" && val.Inspect() != "3" {
		t.Errorf("Expected value to be 2 or 3, got %s", val.Inspect())
	}
}

func TestPoolContextCancellation(t *testing.T) {
	env := object.NewEnvironment()

	worker := &object.Builtin{
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// Simulate work
			select {
			case <-ctx.Done():
				return &object.Error{Message: "cancelled"}
			case <-time.After(100 * time.Millisecond):
				return &object.Integer{Value: 1}
			}
		},
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	pool := newPool(ctx, worker, env, 2, 10)

	// Submit one task
	pool.submit(&object.Integer{Value: 1})

	// Cancel context
	cancel()

	// Close pool - should not block
	pool.close()
}