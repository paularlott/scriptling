package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

func TestErrorLineNumber(t *testing.T) {
	input := `
x = 1
y = "2"
z = x + y
`
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()

	result := Eval(program, env)

	err, ok := result.(*object.Error)
	if !ok {
		t.Fatalf("expected error, got %T (%+v)", result, result)
	}

	if err.Line != 4 {
		t.Errorf("expected error at line 4, got %d. Error: %s", err.Line, err.Message)
	}
}

func TestErrorLineNumberInFunction(t *testing.T) {
	input := `
def foo():
    return 1 + "2"

foo()
`
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()

	result := Eval(program, env)

	err, ok := result.(*object.Error)
	if !ok {
		t.Fatalf("expected error, got %T (%+v)", result, result)
	}

	// The error happens inside foo at line 3.
	if err.Line != 3 {
		t.Errorf("expected error at line 3, got %d. Error: %s", err.Line, err.Message)
	}

	if err.Function != "foo" {
		t.Errorf("expected error in function 'foo', got '%s'", err.Function)
	}
}
