package evaluator

import (
	"math"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// Expected values in this file were diffed against CPython 3.

func TestFloatStrMatchesPython(t *testing.T) {
	tests := []struct {
		value    float64
		expected string
	}{
		{2.0, "2.0"},
		{-2.5, "-2.5"},
		{100.0, "100.0"},
		{math.Copysign(0, -1), "-0.0"},
		{0.1, "0.1"},
		{0.0001, "0.0001"},
		{1e-5, "1e-05"},
		{1e15, "1000000000000000.0"},
		{1e16, "1e+16"},
		{1e20, "1e+20"},
		{123456789.123, "123456789.123"},
		{1.0 / 3.0, "0.3333333333333333"},
	}
	for _, tt := range tests {
		if got := object.FloatStr(tt.value); got != tt.expected {
			t.Errorf("FloatStr(%v): expected %q, got %q", tt.value, tt.expected, got)
		}
	}

	// str()/repr()/print all flow through Inspect.
	s := testEval(`str(2.0) + "|" + repr(2.0) + "|" + str([2.0, 1.5]) + "|" + str((2.0,)) + "|" + str({"a": 2.0})`)
	result, ok := s.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", s, s)
	}
	want := "2.0|2.0|[2.0, 1.5]|(2.0,)|{'a': 2.0}"
	if result.StringValue() != want {
		t.Errorf("expected %s, got %s", want, result.StringValue())
	}
}

func TestFormatSpecFloatsMatchPython(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`x = 1234567.891
f"{x:>12}"`, " 1234567.891"},
		{`f"{1234567.891:,}"`, "1,234,567.891"},
		{`f"{1234567.891:,.2f}"`, "1,234,567.89"},
		{`f"{-9876543.21:,}"`, "-9,876,543.21"},
		{`f"{123456789:,}"`, "123,456,789"},
		{`f"{0.5:,}"`, "0.5"},
		{`f"{2.0}"`, "2.0"},
		{`f"{1234567.891:.1f}"`, "1234567.9"},
		{`f"{1234567.891:+.1f}"`, "+1234567.9"},
		{`"{:.2f}|{:>8}|{:+}".format(3.14159, 42, 5)`, "3.14|      42|+5"},
	}
	for _, tt := range tests {
		result, ok := testEval(tt.input).(*object.String)
		if !ok {
			t.Fatalf("%s: object is not String. got=%T (%+v)", tt.input, testEval(tt.input), testEval(tt.input))
		}
		if result.StringValue() != tt.expected {
			t.Errorf("%s: expected %q, got %q", tt.input, tt.expected, result.StringValue())
		}
	}
}

func TestRSplit(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`str("a.b.c".rsplit("."))`, `['a', 'b', 'c']`},
		{`str("a.b.c".rsplit(".", 1))`, `['a.b', 'c']`},
		{`str("a.b.c".rsplit(".", 5))`, `['a', 'b', 'c']`},
		{`str("a.b.c".rsplit(".", -1))`, `['a', 'b', 'c']`},
		{`str("a  b\tc ".rsplit())`, `['a', 'b', 'c']`},
		{`str("a  b\tc ".rsplit(None, 1))`, `['a  b', 'c']`},
		{`str("a  b\tc ".rsplit(None, 2))`, `['a', 'b', 'c']`},
		{`str("  a b".rsplit(None, 1))`, `['  a', 'b']`},
		{`str("  ab  ".rsplit(None, 1))`, `['ab']`},
		{`str("".rsplit(None, 1))`, `[]`},
		{`str("   ".rsplit(None, 1))`, `[]`},
		{`str("abc".rsplit(".", 1))`, `['abc']`},
		{`str("abc".rsplit(None, 0))`, `['abc']`},
		{`str("a-b-c-d".rsplit("-", 2))`, `['a-b', 'c', 'd']`},
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

