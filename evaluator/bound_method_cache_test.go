package evaluator

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

func TestBoundMethodCacheReusesWrapper(t *testing.T) {
	input := `
class Counter:
    def inc(self):
        return 1

c = Counter()
m1 = c.inc
m2 = c.inc
same = m1 is m2
same
`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	env := object.NewEnvironment()
	ctx := ContextWithCallDepth(context.Background(), DefaultMaxCallDepth)
	result := EvalWithContext(ctx, program, env)

	if object.IsError(result) {
		t.Fatalf("evaluation error: %s", result.Inspect())
	}
	testBooleanObject(t, result, true)
}

func TestBoundMethodCacheInvalidatesOnFieldShadowing(t *testing.T) {
	input := `
class Greeter:
    def greet(self):
        return "method"

g = Greeter()
before = g.greet
g.greet = lambda: "field"
after_set = g.greet()
del g.greet
after_delete = g.greet()
results = [before(), after_set, after_delete]
results
`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	env := object.NewEnvironment()
	ctx := ContextWithCallDepth(context.Background(), DefaultMaxCallDepth)
	result := EvalWithContext(ctx, program, env)

	if object.IsError(result) {
		t.Fatalf("evaluation error: %s", result.Inspect())
	}

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected list, got %T", result)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("expected 3 results, got %d", len(list.Elements))
	}
	testStringObject(t, list.Elements[0], "method")
	testStringObject(t, list.Elements[1], "field")
	testStringObject(t, list.Elements[2], "method")
}
