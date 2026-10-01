package evaluator

import (
	"context"
	"sync"
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

// A function object assembled by a host without a compiled body may be
// shared between independent interpreter trees. Calling it concurrently
// must not race: the body is compiled once and cached on its AST node, and
// the function object itself is never written. Run with -race.
func TestHandBuiltFunctionSharedAcrossTreesIsRaceFree(t *testing.T) {
	l := lexer.New("def f(x):\n    return x * 2\n")
	program := parser.New(l).ParseProgram()
	fs := program.Statements[0].(*ast.FunctionStatement)

	fn := &object.Function{
		Name:             "f",
		Parameters:       fs.Function.Parameters,
		Body:             fs.Function.Body,
		LocalSlots:       fs.Function.LocalSlots,
		LocalSlotNames:   fs.Function.LocalSlotNames,
		ParamSlotIndexes: fs.Function.ParamSlotIndexes,
		// No CompiledBody, no shared frame pool: every call allocates its own
		// environment from the tree it is called in.
		ReuseCallEnv: false,
	}

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			env := object.NewEnvironment()
			local := *fn // each tree holds its own view of the shared definition
			local.Env = env
			for i := 0; i < 50; i++ {
				got := applyUserFunction(context.Background(), &local, []object.Object{object.NewInteger(int64(i))}, nil, env)
				if iv, ok := got.(*object.Integer); !ok || iv.IntValue() != int64(2*i) {
					t.Errorf("worker %d: f(%d) = %v", w, i, got)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	if fn.CompiledBody != nil {
		t.Fatal("the shared function object must not be written")
	}
	if fs.Function.Body.Compiled.Load() == nil {
		t.Fatal("the body closure should be cached on the block node")
	}
}