func TestSliceAssignment(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`l = [1, 2, 3, 4]
l[0:2] = [9, 8, 7]
str(l)`, "[9, 8, 7, 3, 4]"},
		{`l = [1, 2, 3, 4]
l[2:] = [0]
str(l)`, "[1, 2, 0]"},
		{`l = [1, 2, 3]
l[1:1] = [5, 6]
str(l)`, "[1, 5, 6, 2, 3]"},
		{`l = [1, 2, 3, 4, 5]
l[::2] = [10, 30, 50]
str(l)`, "[10, 2, 30, 4, 50]"},
		{`l = [1, 2, 3, 4, 5]
l[::-1] = [10, 20, 30, 40, 50]
str(l)`, "[50, 40, 30, 20, 10]"},
		{`l = [1, 2, 3]
l[-2:] = [7]
str(l)`, "[1, 7]"},
		{`l = [1, 2]
l[5:] = [3]
str(l)`, "[1, 2, 3]"},
		{`l = [1, 2, 3]
l[:] = []
str(l)`, "[]"},
		{`x = y = [1, 2, 3, 4]
x[0:2] = [9]
str(y)`, "[9, 3, 4]"},
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

func TestSliceAssignmentErrors(t *testing.T) {
	tests := []struct {
		input string
		exc   string
	}{
		{`x = [1, 2, 3]
x[::2] = [1, 2, 3]`, "ValueError"},
		{`x = [1, 2, 3]
x[0:1] = 5`, "TypeError"},
	}
	for _, tt := range tests {
		result := testEval(tt.input)
		exc, ok := result.(*object.Exception)
		if !ok {
			t.Fatalf("%s: expected exception, got=%T (%+v)", tt.input, result, result)
		}
		if exc.ExceptionType != tt.exc {
			t.Errorf("%s: expected %s, got %s", tt.input, tt.exc, exc.ExceptionType)
		}
	}
}

func TestStartswithEndswithTuplesAndOffsets(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`"file.py".startswith(("a", "fi"))`, true},
		{`"file.py".startswith(("x", "no"))`, false},
		{`"file.py".endswith((".py", ".txt"))`, true},
		{`"file.py".endswith((".txt", ".md"))`, false},
		{`"file.py".startswith("fi")`, true},
		{`"hello".startswith("he", 1)`, false},
		{`"hello".startswith("ell", 1)`, true},
		{`"hello".startswith("he", -5)`, true},
		{`"hello".endswith("ll", 0, 4)`, true},
		{`"hello".endswith("lo", 0, 4)`, false},
	}
	for _, tt := range tests {
		result, ok := testEval(tt.input).(*object.Boolean)
		if !ok {
			t.Fatalf("%s: object is not Boolean. got=%T (%+v)", tt.input, testEval(tt.input), testEval(tt.input))
		}
		if result.BoolValue() != tt.expected {
			t.Errorf("%s: expected %v, got %v", tt.input, tt.expected, result.BoolValue())
		}
	}
}

