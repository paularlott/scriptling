package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

// Values in this file were diffed against real Python 3.

func TestIntegerFloorDivModPythonSemantics(t *testing.T) {
	tests := []struct {
		div, mod int64
		left     int64
		right    int64
	}{
		{3, 1, 7, 2},    // same signs, exact-free baseline
		{-4, 1, -7, 2},  // floor toward negative infinity
		{-4, -1, 7, -2}, // % takes the divisor's sign
		{3, -1, -7, -2},
		{-4, 0, -8, 2},
		{-3, 0, -9, 3},
	}
	for _, tt := range tests {
		// Literals exercise the parse-time constant folder; variables
		// exercise the evaluator's fast and general integer paths.
		lit := testEval(`[` + itoa(tt.left) + ` // ` + itoa(tt.right) + `, ` + itoa(tt.left) + ` % ` + itoa(tt.right) + `]`)
		vars := testEval(`a = ` + itoa(tt.left) + `
b = ` + itoa(tt.right) + `
[a // b, a % b]`)
		for name, result := range map[string]object.Object{"literal": lit, "variable": vars} {
			tuple, ok := result.(*object.List)
			if !ok {
				t.Fatalf("%s: object is not List. got=%T (%+v)", name, result, result)
			}
			testIntegerObject(t, tuple.Elements[0], tt.div)
			testIntegerObject(t, tuple.Elements[1], tt.mod)
		}
	}
}

func TestFloatModuloPythonSemantics(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{`5.5 % 3`, 2.5},
		{`-7.5 % 2`, 0.5},
		{`7.5 % -2`, -0.5},
		{`-7.5 % -2`, -1.5},
		{`5 % 2.5`, 0},
	}
	for _, tt := range tests {
		result, ok := testEval(tt.input).(*object.Float)
		if !ok {
			t.Fatalf("%s: object is not Float. got=%T (%+v)", tt.input, testEval(tt.input), testEval(tt.input))
		}
		if result.FloatValue() != tt.expected {
			t.Errorf("%s: expected %v, got %v", tt.input, tt.expected, result.FloatValue())
		}
	}
}

func TestEnumerateStartKwarg(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`str(list(enumerate(["a", "b"])))`, `[(0, a), (1, b)]`},
		{`str(list(enumerate(["a", "b"], 1)))`, `[(1, a), (2, b)]`},
		{`str(list(enumerate(["a", "b"], start=1)))`, `[(1, a), (2, b)]`},
		{`str(list(enumerate(["a"], start=5)))`, `[(5, a)]`},
		{`str(list(enumerate(["a"], start=-1)))`, `[(-1, a)]`},
	}
	for _, tt := range tests {
		result, ok := testEval(tt.input).(*object.String)
		if !ok {
			t.Fatalf("%s: object is not String. got=%T (%+v)", tt.input, testEval(tt.input), testEval(tt.input))
		}
		if result.StringValue() != tt.expected {
			t.Errorf("%s: expected %s, got %s", tt.input, tt.expected, result.StringValue())
		}
	}
}

func TestEnumerateStartKwargTypeError(t *testing.T) {
	result := testEval(`list(enumerate(["a"], start="x"))`)
	if !object.IsError(result) {
		t.Fatalf("non-integer start should be an error. got=%T (%+v)", result, result)
	}
}

func TestSortedStability(t *testing.T) {
	// 120 elements with repeated keys: big enough to leave insertion sort
	// behind, so an unstable algorithm reorders equal keys.
	input := `pairs = [(i % 4, i) for i in range(120)]
result = sorted(pairs, key=lambda t: t[0])
ok = True
for i in range(len(result) - 1):
    if result[i][0] == result[i + 1][0] and result[i][1] > result[i + 1][1]:
        ok = False
ok`
	testTruth(t, testEval(input), "sorted() must be stable")

	inPlace := `pairs = [(i % 4, i) for i in range(120)]
pairs.sort(key=lambda t: t[0])
ok = True
for i in range(len(pairs) - 1):
    if pairs[i][0] == pairs[i + 1][0] and pairs[i][1] > pairs[i + 1][1]:
        ok = False
ok`
	testTruth(t, testEval(inPlace), ".sort() must be stable")

	reversedStable := `pairs = [(i % 4, i) for i in range(120)]
result = sorted(pairs, key=lambda t: t[0], reverse=True)
ok = True
for i in range(len(result) - 1):
    if result[i][0] == result[i + 1][0] and result[i][1] > result[i + 1][1]:
        ok = False
ok`
	testTruth(t, testEval(reversedStable), "reverse=True must keep equal keys in original order")
}

