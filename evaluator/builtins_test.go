package evaluator

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestBuiltinLen(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected int64
	}{
		{&object.String{Value: "hello"}, 5},
		{&object.String{Value: ""}, 0},
		{&object.List{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}}, 2},
		{&object.List{Elements: []object.Object{}}, 0},
	}

	for _, tt := range tests {
		result := builtins["len"].Fn(context.Background(), object.NewKwargs(nil), tt.input)
		integer, ok := result.(*object.Integer)
		if !ok {
			t.Errorf("object is not Integer. got=%T (%+v)", result, result)
			continue
		}
		if integer.Value != tt.expected {
			t.Errorf("wrong value. got=%d, want=%d", integer.Value, tt.expected)
		}
	}
}

func TestBuiltinLenError(t *testing.T) {
	result := builtins["len"].Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 1})
	if !object.IsError(result) {
		t.Errorf("expected error for len(1), got %T", result)
	}
}

func TestBuiltinStr(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected string
	}{
		{&object.Integer{Value: 42}, "42"},
		{&object.Float{Value: 3.14}, "3.14"},
		{&object.String{Value: "hello"}, "hello"},
		{&object.Boolean{Value: true}, "true"},
	}

	for _, tt := range tests {
		result := builtins["str"].Fn(context.Background(), object.NewKwargs(nil), tt.input)
		str, ok := result.(*object.String)
		if !ok {
			t.Errorf("object is not String. got=%T (%+v)", result, result)
			continue
		}
		if str.Value != tt.expected {
			t.Errorf("wrong value. got=%q, want=%q", str.Value, tt.expected)
		}
	}
}

func TestBuiltinInt(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected int64
	}{
		{&object.Integer{Value: 42}, 42},
		{&object.Float{Value: 3.14}, 3},
		{&object.String{Value: "123"}, 123},
	}

	for _, tt := range tests {
		result := builtins["int"].Fn(context.Background(), object.NewKwargs(nil), tt.input)
		integer, ok := result.(*object.Integer)
		if !ok {
			t.Errorf("object is not Integer. got=%T (%+v)", result, result)
			continue
		}
		if integer.Value != tt.expected {
			t.Errorf("wrong value. got=%d, want=%d", integer.Value, tt.expected)
		}
	}
}

func TestBuiltinFloat(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected float64
	}{
		{&object.Float{Value: 3.14}, 3.14},
		{&object.Integer{Value: 42}, 42.0},
		{&object.String{Value: "3.14"}, 3.14},
	}

	for _, tt := range tests {
		result := builtins["float"].Fn(context.Background(), object.NewKwargs(nil), tt.input)
		float, ok := result.(*object.Float)
		if !ok {
			t.Errorf("object is not Float. got=%T (%+v)", result, result)
			continue
		}
		if float.Value != tt.expected {
			t.Errorf("wrong value. got=%f, want=%f", float.Value, tt.expected)
		}
	}
}

func TestBuiltinType(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected string
	}{
		{&object.Integer{Value: 42}, "INTEGER"},
		{&object.Float{Value: 3.14}, "FLOAT"},
		{&object.String{Value: "hello"}, "STRING"},
		{&object.Boolean{Value: true}, "BOOLEAN"},
		{&object.List{Elements: []object.Object{}}, "LIST"},
		{&object.Dict{Pairs: make(map[string]object.DictPair)}, "DICT"},
	}

	for _, tt := range tests {
		result := builtins["type"].Fn(context.Background(), object.NewKwargs(nil), tt.input)
		str, ok := result.(*object.String)
		if !ok {
			t.Errorf("object is not String. got=%T (%+v)", result, result)
			continue
		}
		if str.Value != tt.expected {
			t.Errorf("wrong value. got=%q, want=%q", str.Value, tt.expected)
		}
	}
}

func TestBuiltinSortedWithLambda(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{
			name:     "sort numbers with lambda",
			script:   `sorted([3, 1, 4, 1, 5], key=lambda x: x)`,
			expected: "[1, 1, 3, 4, 5]",
		},
		{
			name:     "sort numbers reverse with lambda",
			script:   `sorted([3, 1, 4, 1, 5], key=lambda x: x, reverse=True)`,
			expected: "[5, 4, 3, 1, 1]",
		},
		{
			name:     "sort strings by length",
			script:   `sorted(["ccc", "a", "bb"], key=lambda s: len(s))`,
			expected: `[a, bb, ccc]`,
		},
		{
			name:     "sort with negative key",
			script:   `sorted([1, 2, 3], key=lambda x: -x)`,
			expected: "[3, 2, 1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := testEval(tt.script)
			if object.IsError(result) {
				t.Fatalf("eval error: %s", result.Inspect())
			}

			if result.Inspect() != tt.expected {
				t.Errorf("wrong result. got=%s, want=%s", result.Inspect(), tt.expected)
			}
		})
	}
}
