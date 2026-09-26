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
		{object.NewBoolean(true), "True"},
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
		// key function: the item whose key wins is returned, not the key
		{name: "max key returns original item", script: `max([{"v": 1, "n": "a"}, {"v": 3, "n": "b"}, {"v": 2, "n": "c"}], key=lambda r: r["v"])["n"]`, expected: `b`},
		{name: "min key returns original item", script: `min([{"v": 1, "n": "a"}, {"v": 3, "n": "b"}, {"v": 2, "n": "c"}], key=lambda r: r["v"])["n"]`, expected: `a`},
		{name: "min key builtin len", script: `min(["pear", "fig", "banana"], key=len)`, expected: `fig`},
		{name: "max key builtin len", script: `max(["pear", "fig", "banana"], key=len)`, expected: `banana`},
		{name: "max key ties keep first", script: `max([{"v": 5, "t": 1}, {"v": 5, "t": 2}], key=lambda r: r["v"])["t"]`, expected: `1`},
		{name: "min key ties keep first", script: `min([{"v": 5, "t": 1}, {"v": 5, "t": 2}], key=lambda r: r["v"])["t"]`, expected: `1`},
		{name: "max key with multiple arguments", script: `max({"v": 1}, {"v": 9}, key=lambda r: r["v"])["v"]`, expected: `9`},
		{name: "max key None means no key", script: `max([1, 2], key=None)`, expected: `2`},
		// default for empty iterables
		{name: "max empty with default", script: `max([], default="none")`, expected: `none`},
		{name: "min empty with default", script: `min([], default=0)`, expected: `0`},
		{name: "max empty without default errors", script: `def f():
    return max([])
msg = "no error"
try:
    f()
except Exception as e:
    msg = str(e)
msg`, expected: `max() arg is an empty sequence`},
		{name: "max default rejected with multiple args", script: `def f():
    return max(1, 2, default=0)
msg = "no error"
try:
    f()
except Exception as e:
    msg = str(e)
msg`, expected: `Cannot specify a default for max() with multiple arguments`},
		{name: "min default rejected with multiple args", script: `def f():
    return min(1, 2, default=0)
msg = "no error"
try:
    f()
except Exception as e:
    msg = str(e)
msg`, expected: `Cannot specify a default for min() with multiple arguments`},
		// booleans order as 0 and 1, against each other and against numbers
		{name: "min bools", script: `min([True, False])`, expected: `False`},
		{name: "max bools", script: `max([False, True])`, expected: `True`},
		{name: "min bool multi-arg", script: `min(True, False, True)`, expected: `False`},
		{name: "max with bool among numbers", script: `max(0, True, 2)`, expected: `2`},
		{name: "min tie between 1 and True keeps first", script: `str(min(1, True))`, expected: `1`},
		{name: "max tie between True and 1 keeps first", script: `str(max(True, 1))`, expected: `True`},
		{name: "sorted bools", script: `sorted([True, False, True])`, expected: `[False, True, True]`},
		{name: "sorted mixed bool and int stable", script: `sorted([2, True, 0, False, 1])`, expected: `[0, False, True, 1, 2]`},
		{name: "sorted mixed bool and float", script: `str(sorted([1.5, True, 0.5]))`, expected: `[0.5, True, 1.5]`},
		{name: "max key returning bool", script: `max([{"ok": False, "n": "a"}, {"ok": True, "n": "b"}], key=lambda r: r["ok"])["n"]`, expected: `b`},
		{name: "tuple ordering with bools", script: `str(min([(True, "x"), (False, "y")]))`, expected: `(False, y)`},
		{name: "sorted bool vs string still errors", script: `def f():
    return sorted([True, "x"])
msg = "no error"
try:
    f()
except Exception as e:
    msg = str(e)
msg`, expected: `cannot compare STRING with BOOLEAN`},
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
		{name: "operator matches method", script: `(set([1,2,3]) & set([2,3,4])) == set([1,2,3]).intersection(set([2,3,4]))`, expected: `True`},
		{name: "operands not mutated", script: `a=set([1,2,3]); b=set([2,3,4]); a & b; a`, expected: `{1, 2, 3}`},
		{name: "value equality order-independent", script: `set([1,2,3]) == set([3,2,1])`, expected: `True`},
		{name: "value inequality", script: `set([1,2,3]) != set([1,2])`, expected: `True`},
		{name: "empty set equality", script: `set([]) == set([])`, expected: `True`},
		{name: "cross-type equality false", script: `set([1,2]) == [1,2]`, expected: `False`},
		{name: "chained with equality", script: `(set([1,2,3]) & set([2,3,4])) == set([2,3])`, expected: `True`},
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
		{name: "empty set falsy", script: `bool(set())`, expected: `False`},
		{name: "nonempty set truthy", script: `bool(set([1]))`, expected: `True`},
		{name: "empty tuple falsy", script: `bool(())`, expected: `False`},
		{name: "nonempty tuple truthy", script: `bool((1,))`, expected: `True`},
		{name: "empty dict_keys falsy", script: `bool({}.keys())`, expected: `False`},
		{name: "nonempty dict_values truthy", script: `bool({1: 1}.values())`, expected: `True`},
		{name: "empty dict_items falsy", script: `bool({}.items())`, expected: `False`},
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

// TestTypeOfExceptionReportsRaisedClass: type(e) returns the class the
// exception was raised as, not the generic "EXCEPTION", so except blocks can
// discriminate without message sniffing. (Custom exception classes are not
// possible: classes cannot derive from the built-in exception types.)
func TestTypeOfExceptionReportsRaisedClass(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{"ValueError", `try:
    raise ValueError("v")
except Exception as e:
    t = type(e)
t`, "ValueError"},
		{"KeyError", `try:
    raise KeyError("k")
except Exception as e:
    t = type(e)
t`, "KeyError"},
		{"constructed value reports class too", `type(ValueError("not raised"))`, "ValueError"},
		{"other types unchanged", `type(1)`, "INTEGER"},
		{"string type unchanged", `type("x")`, "STRING"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := testEval(tt.script)
			if result.Inspect() != tt.expected {
				t.Errorf("got=%s, want=%s", result.Inspect(), tt.expected)
			}
		})
	}
}

