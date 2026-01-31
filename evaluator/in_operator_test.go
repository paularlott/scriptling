package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

func testEvalWithEnv(input string) (object.Object, *object.Environment) {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	result := Eval(program, env)
	return result, env
}

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
		_, env := testEvalWithEnv(tt.input)
		result, ok := env.Get("result")
		if !ok {
			t.Errorf("variable result not found in environment")
			continue
		}
		testBooleanObject(t, result, tt.expected)
	}
}

func TestInOperatorWithVariables(t *testing.T) {
	input := `
items = [1, 2, 3, 4, 5]
x = 3
result = x in items
`
	_, env := testEvalWithEnv(input)
	result, ok := env.Get("result")
	if !ok {
		t.Errorf("variable result not found in environment")
		return
	}
	testBooleanObject(t, result, true)
}

func TestNotInOperatorWithVariables(t *testing.T) {
	input := `
items = [1, 2, 3, 4, 5]
x = 10
result = x not in items
`
	_, env := testEvalWithEnv(input)
	result, ok := env.Get("result")
	if !ok {
		t.Errorf("variable result not found in environment")
		return
	}
	testBooleanObject(t, result, true)
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

func TestChainedInExpressionsWithShortCircuit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name: "both conditions true",
			input: `
operation = {"requestBody": {"content": {}}}
result = "requestBody" in operation and "content" in operation["requestBody"]
`,
			expected: true,
		},
		{
			name: "first condition false - should short-circuit",
			input: `
operation = {"other": "value"}
result = "requestBody" in operation and "content" in operation["requestBody"]
`,
			expected: false,
		},
		{
			name: "triple nested check",
			input: `
data = {"level1": {"level2": {"level3": "value"}}}
result = "level1" in data and "level2" in data["level1"] and "level3" in data["level1"]["level2"]
`,
			expected: true,
		},
		{
			name: "triple nested check - first fails",
			input: `
data = {"other": "value"}
result = "level1" in data and "level2" in data["level1"] and "level3" in data["level1"]["level2"]
`,
			expected: false,
		},
		{
			name: "triple nested check - second fails",
			input: `
data = {"level1": {"other": "value"}}
result = "level1" in data and "level2" in data["level1"] and "level3" in data["level1"]["level2"]
`,
			expected: false,
		},
		{
			name: "or operator with short-circuit",
			input: `
data = {}
result = "key" in data or len(data) == 0
`,
			expected: true,
		},
		{
			name: "or operator - first true should not evaluate second",
			input: `
data = {"key": "value"}
result = "key" in data or data["nonexistent"]["nested"]
`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, env := testEvalWithEnv(tt.input)
			result, ok := env.Get("result")
			if !ok {
				t.Errorf("variable result not found in environment")
				return
			}
			testBooleanObject(t, result, tt.expected)
		})
	}
}
