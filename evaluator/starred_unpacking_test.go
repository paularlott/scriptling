package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestStarredUnpacking(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Basic starred unpacking
		{
			`a, *b, c = [1, 2, 3, 4, 5]
result = [a, b, c]
result`,
			"[1, [2, 3, 4], 5]",
		},
		// Starred at beginning
		{
			`*first, last = [1, 2, 3]
result = [first, last]
result`,
			"[[1, 2], 3]",
		},
		// Starred at end
		{
			`first, *rest = [1, 2, 3, 4]
result = [first, rest]
result`,
			"[1, [2, 3, 4]]",
		},
		// Starred with minimal values
		{
			`x, *y = [10, 20]
result = [x, y]
result`,
			"[10, [20]]",
		},
		// Starred with exact minimum
		{
			`a, *b, c = [1, 2]
result = [a, b, c]
result`,
			"[1, [], 2]",
		},
		// Starred in middle with multiple before/after
		{
			`a, b, *middle, y, z = [1, 2, 3, 4, 5, 6, 7]
result = [a, b, middle, y, z]
result`,
			"[1, 2, [3, 4, 5], 6, 7]",
		},
		// Starred with tuple
		{
			`a, *b, c = (10, 20, 30, 40)
result = [a, b, c]
result`,
			"[10, [20, 30], 40]",
		},
		// Starred collecting nothing
		{
			`a, *b, c, d = [1, 2, 3]
result = [a, b, c, d]
result`,
			"[1, [], 2, 3]",
		},
		// Multiple variables before star
		{
			`a, b, c, *rest = [1, 2, 3, 4, 5, 6]
result = [a, b, c, rest]
result`,
			"[1, 2, 3, [4, 5, 6]]",
		},
		// Multiple variables after star
		{
			`*first, x, y, z = [1, 2, 3, 4, 5]
result = [first, x, y, z]
result`,
			"[[1, 2], 3, 4, 5]",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		if evaluated == nil {
			t.Errorf("testEval returned nil for input: %s", tt.input)
			continue
		}
		if evaluated.Inspect() != tt.expected {
			t.Errorf("for input %q: expected %s, got %s", tt.input, tt.expected, evaluated.Inspect())
		}
	}
}

func TestStarredUnpackingErrors(t *testing.T) {
	tests := []struct {
		input       string
		expectedErr string
	}{
		// Not enough values
		{
			`a, *b, c = [1]`,
			"not enough values to unpack",
		},
		// Not enough values with multiple after star
		{
			`*a, b, c = [1]`,
			"not enough values to unpack",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		if evaluated == nil {
			t.Errorf("testEval returned nil for input: %s", tt.input)
			continue
		}
		errObj, ok := evaluated.(*object.Error)
		if !ok {
			t.Errorf("expected error for input %q, got %T: %s", tt.input, evaluated, evaluated.Inspect())
			continue
		}
		if !contains(errObj.Message, tt.expectedErr) {
			t.Errorf("for input %q: expected error containing %q, got %q", tt.input, tt.expectedErr, errObj.Message)
		}
	}
}

func TestStarredUnpackingWithFunctions(t *testing.T) {
	input := `
def get_data():
    return [1, 2, 3, 4, 5]

first, *middle, last = get_data()
result = [first, middle, last]
result
`
	evaluated := testEval(input)
	expected := "[1, [2, 3, 4], 5]"
	if evaluated.Inspect() != expected {
		t.Errorf("expected %s, got %s", expected, evaluated.Inspect())
	}
}

func TestStarredUnpackingInLoop(t *testing.T) {
	input := `
data = [[1, 2, 3], [4, 5, 6, 7], [8, 9]]
results = []
for item in data:
    first, *rest = item
    results.append([first, rest])
results
`
	evaluated := testEval(input)
	expected := "[[1, [2, 3]], [4, [5, 6, 7]], [8, [9]]]"
	if evaluated.Inspect() != expected {
		t.Errorf("expected %s, got %s", expected, evaluated.Inspect())
	}
}

func TestBasicUnpackingStillWorks(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`x, y = [1, 2]
result = [x, y]
result`,
			"[1, 2]",
		},
		{
			`a, b, c = (10, 20, 30)
result = [a, b, c]
result`,
			"[10, 20, 30]",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		if evaluated == nil {
			t.Errorf("testEval returned nil for input: %s", tt.input)
			continue
		}
		if evaluated.Inspect() != tt.expected {
			t.Errorf("for input %q: expected %s, got %s", tt.input, tt.expected, evaluated.Inspect())
		}
	}
}


