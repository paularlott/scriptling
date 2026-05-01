package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

func BenchmarkFibonacci10(b *testing.B) {
	script := `
def fib(n):
    if n <= 1:
        return n
    return fib(n-1) + fib(n-2)

result = fib(10)
`
	p := New()
	stdlib.RegisterAll(p)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := p.Eval(script)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoopSum100k(b *testing.B) {
	script := `
total = 0
for i in range(100000):
    total = total + i
`
	p := New()
	stdlib.RegisterAll(p)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := p.Eval(script)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWhileLoop100k(b *testing.B) {
	script := `
i = 0
total = 0
while i < 100000:
    total = total + i
    i = i + 1
`
	p := New()
	stdlib.RegisterAll(p)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := p.Eval(script)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkArithmeticOnly(b *testing.B) {
	script := `
x = 0
x = x + 1
x = x + 1
x = x + 1
x = x + 1
x = x + 1
x = x + 1
x = x + 1
x = x + 1
x = x + 1
x = x + 1
`
	p := New()
	stdlib.RegisterAll(p)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := p.Eval(script)
		if err != nil {
			b.Fatal(err)
		}
	}
}
