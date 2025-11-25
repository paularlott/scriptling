package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestSliceWithStep(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// List slicing with step
		{"[0, 1, 2, 3, 4, 5, 6, 7, 8, 9][::2]", []int64{0, 2, 4, 6, 8}},
		{"[0, 1, 2, 3, 4, 5, 6, 7, 8, 9][1::2]", []int64{1, 3, 5, 7, 9}},
		{"[0, 1, 2, 3, 4, 5, 6, 7, 8, 9][1:8:2]", []int64{1, 3, 5, 7}},

		// Reverse slicing with negative step
		{"[0, 1, 2, 3, 4, 5][::-1]", []int64{5, 4, 3, 2, 1, 0}},
		{"[0, 1, 2, 3, 4, 5][::-2]", []int64{5, 3, 1}},
		{"[0, 1, 2, 3, 4, 5][4:1:-1]", []int64{4, 3, 2}},

		// String slicing with step
		{`"hello"[::2]`, "hlo"},
		{`"hello"[::-1]`, "olleh"},
		{`"abcdefgh"[1:7:2]`, "bdf"},
		{`"abcdefgh"[::-2]`, "hfdb"},
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
