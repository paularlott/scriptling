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
		{object.NewString("hello"), 5},
		{object.NewString(""), 0},
		{&object.List{Elements: []object.Object{object.NewInteger(1), object.NewInteger(2)}}, 2},
		{&object.List{Elements: []object.Object{}}, 0},
	}

	for _, tt := range tests {
		result := builtins["len"].Fn(context.Background(), object.NewKwargs(nil), tt.input)
		integer, ok := result.(*object.Integer)
		if !ok {
			t.Errorf("object is not Integer. got=%T (%+v)", result, result)
			continue
		}
		if integer.IntValue() != tt.expected {
			t.Errorf("wrong value. got=%d, want=%d", integer.IntValue(), tt.expected)
		}
	}
}

func TestBuiltinLenError(t *testing.T) {
	result := builtins["len"].Fn(context.Background(), object.NewKwargs(nil), object.NewInteger(1))
	if !object.IsError(result) {
		t.Errorf("expected error for len(1), got %T", result)
	}
}

func TestBuiltinCopyInstanceDropsNativeData(t *testing.T) {
	class := &object.Class{Name: "NativeBacked", Methods: map[string]object.Object{}}
	sharedField := &object.List{Elements: []object.Object{object.NewString("value")}}
	instance := object.NewInstanceWithData(class, map[string]object.Object{"items": sharedField}, object.NewString("native"))

	result := builtins["copy"].Fn(context.Background(), object.NewKwargs(nil), instance)
	copied, ok := result.(*object.Instance)
	if !ok {
		t.Fatalf("expected copied instance, got %T", result)
	}
	if copied == instance {
		t.Fatal("expected a new instance")
	}
	if copied.NativeData != nil {
		t.Fatal("expected copied instance to drop NativeData")
	}
	if copied.Field("items") != sharedField {
		t.Fatal("expected shallow copy to share field values")
	}
}

func TestBuiltinStr(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected string
	}{
		{object.NewInteger(42), "42"},
		{object.NewFloat(3.14), "3.14"},
		{object.NewString("hello"), "hello"},
		{object.NewBoolean(true), "true"},
	}

	for _, tt := range tests {
		result := builtins["str"].Fn(context.Background(), object.NewKwargs(nil), tt.input)
		str, ok := result.(*object.String)
		if !ok {
			t.Errorf("object is not String. got=%T (%+v)", result, result)
			continue
		}
		if str.StringValue() != tt.expected {
			t.Errorf("wrong value. got=%q, want=%q", str.StringValue(), tt.expected)
		}
	}
}

func TestBuiltinInt(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected int64
	}{
		{object.NewInteger(42), 42},
		{object.NewFloat(3.14), 3},
		{object.NewString("123"), 123},
	}

	for _, tt := range tests {
		result := builtins["int"].Fn(context.Background(), object.NewKwargs(nil), tt.input)
		integer, ok := result.(*object.Integer)
		if !ok {
			t.Errorf("object is not Integer. got=%T (%+v)", result, result)
			continue
		}
		if integer.IntValue() != tt.expected {
			t.Errorf("wrong value. got=%d, want=%d", integer.IntValue(), tt.expected)
		}
	}
}

func TestBuiltinFloat(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected float64
	}{
		{object.NewFloat(3.14), 3.14},
		{object.NewInteger(42), 42.0},
		{object.NewString("3.14"), 3.14},
	}

	for _, tt := range tests {
		result := builtins["float"].Fn(context.Background(), object.NewKwargs(nil), tt.input)
		float, ok := result.(*object.Float)
		if !ok {
			t.Errorf("object is not Float. got=%T (%+v)", result, result)
			continue
		}
		if float.FloatValue() != tt.expected {
			t.Errorf("wrong value. got=%f, want=%f", float.FloatValue(), tt.expected)
		}
	}
}

func TestBuiltinType(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected string
	}{
		{object.NewInteger(42), "INTEGER"},
		{object.NewFloat(3.14), "FLOAT"},
		{object.NewString("hello"), "STRING"},
		{object.NewBoolean(true), "BOOLEAN"},
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
		if str.StringValue() != tt.expected {
			t.Errorf("wrong value. got=%q, want=%q", str.StringValue(), tt.expected)
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

func TestBuiltinSortedTuplesAndLists(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{
			name:     "sorted tuples by first element",
			script:   `sorted([(3, "c"), (1, "a"), (2, "b")])`,
			expected: `[(1, a), (2, b), (3, c)]`,
		},
		{
			name:     "sorted tuples reverse",
			script:   `sorted([(3, "c"), (1, "a"), (2, "b")], reverse=True)`,
			expected: `[(3, c), (2, b), (1, a)]`,
		},
		{
			name:     "sorted tuples tiebreak on second element",
			script:   `sorted([(1, 9), (1, 3), (1, 7)])`,
			expected: `[(1, 3), (1, 7), (1, 9)]`,
		},
		{
			name:     "sorted lists of lists",
			script:   `sorted([[3], [1], [2]])`,
			expected: `[[1], [2], [3]]`,
		},
		{
			name:     "list.sort mutates tuples in place",
			script:   `x = [(3, "c"), (1, "a"), (2, "b")]; x.sort(); x`,
			expected: `[(1, a), (2, b), (3, c)]`,
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
