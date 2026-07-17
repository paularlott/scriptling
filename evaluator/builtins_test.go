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

// TestBuiltinIterablesOnDictViewsSetsStrings covers sorted/sum/min/max accepting
// any iterable (dict views, sets, strings, dicts), not just lists/tuples.
func TestBuiltinIterablesOnDictViewsSetsStrings(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		expected string
	}{
		// sorted
		{name: "sorted dict_keys", script: `d = {"b": 2, "a": 1, "c": 3}; sorted(d.keys())`, expected: `[a, b, c]`},
		{name: "sorted dict_values", script: `d = {"b": 2, "a": 1, "c": 3}; sorted(d.values())`, expected: `[1, 2, 3]`},
		{name: "sorted dict_items by value", script: `d = {"a": 3, "c": 1, "b": 2}; sorted(d.items(), key=lambda x: x[1])`, expected: `[(c, 1), (b, 2), (a, 3)]`},
		{name: "sorted set", script: `sorted(set([3, 1, 2]))`, expected: `[1, 2, 3]`},
		{name: "sorted string", script: `sorted("cab")`, expected: `[a, b, c]`},
		{name: "sorted dict yields keys", script: `d = {"b": 2, "a": 1}; sorted(d)`, expected: `[a, b]`},
		{name: "sorted does not mutate input list", script: `o = [3, 1, 2]; sorted(o); o`, expected: `[3, 1, 2]`},
		// sum
		{name: "sum dict_values", script: `d = {"a": 1, "b": 2, "c": 3}; sum(d.values())`, expected: `6`},
		{name: "sum set", script: `sum(set([1, 2, 3]))`, expected: `6`},
		{name: "sum tuple", script: `sum((10, 20, 30))`, expected: `60`},
		// min / max
		{name: "min dict_keys", script: `d = {"b": 2, "a": 1, "c": 3}; min(d.keys())`, expected: `a`},
		{name: "max dict_keys", script: `d = {"b": 2, "a": 1, "c": 3}; max(d.keys())`, expected: `c`},
		{name: "min dict_values", script: `d = {"a": 3, "c": 1, "b": 2}; min(d.values())`, expected: `1`},
		{name: "max set", script: `max(set([5, 2, 8]))`, expected: `8`},
		{name: "min string", script: `min("cab")`, expected: `a`},
		{name: "max string", script: `max("cab")`, expected: `c`},
		// multi-arg form still works (regression guard)
		{name: "min multi-arg", script: `min(5, 2, 8)`, expected: `2`},
		{name: "max multi-arg", script: `max(5, 2, 8)`, expected: `8`},
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

// TestSetOperators covers the & | - ^ set-algebra operators and value equality.
func TestSetOperators(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{name: "intersection", script: `set([1,2,3]) & set([2,3,4])`, expected: `{2, 3}`},
		{name: "union", script: `set([1,2,3]) | set([2,3,4])`, expected: `{1, 2, 3, 4}`},
		{name: "difference", script: `set([1,2,3]) - set([2,3,4])`, expected: `{1}`},
		{name: "symmetric difference", script: `set([1,2,3]) ^ set([2,3,4])`, expected: `{1, 4}`},
		{name: "operator matches method", script: `(set([1,2,3]) & set([2,3,4])) == set([1,2,3]).intersection(set([2,3,4]))`, expected: `true`},
		{name: "operands not mutated", script: `a=set([1,2,3]); b=set([2,3,4]); a & b; a`, expected: `{1, 2, 3}`},
		{name: "value equality order-independent", script: `set([1,2,3]) == set([3,2,1])`, expected: `true`},
		{name: "value inequality", script: `set([1,2,3]) != set([1,2])`, expected: `true`},
		{name: "empty set equality", script: `set([]) == set([])`, expected: `true`},
		{name: "cross-type equality false", script: `set([1,2]) == [1,2]`, expected: `false`},
		{name: "chained with equality", script: `(set([1,2,3]) & set([2,3,4])) == set([2,3])`, expected: `true`},
		// empty-set edge cases
		{name: "empty intersection", script: `set([]) & set([1])`, expected: `{}`},
		{name: "empty union", script: `set([]) | set([1])`, expected: `{1}`},
		{name: "difference from empty", script: `set([1]) - set([])`, expected: `{1}`},
		{name: "empty symmetric difference", script: `set([]) ^ set([1])`, expected: `{1}`},
		{name: "two empties intersection", script: `set([]) & set([])`, expected: `{}`},
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

	// A non-set right operand must produce a type error (matches Python).
	errResult := testEval(`set([1,2]) & [1,2]`)
	if !object.IsError(errResult) {
		t.Errorf("expected type error for set & list, got %s", errResult.Inspect())
	}

	// Augmented assignment (&= |= -= ^=) delegates through the infix operators,
	// so it works on sets too — rebinds the name to a new set.
	for _, tt := range []struct{ name, script, expected string }{
		{"&=", `s=set([1,2,3]); s &= set([2,3,4]); s`, `{2, 3}`},
		{"|=", `s=set([1,2,3]); s |= set([5]); s`, `{1, 2, 3, 5}`},
		{"-=", `s=set([1,2,3]); s -= set([1]); s`, `{2, 3}`},
		{"^=", `s=set([1,2,3]); s ^= set([1]); s`, `{2, 3}`},
	} {
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

	// Integer bitwise operators are unaffected by the new Set case.
	for _, tt := range []struct{ name, script, expected string }{
		{"int and", `0xFF & 0x0F`, `15`},
		{"int or", `0xF0 | 0x0F`, `255`},
		{"int xor", `0xFF ^ 0x0F`, `240`},
		{"int sub", `5 - 2`, `3`},
	} {
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

// TestTruthyCollections covers Python-style truthiness for collection types:
// empty Tuple/Set/dict-views are falsy (previously they were all truthy).
func TestTruthyCollections(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{name: "empty set falsy", script: `bool(set())`, expected: `false`},
		{name: "nonempty set truthy", script: `bool(set([1]))`, expected: `true`},
		{name: "empty tuple falsy", script: `bool(())`, expected: `false`},
		{name: "nonempty tuple truthy", script: `bool((1,))`, expected: `true`},
		{name: "empty dict_keys falsy", script: `bool({}.keys())`, expected: `false`},
		{name: "nonempty dict_values truthy", script: `bool({1: 1}.values())`, expected: `true`},
		{name: "empty dict_items falsy", script: `bool({}.items())`, expected: `false`},
		// short-circuit: empty set is falsy so `and` returns it without evaluating RHS
		{name: "empty set short-circuits and", script: `set() and "RHS"`, expected: `{}`},
		{name: "nonempty set and evaluates RHS", script: `set([1]) and "RHS"`, expected: `RHS`},
		{name: "empty tuple short-circuits and", script: `() and "RHS"`, expected: `()`},
		// if-condition uses the same isTruthy path
		{name: "if rejects empty set", script: `r = 0
if set():
    r = 1
r`, expected: `0`},
		{name: "if accepts nonempty set", script: `r = 0
if set([1]):
    r = 1
r`, expected: `1`},
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
