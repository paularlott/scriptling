package evaluator

import (
	"os"
	"runtime"
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/parser"
)

func liveHeapBytes() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// The parse cache budgets entries with ast.EstimateRetainedBytes, which must
// cover both the AST and the compiled closure tree the evaluator caches on
// the program. This measures the real retained size of parsed-and-compiled
// programs and checks the estimate stays close, so neither a new node field
// nor a change in how closures are built can quietly let the cache hold far
// more memory than its limit says.
func TestRetainedEstimateCoversCompiledProgram(t *testing.T) {
	for _, file := range []string{"../tests/test_scripts/closures.py", "../tests/test_scripts/assert_basic.py"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Skipf("%s: %v", file, err)
		}
		const n = 100
		base := liveHeapBytes()
		programs := make([]*ast.Program, n)
		for i := range programs {
			p := parser.New(lexer.New(string(src)))
			program := p.ParseProgram()
			if errs := p.Errors(); len(errs) != 0 {
				t.Fatalf("%s: %v", file, errs)
			}
			ast.FoldConstants(program)
			program.SetCompiled(compileProgram(program))
			programs[i] = program
		}
		actual := float64(liveHeapBytes()-base) / n
		estimate := float64(ast.EstimateRetainedBytes(programs[0], ""))
		ratio := estimate / actual
		t.Logf("%s: measured %.0f bytes, estimated %.0f bytes (%.0f%%)", file, actual, estimate, 100*ratio)
		if ratio < 0.8 || ratio > 1.25 {
			t.Errorf("%s: estimate is %.0f%% of measured retained size, want 80%%-125%%", file, 100*ratio)
		}
		runtime.KeepAlive(programs)
	}
}
