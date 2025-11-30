package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestRangeFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected []int64
	}{
		{"list(range(5))", []int64{0, 1, 2, 3, 4}},
		{"list(range(2, 5))", []int64{2, 3, 4}},
		{"list(range(0, 10, 2))", []int64{0, 2, 4, 6, 8}},
		{"list(range(10, 0, -2))", []int64{10, 8, 6, 4, 2}},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		list, ok := evaluated.(*object.List)
		if !ok {
			t.Errorf("object is not List. got=%T (%+v)", evaluated, evaluated)
			continue
		}
		if len(list.Elements) != len(tt.expected) {
			t.Errorf("wrong number of elements. got=%d, want=%d", len(list.Elements), len(tt.expected))
			continue
		}
		for i, expectedVal := range tt.expected {
			testIntegerObject(t, list.Elements[i], expectedVal)
		}
	}
}

func TestSliceNotation(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"[1, 2, 3, 4, 5][1:3]", []int64{2, 3}},
		{"[1, 2, 3, 4, 5][:3]", []int64{1, 2, 3}},
		{"[1, 2, 3, 4, 5][2:]", []int64{3, 4, 5}},
		{`"hello"[1:4]`, "ell"},
		{`"hello"[:2]`, "he"},
		{`"hello"[2:]`, "llo"},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		switch expected := tt.expected.(type) {
		case []int64:
			list, ok := evaluated.(*object.List)
			if !ok {
				t.Errorf("object is not List. got=%T (%+v)", evaluated, evaluated)
				continue
			}
			if len(list.Elements) != len(expected) {
				t.Errorf("wrong number of elements. got=%d, want=%d", len(list.Elements), len(expected))
				continue
			}
			for i, expectedVal := range expected {
				testIntegerObject(t, list.Elements[i], expectedVal)
			}
		case string:
			str, ok := evaluated.(*object.String)
			if !ok {
				t.Errorf("object is not String. got=%T (%+v)", evaluated, evaluated)
				continue
			}
			if str.Value != expected {
				t.Errorf("wrong string value. got=%q, want=%q", str.Value, expected)
			}
		}
	}
}

func TestDictionaryMethods(t *testing.T) {
	tests := []struct {
		input   string
		checkFn func(object.Object) bool
	}{
		{
			`keys({"a": "1", "b": "2"})`,
			func(obj object.Object) bool {
				list, ok := obj.(*object.List)
				return ok && len(list.Elements) == 2
			},
		},
		{
			`values({"a": "1", "b": "2"})`,
			func(obj object.Object) bool {
				list, ok := obj.(*object.List)
				return ok && len(list.Elements) == 2
			},
		},
		{
			`items({"a": "1", "b": "2"})`,
			func(obj object.Object) bool {
				list, ok := obj.(*object.List)
				if !ok || len(list.Elements) != 2 {
					return false
				}
				// Each item should be a list of 2 elements
				for _, item := range list.Elements {
					itemList, ok := item.(*object.List)
					if !ok || len(itemList.Elements) != 2 {
						return false
					}
				}
				return true
			},
		},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		if !tt.checkFn(evaluated) {
			t.Errorf("test failed for input: %s, got=%T (%+v)", tt.input, evaluated, evaluated)
		}
	}
}

func TestMethodCalls(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`5.type()`, "INTEGER"},
		{`3.14.type()`, "FLOAT"},
		{`"hello".type()`, "STRING"},
		{`True.type()`, "BOOLEAN"},
		{`[1, 2].type()`, "LIST"},
		{`{"a": "b"}.type()`, "DICT"},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		str, ok := evaluated.(*object.String)
		if !ok {
			t.Errorf("object is not String. got=%T (%+v)", evaluated, evaluated)
			continue
		}
		if str.Value != tt.expected {
			t.Errorf("wrong value for %s. got=%q, want=%q", tt.input, str.Value, tt.expected)
		}
	}
}

func TestBreakStatement(t *testing.T) {
	input := `
result = 0
for i in [1, 2, 3, 4, 5]:
    if i == 3:
        break
    result = result + i
result
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 3) // 1 + 2
}

func TestContinueStatement(t *testing.T) {
	input := `
result = 0
for i in [1, 2, 3, 4, 5]:
    if i == 3:
        continue
    result = result + i
result
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 12) // 1 + 2 + 4 + 5
}

func TestPassStatement(t *testing.T) {
	input := `
x = 0
if x == 0:
    pass
x
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 0)
}

func TestAugmentedAssignment(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"x = 10\nx += 5\nx", int64(15)},
		{"x = 10\nx -= 3\nx", int64(7)},
		{"x = 10\nx *= 2\nx", int64(20)},
		{"x = 10\nx /= 2\nx", float64(5.0)},
		{"x = 10\nx %= 3\nx", int64(1)},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		switch expected := tt.expected.(type) {
		case int64:
			testIntegerObject(t, evaluated, expected)
		case float64:
			testFloatObject(t, evaluated, expected)
		}
	}
}

func TestElifStatement(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{`
x = 85
if x >= 90:
    result = 1
elif x >= 80:
    result = 2
elif x >= 70:
    result = 3
else:
    result = 4
result
`, 2},
		{`
x = 95
if x >= 90:
    result = 1
elif x >= 80:
    result = 2
else:
    result = 3
result
`, 1},
		{`
x = 65
if x >= 90:
    result = 1
elif x >= 80:
    result = 2
elif x >= 70:
    result = 3
else:
    result = 4
result
`, 4},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestForLoopWithRange(t *testing.T) {
	input := `
result = 0
for i in range(5):
    result = result + i
result
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 10) // 0+1+2+3+4
}

func TestNestedLoopsWithBreakContinue(t *testing.T) {
	input := `
result = 0
for i in range(3):
    for j in range(3):
        if j == 1:
            continue
        if i == 2 and j == 2:
            break
        result = result + 1
result
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 5) // (0,0), (0,2), (1,0), (1,2), (2,0)
}

func TestForLoopUnpacking(t *testing.T) {
	input := `
result = []
for x, y in [(1, 2), (3, 4), (5, 6)]:
    result.append(x + y)
result
`
	evaluated := testEval(input)
	list := evaluated.(*object.List)
	if len(list.Elements) != 3 {
		t.Errorf("list has wrong number of elements. got=%d", len(list.Elements))
	}
	testIntegerObject(t, list.Elements[0], 3)  // 1+2
	testIntegerObject(t, list.Elements[1], 7)  // 3+4
	testIntegerObject(t, list.Elements[2], 11) // 5+6
}