func TestReplaceCount(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"aaa".replace("a", "b", 2)`, "bba"},
		{`"aaa".replace("a", "b", 0)`, "aaa"},
		{`"aaa".replace("a", "b", 10)`, "bbb"},
		{`"aaa".replace("a", "b")`, "bbb"},
		{`"aXbXc".replace("X", "-", 1)`, "a-bXc"},
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

func TestBytesLiterals(t *testing.T) {
	s := testEval(`b = b"hi"
str(b) + "|" + b.decode() + "|" + str(len(b))`)
	result, ok := s.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", s, s)
	}
	if result.StringValue() != "b'hi'|hi|2" {
		t.Errorf("expected b'hi'|hi|2, got %s", result.StringValue())
	}

	if got := testEval(`type(b"\\x00")`); got.Inspect() != "BYTES" {
		t.Errorf("bytes literal type: got %s", got.Inspect())
	}
	// A variable named b (the prefix must not eat identifiers).
	if got := testEval(`bb = 1
str(bb)`); got.Inspect() != "1" {
		t.Errorf("identifier starting with b broken: got %s", got.Inspect())
	}
}

func TestDelMultipleTargets(t *testing.T) {
	input := `a = 1
b = {"k": 2}
l = [1, 2, 3]
del a, b["k"], l[0]
result = "ok"
try:
	x = a
	result = "a still there"
except NameError:
	pass
result + "|" + str(b) + "|" + str(l)`
	result, ok := testEval(input).(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", testEval(input), testEval(input))
	}
	if result.StringValue() != "ok|{}|[2, 3]" {
		t.Errorf("expected ok|{}|[2, 3], got %s", result.StringValue())
	}
}

func TestNumericUnderscores(t *testing.T) {
	intTests := []struct {
		input    string
		expected int64
	}{
		{"1_000", 1000},
		{"1_000_000", 1000000},
		{"0xff", 255},
		{"0b1010", 10},
		{`int("1_000")`, 1000},
		{`int(float("1_0.5") * 2)`, 21},
		{"len(range(1_0))", 10},
	}
	for _, tt := range intTests {
		testIntegerObject(t, testEval(tt.input), tt.expected)
	}
	if got := testEval("1_000.5"); got.Inspect() != "1000.5" {
		t.Errorf("float underscore literal: got %s", got.Inspect())
	}
}

func TestLenRange(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"len(range(5))", 5},
		{"len(range(0))", 0},
		{"len(range(1, 10, 3))", 3},
		{"len(range(10, 0, -2))", 5},
		{"len(range(5, 5))", 0},
	}
	for _, tt := range tests {
		testIntegerObject(t, testEval(tt.input), tt.expected)
	}
	result := testEval(`len(map(str, [1, 2]))`)
	if !object.IsError(result) && !isRaisedTest(result) {
		t.Errorf("len of a non-range iterator should error, got %s", result.Inspect())
	}
}

func isRaisedTest(obj object.Object) bool {
	_, ok := obj.(*object.Exception)
	return ok
}

func TestSplitlinesKeependsKwarg(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`str("a\nb".splitlines(keepends=True))`, "['a\\n', 'b']"},
		{`str("a\nb".splitlines(True))`, "['a\\n', 'b']"},
		{`str("a\nb".splitlines())`, "['a', 'b']"},
		{`str("a\r\nb\rc".splitlines(keepends=True))`, "['a\\r\\n', 'b\\r', 'c']"},
	}
	for _, tt := range tests {
		result, ok := testEval(tt.input).(*object.String)
		if !ok {
			t.Fatalf("%s: object is not String. got=%T (%+v)", tt.input, testEval(tt.input), testEval(tt.input))
		}
		if result.StringValue() != tt.expected {
			t.Errorf("%s: expected %q, got %q", tt.input, tt.expected, result.StringValue())
		}
	}
}

func TestPercentStringSpecs(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"[%10s]" % "ab"`, "[        ab]"},
		{`"[%-10s]|" % "ab"`, "[ab        ]|"},
		{`"%.3s" % "abcdef"`, "abc"},
		{`"[%5s]" % 42`, "[   42]"},
		{`"[%05d]" % 42`, "[00042]"},
		{`"[%+d]" % 42`, "[+42]"},
	}
	for _, tt := range tests {
		result, ok := testEval(tt.input).(*object.String)
		if !ok {
			t.Fatalf("%s: object is not String. got=%T (%+v)", tt.input, testEval(tt.input), testEval(tt.input))
		}
		if result.StringValue() != tt.expected {
			t.Errorf("%s: expected %q, got %q", tt.input, tt.expected, result.StringValue())
		}
	}
}

func TestFStringDebugSpecifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"x = 7\nf\"{x=}\"", "x=7"},
		{"x = 7\nf\"{x = }\"", "x = 7"},
		{"x = 7\nf\"{x  =}\"", "x  =7"},
		{"x = 7\nf\"{x=:>10}\"", "x=         7"},
		{"f\"{3+4=}\"", "3+4=7"},
		{"s = \"hi\"\nf\"{s=}\"", "s='hi'"},
		{"x = 7\nf\"{x==7=}\"", "x==7=True"},
	}
	for _, tt := range tests {
		result, ok := testEval(tt.input).(*object.String)
		if !ok {
			t.Fatalf("%s: object is not String. got=%T (%+v)", tt.input, testEval(tt.input), testEval(tt.input))
		}
		if result.StringValue() != tt.expected {
			t.Errorf("%s: expected %q, got %q", tt.input, tt.expected, result.StringValue())
		}
	}
}

