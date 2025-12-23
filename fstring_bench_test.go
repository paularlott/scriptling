package scriptling

import (
	"testing"
)

// BenchmarkFStringSimple benchmarks simple f-string evaluation
func BenchmarkFStringSimple(b *testing.B) {
	s := New()
	script := `f"Hello {name}!"`
	s.SetVar("name", "World")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Eval(script)
	}
}

// BenchmarkFStringMultiple benchmarks f-string with multiple expressions
func BenchmarkFStringMultiple(b *testing.B) {
	s := New()
	script := `f"Hello {name}, you are {age} years old and your score is {score}."`
	s.SetVar("name", "Alice")
	s.SetVar("age", 30)
	s.SetVar("score", 95)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Eval(script)
	}
}

// BenchmarkFStringComplex benchmarks f-string with format specs
func BenchmarkFStringComplex(b *testing.B) {
	s := New()
	script := `f"Value: {value:02d}, Price: ${price:.2f}, Name: {name}"`
	s.SetVar("value", 42)
	s.SetVar("price", 19.99)
	s.SetVar("name", "Widget")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Eval(script)
	}
}

// BenchmarkStringConcat benchmarks traditional string concatenation
func BenchmarkStringConcat(b *testing.B) {
	s := New()
	script := `"Hello " + name + "!"`
	s.SetVar("name", "World")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Eval(script)
	}
}

// BenchmarkFStringVsConcat compares f-string vs concatenation
func BenchmarkFStringVsConcat(b *testing.B) {
	s := New()
	s.SetVar("name", "World")
	s.SetVar("age", 30)

	b.Run("FString", func(b *testing.B) {
		script := `f"Hello {name}, age {age}"`
		for i := 0; i < b.N; i++ {
			s.Eval(script)
		}
	})

	b.Run("Concat", func(b *testing.B) {
		script := `"Hello " + name + ", age " + str(age)`
		for i := 0; i < b.N; i++ {
			s.Eval(script)
		}
	})
}
