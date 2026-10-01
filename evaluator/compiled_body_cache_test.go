package evaluator

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

func parseDefs(t *testing.T, src string) []*ast.FunctionStatement {
	t.Helper()
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	var defs []*ast.FunctionStatement
	for _, s := range program.Statements {
		if fs, ok := s.(*ast.FunctionStatement); ok {
			defs = append(defs, fs)
		}
	}
	return defs
}

// Function bodies are compiled on the first call, not when the def runs, and
// the closure is cached on the body's AST node: a library that defines many
// functions retains closures only for the ones that are called, and a def
// that runs repeatedly (a class defined in a loop) never recompiles.
func TestFunctionBodyCompiledOnFirstCallAndCached(t *testing.T) {
	defs := parseDefs(t, "def used(x):\n    return x + 1\n\ndef unused(x):\n    return x - 1\n")
	env := object.NewEnvironment()
	ctx := context.Background()

	define := compileFunctionStatement(defs[0])
	fn := define(ctx, env).(*object.Function)
	if defs[0].Function.Body.Compiled.Load() != nil || fn.CompiledBody != nil {
		t.Fatal("defining a function must not compile its body")
	}
	if got := applyUserFunction(ctx, fn, []object.Object{object.NewInteger(1)}, nil, env); got.Inspect() != "2" {
		t.Fatalf("used(1) = %s", got.Inspect())
	}
	if defs[0].Function.Body.Compiled.Load() == nil {
		t.Fatal("the first call must cache the compiled body on the block node")
	}
	if fn.CompiledBody == nil {
		t.Fatal("a compiler-created function memoises the body after the first call")
	}

	compileFunctionStatement(defs[1])(ctx, env)
	if defs[1].Function.Body.Compiled.Load() != nil {
		t.Fatal("a function that is never called must not be compiled")
	}

	// Repeated definitions and repeated calls allocate no compiled bodies.
	if allocs := testing.AllocsPerRun(50, func() { define(ctx, env) }); allocs > 2 {
		t.Fatalf("defining allocated %.0f times per run, want at most the function object", allocs)
	}
	if allocs := testing.AllocsPerRun(100, func() { functionBody(fn) }); allocs != 0 {
		t.Fatalf("functionBody allocated %.0f times once cached, want 0", allocs)
	}
}
