package scriptling

import (
	"fmt"
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

func BenchmarkParseUncached_Import(b *testing.B) {
	benchmarkParseUncached(b, "from alpha.beta.gamma import delta as d, epsilon as e")
}

func BenchmarkParseUncached_AdjacentStrings(b *testing.B) {
	benchmarkParseUncached(b, `result = "alpha" "beta" "gamma" "delta"`)
}

func BenchmarkParseCached_Hit(b *testing.B) {
	script := "def add(a, b):\n    return a + b\nresult = add(5, 3)"
	globalCache = newProgramCache(1000)
	if _, err := parseProgramCached(script); err != nil {
		b.Fatalf("warmup parse failed: %v", err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		program, err := parseProgramCached(script)
		if err != nil {
			b.Fatalf("unexpected parse error: %v", err)
		}
		if program == nil {
			b.Fatal("expected cached program")
		}
	}
}

func BenchmarkParseCached_TinyHit(b *testing.B) {
	script := "x = 5"
	globalCache = newProgramCache(1000)
	if _, err := parseProgramCached(script); err != nil {
		b.Fatalf("warmup parse failed: %v", err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		program, err := parseProgramCached(script)
		if err != nil {
			b.Fatalf("unexpected parse error: %v", err)
		}
		if program == nil {
			b.Fatal("expected cached program")
		}
	}
}

func BenchmarkParseCached_Miss(b *testing.B) {
	globalCache = newProgramCache(1000)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		script := fmt.Sprintf("x = %d\n# miss_%d", i, i)
		program, err := parseProgramCached(script)
		if err != nil {
			b.Fatalf("unexpected parse error: %v", err)
		}
		if program == nil {
			b.Fatal("expected program")
		}
	}
}

func BenchmarkParseCached_WorkingSet(b *testing.B) {
	const workingSet = 1500
	scripts := make([]string, workingSet)
	for i := range scripts {
		scripts[i] = fmt.Sprintf("def f%d(x):\n    return x + %d\nresult = f%d(10)", i, i, i)
	}
	globalCache = newProgramCache(1000)
	for _, script := range scripts {
		if _, err := parseProgramCached(script); err != nil {
			b.Fatalf("warmup parse failed: %v", err)
		}
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		script := scripts[i%workingSet]
		program, err := parseProgramCached(script)
		if err != nil {
			b.Fatalf("unexpected parse error: %v", err)
		}
		if program == nil {
			b.Fatal("expected program")
		}
	}
}
