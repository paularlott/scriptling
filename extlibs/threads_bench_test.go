package extlibs

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// BenchmarkThreadsRun benchmarks the performance of threads.run
// Measures the overhead of spawning goroutines with environment cloning
func BenchmarkThreadsRun(b *testing.B) {
	// Save original function
	origApply := ApplyFunctionFunc
	defer func() {
		ApplyFunctionFunc = origApply
	}()

	// Simple worker function
	worker := &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Integer{Value: 42}
		},
	}

	ApplyFunctionFunc = func(ctx context.Context, fn object.Object, fnArgs []object.Object, fnKwargs map[string]object.Object, env *object.Environment) object.Object {
		if builtin, ok := fn.(*object.Builtin); ok {
			return builtin.Fn(ctx, object.NewKwargs(fnKwargs), fnArgs...)
		}
		return &object.Error{Message: "not a builtin"}
	}

	// Create environment with libraries (simulating typical usage)
	env := object.NewEnvironment()
	env.Set("json", object.NewLibrary("json", map[string]*object.Builtin{}, map[string]object.Object{}, "json library"))
	env.Set("math", object.NewLibrary("math", map[string]*object.Builtin{}, map[string]object.Object{}, "math library"))
	env.Set("time", object.NewLibrary("time", map[string]*object.Builtin{}, map[string]object.Object{}, "time library"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate threads.run() call
		cloned := cloneEnvironment(env)
		promise := newPromise()
		go func() {
			result := ApplyFunctionFunc(context.Background(), worker, nil, nil, cloned)
			promise.set(result, nil)
		}()
		promise.get()
	}
}

// BenchmarkThreadsRunWithLargeEnvironment benchmarks with many variables in parent
// This shows the benefit of O(1) cloning when the parent has many variables
func BenchmarkThreadsRunWithLargeEnvironment(b *testing.B) {
	// Save original function
	origApply := ApplyFunctionFunc
	defer func() {
		ApplyFunctionFunc = origApply
	}()

	// Simple worker function
	worker := &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Integer{Value: 42}
		},
	}

	ApplyFunctionFunc = func(ctx context.Context, fn object.Object, fnArgs []object.Object, fnKwargs map[string]object.Object, env *object.Environment) object.Object {
		if builtin, ok := fn.(*object.Builtin); ok {
			return builtin.Fn(ctx, object.NewKwargs(fnKwargs), fnArgs...)
		}
		return &object.Error{Message: "not a builtin"}
	}

	// Create environment with many user variables (simulating typical usage)
	env := object.NewEnvironment()
	env.Set("json", object.NewLibrary("json", map[string]*object.Builtin{}, map[string]object.Object{}, "json library"))
	env.Set("math", object.NewLibrary("math", map[string]*object.Builtin{}, map[string]object.Object{}, "math library"))

	// Add 100 user variables (these should NOT be cloned)
	for i := 0; i < 100; i++ {
		env.Set(string(rune('a'+i%26)), &object.Integer{Value: int64(i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate threads.run() call
		cloned := cloneEnvironment(env)
		promise := newPromise()
		go func() {
			result := ApplyFunctionFunc(context.Background(), worker, nil, nil, cloned)
			promise.set(result, nil)
		}()
		promise.get()
	}
}

// BenchmarkCloneEnvironmentOnly benchmarks just the clone operation
func BenchmarkCloneEnvironmentOnly(b *testing.B) {
	env := object.NewEnvironment()
	env.Set("json", object.NewLibrary("json", map[string]*object.Builtin{}, map[string]object.Object{}, "json library"))
	env.Set("math", object.NewLibrary("math", map[string]*object.Builtin{}, map[string]object.Object{}, "math library"))

	// Add many user variables (should NOT affect performance)
	for i := 0; i < 100; i++ {
		env.Set(string(rune('a'+i%26)), &object.Integer{Value: int64(i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloneEnvironment(env)
	}
}

// BenchmarkCloneEnvironmentWithOnlyLibraries benchmarks the ideal case
// Shows performance when only libraries are present (no user variables)
func BenchmarkCloneEnvironmentWithOnlyLibraries(b *testing.B) {
	env := object.NewEnvironment()
	env.Set("json", object.NewLibrary("json", map[string]*object.Builtin{}, map[string]object.Object{}, "json library"))
	env.Set("math", object.NewLibrary("math", map[string]*object.Builtin{}, map[string]object.Object{}, "math library"))
	env.Set("time", object.NewLibrary("time", map[string]*object.Builtin{}, map[string]object.Object{}, "time library"))
	env.Set("regex", object.NewLibrary("regex", map[string]*object.Builtin{}, map[string]object.Object{}, "regex library"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloneEnvironment(env)
	}
}
