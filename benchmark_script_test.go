package scriptling

import (
	"os"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

func TestBenchmarkScript(t *testing.T) {
	script, err := os.ReadFile("examples/scripts/benchmark.py")
	if err != nil {
		t.Fatalf("Failed to read benchmark.py: %v", err)
	}

	p := New()
	stdlib.RegisterAll(p)
	_, err = p.Eval(string(script))
	if err != nil {
		t.Fatalf("Benchmark script failed: %v", err)
	}
}