func TestAugmentedAssignMutatesInPlace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"list +=", `
x = y = [1]
x += [2]
str(y)`, "[1, 2]"},
		{"list *=", `
x = y = [1, 2]
x *= 3
str(y)`, "[1, 2, 1, 2, 1, 2]"},
		{"set |=", `
x = y = set([1])
x |= set([2])
str(sorted(y))`, "[1, 2]"},
		{"set &=", `
x = y = set([1, 2, 3])
x &= set([2, 3, 4])
str(sorted(y))`, "[2, 3]"},
		{"set -=", `
x = y = set([1, 2, 3])
x -= set([2])
str(sorted(y))`, "[1, 3]"},
		{"set ^=", `
x = y = set([1, 2, 3])
x ^= set([3, 4])
str(sorted(y))`, "[1, 2, 4]"},
		{"dict |=", `
x = y = {"a": 1}
x |= {"b": 2}
str(len(y)) + str(y["b"])`, "22"},
	}
	for _, tt := range tests {
		result, ok := testEval(tt.input).(*object.String)
		if !ok {
			t.Fatalf("%s: object is not String. got=%T (%+v)", tt.name, testEval(tt.input), testEval(tt.input))
		}
		if result.StringValue() != tt.expected {
			t.Errorf("%s: expected %s, got %s", tt.name, tt.expected, result.StringValue())
		}
	}
}

func TestRoundHalfToEven(t *testing.T) {
	intTests := []struct {
		input    string
		expected int64
	}{
		{`round(0.5)`, 0},
		{`round(1.5)`, 2},
		{`round(2.5)`, 2},
		{`round(3.5)`, 4},
		{`round(-0.5)`, 0},
		{`round(-1.5)`, -2},
		{`round(-2.5)`, -2},
		{`round(-3.5)`, -4},
		{`round(4.4)`, 4},
		{`round(4.6)`, 5},
		{`round(7)`, 7},
		{`round(1250, -2)`, 1200},
		{`round(1350, -2)`, 1400},
		{`round(5, 2)`, 5},
	}
	for _, tt := range intTests {
		testIntegerObject(t, testEval(tt.input), tt.expected)
	}

	floatTests := []struct {
		input    string
		expected float64
	}{
		{`round(2.675, 2)`, 2.67},
		{`round(0.125, 2)`, 0.12},
		{`round(0.375, 2)`, 0.38},
		{`round(0.135, 2)`, 0.14},
		{`round(2.5, 0)`, 2},
		{`round(3.5, 0)`, 4},
		{`round(12.345, 2)`, 12.35},
		{`round(1234.5, -2)`, 1200},
	}
	for _, tt := range floatTests {
		result, ok := testEval(tt.input).(*object.Float)
		if !ok {
			t.Fatalf("%s: object is not Float. got=%T (%+v)", tt.input, testEval(tt.input), testEval(tt.input))
		}
		if result.FloatValue() != tt.expected {
			t.Errorf("%s: expected %v, got %v", tt.input, tt.expected, result.FloatValue())
		}
	}
}

func testTruth(t *testing.T, obj object.Object, msg string) {
	t.Helper()
	b, ok := obj.(*object.Boolean)
	if !ok {
		t.Fatalf("%s: object is not Boolean. got=%T (%+v)", msg, obj, obj)
	}
	if !b.BoolValue() {
		t.Error(msg)
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	digits := ""
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	if neg {
		return "-" + digits
	}
	return digits
}
