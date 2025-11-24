package evaluator

import (
	"testing"
	"github.com/paularlott/scriptling/object"
)

func TestInOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// List membership
		{`result = 5 in [1, 2, 3, 4, 5]`, true},
		{`result = 6 in [1, 2, 3, 4, 5]`, false},
		{`result = "hello" in ["hello", "world"]`, true},
		{`result = "foo" in ["hello", "world"]`, false},
		
		// Dict membership (keys)
		{`result = "name" in {"name": "Alice", "age": 30}`, true},
		{`result = "email" in {"name": "Alice", "age": 30}`, false},
		
		// String substring
		{`result = "hello" in "hello world"`, true},
		{`result = "foo" in "hello world"`, false},
		{`result = "world" in "hello world"`, true},
		
		// not in operator
		{`result = 6 not in [1, 2, 3, 4, 5]`, true},
		{`result = 5 not in [1, 2, 3, 4, 5]`, false},
		{`result = "foo" not in ["hello", "world"]`, true},
		{`result = "hello" not in ["hello", "world"]`, false},
		{`result = "email" not in {"name": "Alice"}`, true},
		{`result = "name" not in {"name": "Alice"}`, false},
		{`result = "foo" not in "hello world"`, true},
		{`result = "hello" not in "hello world"`, false},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestInOperatorWithVariables(t *testing.T) {
	input := `
items = [1, 2, 3, 4, 5]
x = 3
result = x in items
`
	evaluated := testEval(input)
	testBooleanObject(t, evaluated, true)
}

func TestNotInOperatorWithVariables(t *testing.T) {
	input := `
items = [1, 2, 3, 4, 5]
x = 10
result = x not in items
`
	evaluated := testEval(input)
	testBooleanObject(t, evaluated, true)
}

func TestInOperatorInIfStatement(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{
			`
if 5 in [1, 2, 3, 4, 5]:
    x = 10
else:
    x = 20
x
`,
			10,
		},
		{
			`
if 6 in [1, 2, 3, 4, 5]:
    x = 10
else:
    x = 20
x
`,
			20,
		},
		{
			`
data = {"name": "Alice", "age": 30}
if "name" in data:
    result = "found"
else:
    result = "not found"
result
`,
			"found",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		switch expected := tt.expected.(type) {
		case int:
			testIntegerObject(t, evaluated, int64(expected))
		case string:
			str, ok := evaluated.(*object.String)
			if !ok {
				t.Errorf("object is not String. got=%T", evaluated)
				return
			}
			if str.Value != expected {
				t.Errorf("String has wrong value. got=%q, want=%q", str.Value, expected)
			}
		}
	}
}
