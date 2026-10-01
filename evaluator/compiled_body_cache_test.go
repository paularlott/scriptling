package evaluator

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

// A function body must be compiled once per AST node, not once per time the
// def statement runs. Class bodies still define their methods through the
// fallback evalFunctionStatement, so a class defined inside a loop or a
// function would otherwise recompile every method body on every definition.
func TestCompiledFunctionBodyCachedOnNode(t *testing.T) {
	l := lexer.New("def f(x):\n    return x + 1\n")
	program := parser.New(l).ParseProgram()
	fs, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("expected FunctionStatement, got %T", program.Statements[0])
	}
	if fs.Function.Compiled() != nil {
		t.Fatal("body should not be compiled before first use")
	}
	if compiledFunctionBody(fs.Function) == nil {
		t.Fatal("compiledFunctionBody returned nil")
	}
	if fs.Function.Compiled() == nil {
		t.Fatal("first use should cache the compiled body on the node")
	}
	if allocs := testing.AllocsPerRun(100, func() { compiledFunctionBody(fs.Function) }); allocs != 0 {
		t.Fatalf("compiledFunctionBody allocated %.0f times once cached, want 0", allocs)
	}

	// The compiled def closure must hand the cached body to every function it
	// creates: defining the function repeatedly must not compile again.
	env := object.NewEnvironment()
	define := compileFunctionStatement(fs)
	first := define(context.Background(), env).(*object.Function)
	if first.CompiledBody == nil {
		t.Fatal("compiled def should attach a compiled body")
	}
	allocs := testing.AllocsPerRun(50, func() {
		define(context.Background(), env)
	})
	// One Function object per definition is expected; compiling the body
	// again would add many more.
	if allocs > 2 {
		t.Fatalf("defining a function allocated %.0f times, want at most 2", allocs)
	}
}
