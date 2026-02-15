package evaluator

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

func TestBoundMethodAsCallback(t *testing.T) {
	input := `
class Handler:
    def __init__(self):
        self.called = False
        self.args = None
    
    def handle(self, arg1, arg2):
        self.called = True
        self.args = [arg1, arg2]
        return "handled"

def call_callback(callback, a, b):
    return callback(a, b)

h = Handler()
result = call_callback(h.handle, "test1", "test2")
results = [result, h.called, h.args]
results
`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	
	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	env := object.NewEnvironment()
	ctx := ContextWithCallDepth(context.Background(), DefaultMaxCallDepth)
	result := EvalWithContext(ctx, program, env)

	if object.IsError(result) {
		t.Fatalf("Evaluation error: %s", result.Inspect())
	}

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("Expected list, got %T", result)
	}

	if len(list.Elements) != 3 {
		t.Fatalf("Expected 3 elements, got %d", len(list.Elements))
	}

	// Check result
	str, ok := list.Elements[0].(*object.String)
	if !ok || str.Value != "handled" {
		t.Errorf("Expected 'handled', got %v", list.Elements[0].Inspect())
	}

	// Check called flag
	called, ok := list.Elements[1].(*object.Boolean)
	if !ok || !called.Value {
		t.Errorf("Expected called=true, got %v", list.Elements[1].Inspect())
	}

	// Check args
	argsList, ok := list.Elements[2].(*object.List)
	if !ok {
		t.Errorf("Expected args list, got %T", list.Elements[2])
	} else if len(argsList.Elements) != 2 {
		t.Errorf("Expected 2 args, got %d", len(argsList.Elements))
	}
}

func TestBoundMethodWithKwargs(t *testing.T) {
	input := `
class Handler:
    def __init__(self, prefix):
        self.prefix = prefix
    
    def handle(self, bot, update):
        return self.prefix + ": " + bot + " " + update

def poll_updates(callback, timeout=10):
    # Simulate calling the callback with bot and update
    return callback("bot_obj", "update_obj")

h = Handler("BOT")
result = poll_updates(h.handle, timeout=120)
result
`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	
	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	env := object.NewEnvironment()
	ctx := ContextWithCallDepth(context.Background(), DefaultMaxCallDepth)
	result := EvalWithContext(ctx, program, env)

	if object.IsError(result) {
		t.Fatalf("Evaluation error: %s", result.Inspect())
	}

	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("Expected string, got %T", result)
	}

	expected := "BOT: bot_obj update_obj"
	if str.Value != expected {
		t.Errorf("Expected %q, got %q", expected, str.Value)
	}
}

func TestBoundMethodVsLambda(t *testing.T) {
	// Test that both bound method and lambda wrapper work the same
	input := `
class Handler:
    def __init__(self, name):
        self.name = name
    
    def handle(self, x, y):
        return self.name + ": " + str(x) + " " + str(y)

def call_with_args(callback, a, b):
    return callback(a, b)

h = Handler("TEST")

# Direct bound method
result1 = call_with_args(h.handle, 1, 2)

# Lambda wrapper (current workaround)
result2 = call_with_args(lambda x, y: h.handle(x, y), 1, 2)

results = [result1, result2, result1 == result2]
results
`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	
	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	env := object.NewEnvironment()
	ctx := ContextWithCallDepth(context.Background(), DefaultMaxCallDepth)
	result := EvalWithContext(ctx, program, env)

	if object.IsError(result) {
		t.Fatalf("Evaluation error: %s", result.Inspect())
	}

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("Expected list, got %T", result)
	}

	// Both should produce the same result
	str1, ok1 := list.Elements[0].(*object.String)
	str2, ok2 := list.Elements[1].(*object.String)
	
	if !ok1 || !ok2 {
		t.Fatalf("Expected strings, got %T and %T", list.Elements[0], list.Elements[1])
	}

	if str1.Value != str2.Value {
		t.Errorf("Bound method and lambda should produce same result:\n  bound: %q\n  lambda: %q", str1.Value, str2.Value)
	}

	// Check equality
	equal, ok := list.Elements[2].(*object.Boolean)
	if !ok || !equal.Value {
		t.Errorf("Results should be equal")
	}
}
