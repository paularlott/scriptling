package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestBooleanShortCircuitAssignment(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// or returns first truthy value
		{`x = 5 or 10
x`, 5},
		{`x = 0 or 10
x`, 10},
		{`x = "" or "default"
x`, "default"},
		{`x = "value" or "default"
x`, "value"},
		{`x = None or 42
x`, 42},
		{`x = False or True
x`, true},
		{`x = False or False
x`, false},

		// and returns first falsy value or last value
		{`x = 5 and 10
x`, 10},
		{`x = 0 and 10
x`, 0},
		{`x = "" and "value"
x`, ""},
		{`x = "a" and "b"
x`, "b"},
		{`x = True and False
x`, false},
		{`x = True and True
x`, true},
		{`x = None and 42
x`, nil},

		// Chained short-circuit
		{`x = 0 or 5 or 10
x`, 5},
		{`x = 0 or 0 or 10
x`, 10},
		{`x = 1 and 2 and 3
x`, 3},
		{`x = 1 and 0 and 3
x`, 0},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		switch expected := tt.expected.(type) {
		case int:
			testIntegerObject(t, evaluated, int64(expected))
		case string:
			str, ok := evaluated.(*object.String)
			if !ok {
				t.Errorf("object is not String. got=%T (%+v)", evaluated, evaluated)
				continue
			}
			if str.Value != expected {
				t.Errorf("wrong string value. got=%q, want=%q", str.Value, expected)
			}
		case bool:
			testBooleanObject(t, evaluated, expected)
		case nil:
			if evaluated != NULL {
				t.Errorf("object is not NULL. got=%T (%+v)", evaluated, evaluated)
			}
		}
	}
}

func TestChainedComparisons(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Basic chained comparisons
		{"1 < 2 < 3", true},
		{"1 < 2 < 1", false},
		{"3 > 2 > 1", true},
		{"3 > 2 > 3", false},

		// Mixed operators
		{"1 < 2 <= 2", true},
		{"1 < 2 <= 1", false},
		{"5 > 3 >= 3", true},
		{"5 > 3 >= 4", false},

		// Equality chains
		{"1 == 1 == 1", true},
		{"1 == 1 == 2", false},
		{"1 != 2 != 3", true},
		{"1 != 2 != 2", false},

		// Longer chains
		{"1 < 2 < 3 < 4", true},
		{"1 < 2 < 3 < 2", false},
		{"5 > 4 > 3 > 2", true},
		{"5 > 4 > 3 > 4", false},

		// With variables
		{`x = 5
y = 10
z = 15
x < y < z`, true},
		{`x = 5
y = 10
z = 5
x < y < z`, false},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}
