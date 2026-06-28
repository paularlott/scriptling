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
	env.Set("x", NewInteger(1))
	env.Set("y", NewInteger(2))
	env.Set("z", NewInteger(3))
	env.Set("name", NewString("test"))
	env.Set("flag", NewBoolean(true))

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
		env.Set("x", NewInteger(int64(i)))
		env.Set("y", NewInteger(int64(i + 1)))
		env.Set("z", NewInteger(int64(i + 2)))
	}
}

func BenchmarkEnvironmentGetSet(b *testing.B) {
	env := NewEnvironment()
	env.Set("counter", NewInteger(0))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		val, _ := env.Get("counter")
		if intVal, ok := val.(*Integer); ok {
			env.Set("counter", NewInteger(intVal.IntValue()+1))
		}
	}
}

// Benchmark with nested environments (simulating function scopes)
func BenchmarkEnvironmentNestedGet(b *testing.B) {
	global := NewEnvironment()
	global.Set("global_var", NewInteger(100))

	outer := NewEnclosedEnvironment(global)
	outer.Set("outer_var", NewInteger(50))

	inner := NewEnclosedEnvironment(outer)
	inner.Set("inner_var", NewInteger(25))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inner.Get("inner_var")  // Direct access
		inner.Get("outer_var")  // One level up
		inner.Get("global_var") // Two levels up
	}
}

func BenchmarkEnvironmentSlotGet(b *testing.B) {
	env := NewEnclosedEnvironmentWithSlots(NewEnvironment(), map[string]int{
		"x":    0,
		"y":    1,
		"name": 2,
	}, []string{"x", "y", "name"})
	env.Set("x", NewInteger(1))
	env.Set("y", NewInteger(2))
	env.Set("name", NewString("test"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.Get("x")
		env.Get("y")
		env.Get("name")
	}
}

func BenchmarkEnvironmentSlotSet(b *testing.B) {
	env := NewEnclosedEnvironmentWithSlots(NewEnvironment(), map[string]int{
		"x": 0,
		"y": 1,
		"z": 2,
	}, []string{"x", "y", "z"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.Set("x", NewInteger(int64(i)))
		env.Set("y", NewInteger(int64(i + 1)))
		env.Set("z", NewInteger(int64(i + 2)))
	}
}

func BenchmarkClassLookupMemberHot(b *testing.B) {
	base := &Class{
		Name:    "Base",
		Methods: map[string]Object{"work": &Builtin{}},
	}
	child := &Class{
		Name:      "Child",
		BaseClass: base,
		Methods:   map[string]Object{},
	}
	child.LookupMember("work")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		child.LookupMember("work")
	}
}

func BenchmarkClassLookupMemberCold(b *testing.B) {
	base := &Class{
		Name:    "Base",
		Methods: map[string]Object{"work": &Builtin{}},
	}
	child := &Class{
		Name:      "Child",
		BaseClass: base,
		Methods:   map[string]Object{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		base.InvalidateLookupCache()
		child.LookupMember("work")
	}
}

func BenchmarkInstanceGetBoundMethodHot(b *testing.B) {
	method := &Builtin{}
	instance := NewInstanceWithFields(&Class{
			Name:    "Worker",
			Methods: map[string]Object{"work": method},
		}, nil)
	instance.GetBoundMethod("work", method)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance.GetBoundMethod("work", method)
	}
}

func BenchmarkInstanceGetBoundMethodCold(b *testing.B) {
	method := &Builtin{}
	instance := NewInstanceWithFields(&Class{
			Name:    "Worker",
			Methods: map[string]Object{"work": method},
		}, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance.InvalidateBoundMethod("work")
		instance.GetBoundMethod("work", method)
	}
}

// Benchmark concurrent access (simulating goroutines)
func BenchmarkEnvironmentConcurrent(b *testing.B) {
	env := NewEnvironment()
	env.Set("x", NewInteger(1))
	env.Set("y", NewInteger(2))
	env.Set("z", NewInteger(3))

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
		env.Set(string(rune('a'+i%26)), NewInteger(int64(i)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.Get("a")
		env.Get("z")
		env.Get("m")
	}
}