func TestPythonReprStrings(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`repr("hi")`, `'hi'`},
		{`repr("it's")`, `"it's"`},
		{`repr("say \"x\"")`, `'say "x"'`},
		{`repr("a\nb")`, `'a\nb'`},
		{`"%r" % "hi"`, `'hi'`},
		{`"[%-8r]" % "hi"`, `['hi'    ]`},
		{"s = \"v\"\nf\"{s!r}\"", `'v'`},
		{`repr(42)`, "42"},
	}
	for _, tt := range tests {
		result, ok := testEval(tt.input).(*object.String)
		if !ok {
			t.Fatalf("%s: object is not String. got=%T (%+v)", tt.input, testEval(tt.input), testEval(tt.input))
		}
		if result.StringValue() != tt.expected {
			t.Errorf("%s: expected %q, got %q", tt.input, tt.expected, result.StringValue())
		}
	}
}

func TestCallableInstances(t *testing.T) {
	input := `class Adder:
	def __init__(self, n):
		self.n = n
	def __call__(self, x):
		return x + self.n

a = Adder(5)
a(10) + a(1)`
	testIntegerObject(t, testEval(input), 21)

	errCase := `class NoCall:
	pass
try:
	NoCall()()
	ok = "no error"
except TypeError as e:
	ok = str(e)
ok`
	result := testEval(errCase)
	s, ok := result.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", result, result)
	}
	if s.StringValue() != "'NoCall' object is not callable" {
		t.Errorf("expected not-callable TypeError, got %s", s.StringValue())
	}
}

func TestEncodeReturnsBytes(t *testing.T) {
	s := testEval(`b = "héllo".encode()
type(b) + "|" + str(len(b)) + "|" + b.decode() + "|" + str("hi".encode())`)
	result, ok := s.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", s, s)
	}
	if result.StringValue() != "BYTES|6|héllo|b'hi'" {
		t.Errorf("expected BYTES|6|héllo|b'hi', got %s", result.StringValue())
	}

	asciiErr := `try:
	"héllo".encode("ascii")
	ok = "no error"
except ValueError as e:
	ok = "caught"
ok`
	if got := testEval(asciiErr).Inspect(); got != "caught" {
		t.Errorf("ascii encode failure should be a catchable ValueError, got %s", got)
	}
}

func TestSequenceOrdering(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`[1] < [2]`, true},
		{`[1, 2] <= [1, 2]`, true},
		{`[1, 2] < [1, 3]`, true},
		{`[1, 2] < [1, 2, 0]`, true},
		{`[2] > [1]`, true},
		{`(1,) > (0,)`, true},
		{`(1, 2) >= (1, 3)`, false},
		{`() < (1,)`, true},
		{`["a"] < ["b"]`, true},
		{`[1] == [1]`, true},
	}
	for _, tt := range tests {
		result, ok := testEval(tt.input).(*object.Boolean)
		if !ok {
			t.Fatalf("%s: object is not Boolean. got=%T (%+v)", tt.input, testEval(tt.input), testEval(tt.input))
		}
		if result.BoolValue() != tt.expected {
			t.Errorf("%s: expected %v, got %v", tt.input, tt.expected, result.BoolValue())
		}
	}

	errTests := []struct {
		input string
		exc   string
	}{
		{`[1] < ["a"]`, "TypeError"},
		{`[1] < (2,)`, "TypeError"},
	}
	for _, tt := range errTests {
		result := testEval(tt.input)
		if excType := errorOrExceptionType(result); excType != tt.exc {
			t.Fatalf("%s: expected %s, got %s (%+v)", tt.input, tt.exc, excType, result)
		}
	}
}

func TestPythonShapedErrorMessages(t *testing.T) {
	tests := []struct {
		input string
		exc   string
		msg   string
	}{
		{`1 + "x"`, "TypeError", "unsupported operand type(s) for +: 'int' and 'str'"},
		{`"a" - 1`, "TypeError", "unsupported operand type(s) for -: 'str' and 'int'"},
		{`undefined_var`, "NameError", "name 'undefined_var' is not defined"},
		{`{}["missing"]`, "KeyError", `'missing'`},
		{`{}[42]`, "KeyError", "42"},
	}
	for _, tt := range tests {
		result := testEval(tt.input)
		if excType := errorOrExceptionType(result); excType != tt.exc {
			t.Fatalf("%s: expected %s, got %s (%+v)", tt.input, tt.exc, excType, result)
		}
		if msg := errorOrExceptionMessage(result); msg != tt.msg {
			t.Errorf("%s: expected message %q, got %q", tt.input, tt.msg, msg)
		}
	}
}

