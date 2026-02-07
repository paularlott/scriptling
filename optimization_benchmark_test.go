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
