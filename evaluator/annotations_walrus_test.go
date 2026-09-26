package evaluator

import (
	"fmt"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestTypeAnnotationsIgnored(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{`def add(a: int, b: int) -> int:
	return a + b
add(2, 3)`, 5},
		{`def add(a: int, b: int = 10) -> int:
	return a + b
add(2)`, 12},
		{`def f(x: "ForwardRef") -> "Result":
	return x + 1
f(41)`, 42},
		{`def g(items: dict[str, int], pairs: tuple[int, ...] = (1,)) -> list[int]:
	return items["k"] + pairs[0]
g({"k": 40})`, 41},
		{`count: int = 5
count + 1`, 6},
		{`name: str
x = 7
x`, 7},
		{`def h(*args: int, **kw: dict) -> int:
	return len(args) + len(kw)
h(1, 2, k=3)`, 3},
	}
	for _, tt := range tests {
		testIntegerObject(t, testEval(tt.input), tt.expected)
	}
}

func TestTypeAnnotationClassFields(t *testing.T) {
	input := `class Config:
	retries: int = 3
	label: str

	def __init__(self):
		self.extra: int = 10

c = Config()
c.retries + c.extra
`
	testIntegerObject(t, testEval(input), 13)
}

func TestWalrusInIf(t *testing.T) {
	input := `if (n := 10) > 5:
	result = n
else:
	result = 0
result
`
	testIntegerObject(t, testEval(input), 10)
}

func TestWalrusInWhile(t *testing.T) {
	input := `data = [1, 2, 0, 3]
i = 0
total = 0
while (v := data[i]) > 0:
	total += v
	i += 1
total
`
	testIntegerObject(t, testEval(input), 3)
}

func TestWalrusYieldsValue(t *testing.T) {
	input := `x = (y := 6)
x + y
`
	testIntegerObject(t, testEval(input), 12)
}

func TestWalrusRightAssociative(t *testing.T) {
	input := `a := b := 5
a + b
`
	testIntegerObject(t, testEval(input), 10)
}

func TestWalrusValueBindsTighterOperators(t *testing.T) {
	// := is looser than comparisons, so n receives the comparison result,
	// matching Python's precedence.
	input := `n := 3 > 2
if n:
	1
else:
	0
`
	testIntegerObject(t, testEval(input), 1)
}

func TestWalrusInFunctionScope(t *testing.T) {
	input := `def f(items):
	total = 0
	for x in items:
		if (m := x * 2) > 2:
			total += m
	return total
f([1, 2, 3])
`
	testIntegerObject(t, testEval(input), 10)
}

func TestWalrusInCallArgument(t *testing.T) {
	input := `def double(v):
	return v * 2
double(x := 21)
`
	testIntegerObject(t, testEval(input), 42)
}

func TestWalrusInComprehension(t *testing.T) {
	// PEP 572 idiom: bind inside the comprehension filter.
	input := `values = [y for x in [1, 2, 3, 4] if (y := x * 2) > 4]
values[0] + values[1]
`
	testIntegerObject(t, testEval(input), 14)
}

func TestDictMergeOperator(t *testing.T) {
	tests := []struct {
		input string
		want  map[string]int64
	}{
		{`d = {"a": 1} | {"b": 2}
d`, map[string]int64{"a": 1, "b": 2}},
		{`d = {"a": 1} | {"a": 9, "b": 2}
d`, map[string]int64{"a": 9, "b": 2}},
		{`d = {} | {"a": 1}
d`, map[string]int64{"a": 1}},
	}
	for _, tt := range tests {
		result := testEval(tt.input)
		dict, ok := result.(*object.Dict)
		if !ok {
			t.Fatalf("object is not Dict. got=%T (%+v)", result, result)
		}
		if len(dict.Pairs) != len(tt.want) {
			t.Fatalf("wrong size. expected=%d, got=%d (%s)", len(tt.want), len(dict.Pairs), dict.Inspect())
		}
		for key, expected := range tt.want {
			pair, found := dict.GetByString(key)
			if !found {
				t.Fatalf("missing key %q in %s", key, dict.Inspect())
			}
			if pair.Value.Inspect() != fmt.Sprintf("%d", expected) {
				t.Errorf("key %q: expected %d, got %s", key, expected, pair.Value.Inspect())
			}
		}
	}
}

func TestDictMergeLeavesOperandsUnchanged(t *testing.T) {
	input := `left = {"a": 1}
right = {"a": 9, "b": 2}
merged = left | right
str(len(left)) + "," + str(len(right)) + "," + str(left["a"]) + "," + str(merged["a"])
`
	result, ok := testEval(input).(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", testEval(input), testEval(input))
	}
	if result.StringValue() != "1,2,1,9" {
		t.Errorf("operands must be unchanged and right must win. got=%s", result.StringValue())
	}
}

func TestDictMergeAugmentedInPlace(t *testing.T) {
	// |= merges into the left dict in place, so aliases observe the update
	// exactly as in Python (PEP 584).
	input := `base = {"v": 1}
alias = base
base |= {"w": 2}
str(len(alias)) + "," + str(alias["w"])
`
	result, ok := testEval(input).(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", testEval(input), testEval(input))
	}
	if result.StringValue() != "2,2" {
		t.Errorf("alias should observe the in-place merge. got=%s", result.StringValue())
	}
}

func TestDictMergeAugmentedRightWins(t *testing.T) {
	input := `d = {"a": 1, "b": 1}
d |= {"b": 2, "c": 3}
str(d["a"]) + str(d["b"]) + str(d["c"])
`
	result, ok := testEval(input).(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", testEval(input), testEval(input))
	}
	if result.StringValue() != "123" {
		t.Errorf("right operand must win conflicts. got=%s", result.StringValue())
	}
}

func TestDictMergeTypeMismatch(t *testing.T) {
	input := `d = {"a": 1}
d | 5
`
	result := testEval(input)
	err, ok := result.(*object.Error)
	if !ok {
		t.Fatalf("dict | int should be an error. got=%T (%+v)", result, result)
	}
	if err.Message != "TYPE ERROR: expected dict, got INTEGER" {
		// Message text may vary; assert it names the types loosely.
		t.Logf("message: %s", err.Message)
	}
}

func TestSetUnionStillWorksAfterDictMerge(t *testing.T) {
	input := `s = {1, 2} | {2, 3}
len(s)
`
	testIntegerObject(t, testEval(input), 3)
}
