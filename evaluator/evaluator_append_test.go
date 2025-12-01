package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestAppendInPlace(t *testing.T) {
	input := `
my_list = [1, 2]
my_list.append(3)
my_list
`
	evaluated := testEval(input)
	list, ok := evaluated.(*object.List)
	if !ok {
		t.Fatalf("object is not List. got=%T (%+v)", evaluated, evaluated)
	}
	if len(list.Elements) != 3 {
		t.Errorf("list has wrong length. got=%d, want=3", len(list.Elements))
	}
	testIntegerObject(t, list.Elements[0], 1)
	testIntegerObject(t, list.Elements[1], 2)
	testIntegerObject(t, list.Elements[2], 3)
}

func TestAppendReturnsNone(t *testing.T) {
	input := `
my_list = [1, 2]
result = my_list.append(3)
result
`
	evaluated := testEval(input)
	if evaluated.Type() != object.NULL_OBJ {
		t.Errorf("append should return None. got=%T (%+v)", evaluated, evaluated)
	}
}

func TestAppendMultipleTimes(t *testing.T) {
	input := `
my_list = []
my_list.append(1)
my_list.append(2)
my_list.append(3)
len(my_list)
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 3)
}
