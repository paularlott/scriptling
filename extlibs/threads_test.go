package extlibs

import (
	"context"
	"testing"
	"time"

	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

func TestCloneEnvironment(t *testing.T) {
	// Create a base environment with some variables
	env := object.NewEnvironment()
	env.Set("test_var", &object.Integer{Value: 42})
	env.Set("test_str", &object.String{Value: "hello"})

	// Add a library to verify it gets copied
	testLib := object.NewLibrary(
		"test_lib",
		map[string]*object.Builtin{
			"test_func": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return &object.String{Value: "test"}
				},
			},
		},
		map[string]object.Object{},
		"Test library",
	)
	env.Set("test_lib", testLib)

	// Clone the environment
	cloned := cloneEnvironment(env)

	// User variables should NOT be copied (new behavior)
	_, ok := cloned.Get("test_var")
	if ok {
		t.Error("User variables should NOT be copied to thread environment")
	}

	_, ok = cloned.Get("test_str")
	if ok {
		t.Error("User variables should NOT be copied to thread environment")
	}

	// Libraries SHOULD be copied
	lib, ok := cloned.Get("test_lib")
	if !ok {
		t.Error("Libraries should be copied to thread environment")
	} else if _, isLib := lib.(*object.Library); !isLib {
		t.Error("Library should remain a Library type")
	}
}

func TestPoolWithSharedState(t *testing.T) {
	// Track executions using an atomic counter
	counter := newAtomicInt64(0)
	env := object.NewEnvironment()

	// Create a worker function that increments the counter
	worker := &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			counter.add(1)
			return &object.Null{}
		},
	}

	// Create mock evaluator
	mockEval := &mockEvaluator{}
	ctx := evaliface.WithEvaluator(context.Background(), mockEval)

	// Create a pool
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
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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

func TestPromiseResultPositionalAndKwargs(t *testing.T) {
	// Create a worker function that returns positional + kwargs
	worker := &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			result := int64(0)
			for _, arg := range args {
				if i, ok := arg.(*object.Integer); ok {
					result += i.Value
				}
			}
			for _, key := range kwargs.Keys() {
				arg := kwargs.Get(key)
				if i, ok := arg.(*object.Integer); ok {
					result += i.Value
				}
			}
			return &object.Integer{Value: result}
		},
	}

	// Create mock evaluator
	mockEval := &mockEvaluator{}
	ctx := evaliface.WithEvaluator(context.Background(), mockEval)
	env := object.NewEnvironment()

	// Test with positional arguments
	promise1 := newPromise()
	go func() {
		result := mockEval.CallObjectFunction(ctx, worker, []object.Object{
			&object.Integer{Value: 10},
			&object.Integer{Value: 20},
		}, nil, env)
		promise1.set(result, nil)
	}()

	result1, err1 := promise1.get()
	if err1 != nil {
		t.Fatalf("Promise get failed: %v", err1)
	}
	if i, ok := result1.(*object.Integer); !ok || i.Value != 30 {
		t.Errorf("Expected 30 from positional args, got %v", result1)
	}

	// Test with keyword arguments
	promise2 := newPromise()
	go func() {
		result := mockEval.CallObjectFunction(ctx, worker, nil, map[string]object.Object{
			"a": &object.Integer{Value: 5},
			"b": &object.Integer{Value: 15},
		}, env)
		promise2.set(result, nil)
	}()

	result2, err2 := promise2.get()
	if err2 != nil {
		t.Fatalf("Promise get failed: %v", err2)
	}
	if i, ok := result2.(*object.Integer); !ok || i.Value != 20 {
		t.Errorf("Expected 20 from kwargs, got %v", result2)
	}

	// Test with both positional and keyword arguments
	promise3 := newPromise()
	go func() {
		result := mockEval.CallObjectFunction(ctx, worker, []object.Object{
			&object.Integer{Value: 100},
		}, map[string]object.Object{
			"x": &object.Integer{Value: 10},
		}, env)
		promise3.set(result, nil)
	}()

	result3, err3 := promise3.get()
	if err3 != nil {
		t.Fatalf("Promise get failed: %v", err3)
	}
	if i, ok := result3.(*object.Integer); !ok || i.Value != 110 {
		t.Errorf("Expected 110 from both args, got %v", result3)
	}
}

// mockEvaluator for testing
type mockEvaluator struct{}

func (m *mockEvaluator) CallFunction(ctx context.Context, fn *object.Function, args []object.Object, kwargs map[string]object.Object) object.Object {
	return &object.Null{}
}

func (m *mockEvaluator) CallObjectFunction(ctx context.Context, fn object.Object, args []object.Object, kwargs map[string]object.Object, env *object.Environment) object.Object {
	if builtin, ok := fn.(*object.Builtin); ok {
		return builtin.Fn(ctx, object.NewKwargs(kwargs), args...)
	}
	return &object.Error{Message: "not a builtin"}
}

func (m *mockEvaluator) CallMethod(ctx context.Context, instance *object.Instance, method *object.Function, args []object.Object) object.Object {
	return &object.Null{}
}