package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/token"
)

func benchmarkLexAll(b *testing.B, input string) {
	b.Helper()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l := lexer.New(input)
		for {
			if tok := l.NextToken(); tok.Type == token.EOF {
				break
			}
		}
	}
}

func benchmarkParseUncached(b *testing.B, input string) {
	b.Helper()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		program, err := parseProgramUncached(input)
		if err != nil {
			b.Fatalf("unexpected parse error: %v", err)
		}
		if program == nil {
			b.Fatal("expected parsed program")
		}
	}
}

func BenchmarkLex_Simple(b *testing.B) {
	benchmarkLexAll(b, "x = 5")
}

func BenchmarkLex_Function(b *testing.B) {
	benchmarkLexAll(b, "def add(a, b):\n    return a + b\nresult = add(5, 3)")
}

func BenchmarkParseUncached_Simple(b *testing.B) {
	benchmarkParseUncached(b, "x = 5")
}

func BenchmarkParseUncached_Function(b *testing.B) {
	benchmarkParseUncached(b, "def add(a, b):\n    return a + b\nresult = add(5, 3)")
}

func BenchmarkParseUncached_Loop(b *testing.B) {
	benchmarkParseUncached(b, "for i in [1, 2, 3, 4, 5]:\n    x = i * 2")
}

func BenchmarkParseUncached_Complex(b *testing.B) {
	benchmarkParseUncached(b, "def fib(n):\n    if n <= 1:\n        return n\n    return fib(n-1) + fib(n-2)\nresult = fib(10)")
}
