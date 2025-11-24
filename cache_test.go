package scriptling

import (
	"testing"
)

func BenchmarkEvalWithCache(b *testing.B) {
	p := New()
	script := `
x = 10
y = 20
result = x + y
`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Eval(script)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvalDifferentScripts(b *testing.B) {
	p := New()
	scripts := []string{
		"x = 1 + 1",
		"y = 2 * 2",
		"z = 3 + 3",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		script := scripts[i%len(scripts)]
		_, err := p.Eval(script)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvalMultipleInstances(b *testing.B) {
	script := "result = 5 + 3"
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		p := New()
		for pb.Next() {
			_, err := p.Eval(script)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
