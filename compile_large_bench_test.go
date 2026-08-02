package scriptling

import (
	"os"
	"testing"
)

// Compile-time benchmarks over a realistic file rather than one-liners, so lexer
// and parser costs are measured at a scale where per-file overheads (symbol
// interning, node allocation, slot analysis) actually show up.

func benchmarkParseFile(b *testing.B, path string) {
	b.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		b.Skipf("fixture not available: %v", err)
	}
	text := string(src)
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		program, err := parseProgramUncached(text)
		if err != nil {
			b.Fatalf("parse error: %v", err)
		}
		if program == nil {
			b.Fatal("expected parsed program")
		}
	}
}

func BenchmarkParseUncached_LargeFile(b *testing.B) {
	benchmarkParseFile(b, "tests/test_scripts/integration_patterns.py")
}
