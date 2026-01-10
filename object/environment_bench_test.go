package object

import (
	"testing"
)

// Benchmark baseline environment operations
// These tests measure the performance of Get/Set operations
// which are called frequently during script execution

func BenchmarkEnvironmentGet(b *testing.B) {
	env := NewEnvironment()
	// Pre-populate with some variables
	env.Set("x", &Integer{Value: 1})
	env.Set("y", &Integer{Value: 2})
	env.Set("z", &Integer{Value: 3})
	env.Set("name", &String{Value: "test"})
	env.Set("flag", &Boolean{Value: true})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.Get("x")
		env.Get("y")
		env.Get("z")
		env.Get("name")
		env.Get("flag")
	}
}

func BenchmarkEnvironmentSet(b *testing.B) {
	env := NewEnvironment()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.Set("x", &Integer{Value: int64(i)})
		env.Set("y", &Integer{Value: int64(i + 1)})
		env.Set("z", &Integer{Value: int64(i + 2)})
	}
}

func BenchmarkEnvironmentGetSet(b *testing.B) {
	env := NewEnvironment()
	env.Set("counter", &Integer{Value: 0})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		val, _ := env.Get("counter")
		if intVal, ok := val.(*Integer); ok {
			env.Set("counter", &Integer{Value: intVal.Value + 1})
		}
	}
}

// Benchmark with nested environments (simulating function scopes)
func BenchmarkEnvironmentNestedGet(b *testing.B) {
	global := NewEnvironment()
	global.Set("global_var", &Integer{Value: 100})

	outer := NewEnclosedEnvironment(global)
	outer.Set("outer_var", &Integer{Value: 50})

	inner := NewEnclosedEnvironment(outer)
	inner.Set("inner_var", &Integer{Value: 25})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inner.Get("inner_var")   // Direct access
		inner.Get("outer_var")    // One level up
		inner.Get("global_var")   // Two levels up
	}
}

// Benchmark concurrent access (simulating goroutines)
func BenchmarkEnvironmentConcurrent(b *testing.B) {
	env := NewEnvironment()
	env.Set("x", &Integer{Value: 1})
	env.Set("y", &Integer{Value: 2})
	env.Set("z", &Integer{Value: 3})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			env.Get("x")
			env.Get("y")
			env.Get("z")
		}
	})
}

// Benchmark with many variables (simulating complex scope)
func BenchmarkEnvironmentManyVars(b *testing.B) {
	env := NewEnvironment()
	// Create 100 variables
	for i := 0; i < 100; i++ {
		env.Set(string(rune('a'+i%26)), &Integer{Value: int64(i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.Get("a")
		env.Get("z")
		env.Get("m")
	}
}
