package scriptling

import (
	"os"
	"testing"
)

func TestBenchmarkScript(t *testing.T) {
	script, err := os.ReadFile("examples/benchmark.py")
	if err != nil {
		t.Fatalf("Failed to read benchmark.py: %v", err)
	}

	p := New("json", "time")
	_, err = p.Eval(string(script))
	if err != nil {
		t.Fatalf("Benchmark script failed: %v", err)
	}
}
