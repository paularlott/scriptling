package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestAppendBuiltinFunctionFastPath(t *testing.T) {
	input := `
x = []
append(x, 1)
append(x, 2)
x
`
	evaluated := testEval(input)
	list, ok := evaluated.(*object.List)
	if !ok {
		t.Fatalf("object is not List. got=%T (%+v)", evaluated, evaluated)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("list has wrong length. got=%d, want=2", len(list.Elements))
	}
	testIntegerObject(t, list.Elements[0], 1)
	testIntegerObject(t, list.Elements[1], 2)
}

func TestAppendBuiltinFunctionRespectsShadowing(t *testing.T) {
	input := `
def append(x, y):
    return 42

append([], 1)
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 42)
}

func TestListComprehensionFastPathWithCondition(t *testing.T) {
	input := `
offset = 3
result = [i + offset for i in range(5) if i % 2 == 0]
result
`
	evaluated := testEval(input)
	list, ok := evaluated.(*object.List)
	if !ok {
		t.Fatalf("object is not List. got=%T (%+v)", evaluated, evaluated)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("list has wrong length. got=%d, want=3", len(list.Elements))
	}
	testIntegerObject(t, list.Elements[0], 3)
	testIntegerObject(t, list.Elements[1], 5)
	testIntegerObject(t, list.Elements[2], 7)
}