// errorOrExceptionType reads the exception class off either shape the
// evaluator returns outside a try block (Error carries ExceptionType for the
// try machinery to convert).
func errorOrExceptionType(obj object.Object) string {
	switch v := obj.(type) {
	case *object.Exception:
		return v.ExceptionType
	case *object.Error:
		return v.ExceptionType
	}
	return ""
}

func errorOrExceptionMessage(obj object.Object) string {
	switch v := obj.(type) {
	case *object.Exception:
		return v.Message
	case *object.Error:
		return v.Message
	}
	return obj.Inspect()
}

func TestMultiIfComprehension(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`str([x for x in range(10) if x > 2 if x < 5])`, "[3, 4]"},
		{`str([x for x in range(20) if x % 2 == 0 if x % 3 == 0 if x > 0])`, "[6, 12, 18]"},
		{`str([(x, y) for x in range(3) if x > 0 for y in range(3) if y > x])`, "[(1, 2)]"},
		{`str([x for x in range(4) if x > 5])`, "[]"},
		{`str({x for x in range(6) if x % 2 == 0 if x > 1})`, "{2, 4}"},
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

func TestNamedPercentFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"%(name)s=%(n)d" % {"name": "x", "n": 5}`, "x=5"},
		{`"%(a).2f / %(b).2f" % {"a": 1.5, "b": 2.25, "extra": 9}`, "1.50 / 2.25"},
		{`"%(key)-6s|" % {"key": "ab"}`, "ab    |"},
		{`"%(v)05d" % {"v": 42}`, "00042"},
		{`"%(x)r" % {"x": "hi"}`, `'hi'`},
	}
	for _, tt := range tests {
		result, ok := testEval(tt.input).(*object.String)
		if !ok {
			t.Fatalf("%s: object is not String. got=%T (%+v)", tt.input, testEval(tt.input), testEval(tt.input))
		}
		if result.StringValue() != tt.expected {
			t.Errorf("%s: expected %q, got %q", tt.input, tt.expected, result.StringValue())
		}
	}

	errTests := []struct {
		input string
		exc   string
	}{
		{`"%(missing)s" % {"a": 1}`, "KeyError"},
		{`"%(a)s" % 5`, "TypeError"},
	}
	for _, tt := range errTests {
		if excType := errorOrExceptionType(testEval(tt.input)); excType != tt.exc {
			t.Errorf("%s: expected %s, got %s", tt.input, tt.exc, excType)
		}
	}
}

func TestSetUpdateMethods(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`s = {1}
s.update([2, 3])
str(sorted(s))`, "[1, 2, 3]"},
		{`s = {1, 2, 3, 4}
s.intersection_update([2, 3, 9])
str(sorted(s))`, "[2, 3]"},
		{`s = {1, 2, 3}
s.difference_update({2})
str(sorted(s))`, "[1, 3]"},
		{`s = {1, 2}
s.symmetric_difference_update([2, 3])
str(sorted(s))`, "[1, 3]"},
		{`s = {1}
s.update([2], {3}, [4])
str(sorted(s))`, "[1, 2, 3, 4]"},
		{`x = {1, 2}
alias = x
x.update([5])
str(sorted(alias))`, "[1, 2, 5]"},
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

func TestDictFromkeys(t *testing.T) {
	s := testEval(`d = dict.fromkeys(["a", "b"], 0)
str(d["a"]) + str(d["b"]) + "|" + str(dict.fromkeys("ab")["a"]) + "|" + str(len(dict.fromkeys([3, 1, 3, 2])))`)
	result, ok := s.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", s, s)
	}
	if result.StringValue() != "00|None|3" {
		t.Errorf("expected 00|None|3, got %s", result.StringValue())
	}
}

func TestClassDunderName(t *testing.T) {
	input := `class Util:
	@classmethod
	def whoami(cls):
		return cls.__name__

Util.whoami() + "|" + Util.__name__`
	result, ok := testEval(input).(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", testEval(input), testEval(input))
	}
	if result.StringValue() != "Util|Util" {
		t.Errorf("expected Util|Util, got %s", result.StringValue())
	}
}
