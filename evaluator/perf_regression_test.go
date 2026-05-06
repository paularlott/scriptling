package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

func TestStringConcatChainDoesNotDoubleExecuteSideEffects(t *testing.T) {
	input := `
count = 0

def next_value():
    global count
    count = count + 1
    return "x"

result = next_value() + "y" + 1
`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()

	result := Eval(program, env)
	if !object.IsError(result) {
		t.Fatalf("expected error result, got=%T (%+v)", result, result)
	}

	count, ok := env.Get("count")
	if !ok {
		t.Fatal("expected count to be set")
	}
	testIntegerObject(t, count, 1)
}

func TestSlotCacheDoesNotCaptureOuterLookupAsLocal(t *testing.T) {
	input := `
x = "outer"

def demo():
    first = x
    x = "inner"
    second = x
    return [first, second]

demo()
`

	evaluated := testEval(input)
	list, ok := evaluated.(*object.List)
	if !ok {
		t.Fatalf("object is not List. got=%T (%+v)", evaluated, evaluated)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("list has wrong length. got=%d, want=2", len(list.Elements))
	}

	first, ok := list.Elements[0].(*object.String)
	if !ok {
		t.Fatalf("first element is not String. got=%T", list.Elements[0])
	}
	second, ok := list.Elements[1].(*object.String)
	if !ok {
		t.Fatalf("second element is not String. got=%T", list.Elements[1])
	}
	if first.Value != "outer" {
		t.Fatalf("wrong first value. got=%q want=%q", first.Value, "outer")
	}
	if second.Value != "inner" {
		t.Fatalf("wrong second value. got=%q want=%q", second.Value, "inner")
	}
}
