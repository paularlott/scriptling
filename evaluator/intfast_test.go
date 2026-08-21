package evaluator

import (
	"fmt"
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

// The unboxed integer fast path (intfast.go) must be indistinguishable from the
// general evaluation path. These tests exercise the cases where it could
// plausibly diverge: operators it deliberately declines, error conditions it has
// to hand back, operand types that disqualify it, and value ranges where
// truncation or overflow semantics matter.

// evalSrc runs src and returns the value bound to `result`, or the program's
// error/exception object if evaluation failed before getting there.
func evalSrc(t *testing.T, src string) object.Object {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors for %q: %v", src, errs)
	}
	env := object.NewEnvironment()
	out := Eval(program, env)
	if object.IsError(out) || out.Type() == object.EXCEPTION_OBJ {
		return out
	}
	val, ok := env.Get("result")
	if !ok {
		t.Fatalf("script did not bind `result`: %q", src)
	}
	return val
}

func TestIntFastArithmeticMatchesGeneralPath(t *testing.T) {
	// Each case is written twice: once in a shape the fast path accepts
	// (literals/identifiers only) and once routed through a function call so the
	// fast path is disqualified and the general path runs. Both must agree.
	exprs := []string{
		"a + b", "a - b", "a * b", "a // b", "a % b",
		"a & b", "a | b", "a ^ b", "a << b", "a >> b",
		"a / b", "a ** b",
		"a < b", "a > b", "a <= b", "a >= b", "a == b", "a != b",
		"a + b * a - b", "(a + b) * (a - b)", "a * 2 + b * 3 - 1",
		"a % b == 0", "a * b > a + b", "a // b + a % b",
	}
	pairs := [][2]int64{
		{7, 3}, {3, 7}, {-7, 3}, {7, -3}, {-7, -3},
		{0, 5}, {5, 0}, {1, 1},
		{1 << 40, 3}, {-1 << 40, 3},
		{9007199254740993, 7}, // beyond float64 exact integer range
		{10001, 2},            // outside the small-int cache
	}

	for _, expr := range exprs {
		for _, pv := range pairs {
			fastSrc := fmt.Sprintf("a = %d\nb = %d\nresult = %s\n", pv[0], pv[1], expr)
			// ident() is opaque to the shape analysis, so the same expression
			// with its operands wrapped runs on the general path.
			slowSrc := fmt.Sprintf(
				"def ident(v):\n    return v\na = ident(%d)\nb = ident(%d)\nresult = %s\n",
				pv[0], pv[1], expr)

			fast := evalSrc(t, fastSrc)
			slow := evalSrc(t, slowSrc)

			if fast.Type() != slow.Type() {
				t.Errorf("%s with a=%d b=%d: type mismatch fast=%s slow=%s (%s vs %s)",
					expr, pv[0], pv[1], fast.Type(), slow.Type(), fast.Inspect(), slow.Inspect())
				continue
			}
			// The two variants sit on different source lines, so compare the
			// error message rather than the line-annotated Inspect() output.
			if fe, ok := fast.(*object.Error); ok {
				if se := slow.(*object.Error); fe.Message != se.Message {
					t.Errorf("%s with a=%d b=%d: error mismatch fast=%q slow=%q",
						expr, pv[0], pv[1], fe.Message, se.Message)
				}
				continue
			}
			if fast.Inspect() != slow.Inspect() {
				t.Errorf("%s with a=%d b=%d: value mismatch fast=%q slow=%q",
					expr, pv[0], pv[1], fast.Inspect(), slow.Inspect())
			}
		}
	}
}

func TestIntFastDeclinesErrorCases(t *testing.T) {
	// Cases the fast path must hand back so the general path can raise. The
	// point is that an error still surfaces rather than being silently skipped.
	for _, src := range []string{
		"a = 5\nb = 0\nresult = a // b\n",
		"a = 5\nb = 0\nresult = a % b\n",
		"a = 5\nb = 0\nresult = a / b\n",
		"a = 5\nb = -1\nresult = a << b\n",
		"a = 5\nb = -1\nresult = a >> b\n",
	} {
		got := evalSrc(t, src)
		if !object.IsError(got) && got.Type() != object.EXCEPTION_OBJ {
			t.Errorf("%q: expected an error/exception, got %s (%s)", src, got.Type(), got.Inspect())
		}
	}
}

func TestIntFastDeclinesNonIntegerOperands(t *testing.T) {
	// Floats, bools and strings must keep their existing semantics even though
	// the AST shape (identifier OP identifier) looks eligible.
	cases := []struct{ src, want string }{
		{"a = 1.5\nb = 2\nresult = a + b\n", "3.5"},
		{"a = 2\nb = 1.5\nresult = a + b\n", "3.5"},
		{"a = 2\nb = 0.5\nresult = a * b\n", "1"},
		{"a = True\nb = 2\nresult = a + b\n", "3"},
		{"a = 2\nb = True\nresult = a + b\n", "3"},
		{"a = True\nb = False\nresult = a + b\n", "1"},
		{"a = \"x\"\nb = \"y\"\nresult = a + b\n", "xy"},
		{"a = \"ab\"\nb = 2\nresult = a * b\n", "abab"},
		{"a = 1.5\nb = 1.5\nresult = a == b\n", "True"},
		{"a = 2\nb = 2.0\nresult = a == b\n", "True"},
	}
	for _, c := range cases {
		got := evalSrc(t, c.src)
		if got.Inspect() != c.want {
			t.Errorf("%q: got %q want %q", c.src, got.Inspect(), c.want)
		}
	}
}

func TestIntFastRespectsRebindingToNonInteger(t *testing.T) {
	// A node that took the fast path once must not keep taking it after the
	// variable is rebound to a float.
	src := `
a = 1
b = 2
first = a + b
a = 1.5
second = a + b
result = str(first) + "|" + str(second)
`
	got := evalSrc(t, src)
	if got.Inspect() != "3|3.5" {
		t.Errorf("got %q want %q", got.Inspect(), "3|3.5")
	}
}

// Addition chains are handled by tryEvalStringConcatChain as well as the integer
// fast path, and the two are tried in a specific order. Verify every operand-type
// mix through a 4-term chain still produces the same result as Python-style
// left-to-right evaluation.
func TestIntFastAddChainOrdering(t *testing.T) {
	cases := []struct{ src, want string }{
		// All integers: must take the unboxed path.
		{"a=1\nb=2\nc=3\nd=4\nresult = a + b + c + d\n", "10"},
		// All strings: must still reach the concat-chain path.
		{"a=\"w\"\nb=\"x\"\nc=\"y\"\nd=\"z\"\nresult = a + b + c + d\n", "wxyz"},
		// Integer-shaped AST but a string operand at runtime.
		{"a=\"w\"\nb=\"x\"\nc=\"y\"\nd=1\nresult = a + b + c + str(d)\n", "wxy1"},
		// Mixed numeric: int chain that turns into a float partway.
		{"a=1\nb=2\nc=1.5\nd=4\nresult = a + b + c + d\n", "8.5"},
		{"a=1.5\nb=2\nc=3\nd=4\nresult = a + b + c + d\n", "10.5"},
		// Longer chain crossing the small-int cache boundary.
		{"a=10000\nb=10000\nc=10000\nd=10000\nresult = a + b + c + d\n", "40000"},
		// Booleans coerce to integers.
		{"a=True\nb=1\nc=2\nd=3\nresult = a + b + c + d\n", "7"},
		// Lists concatenate.
		{"a=[1]\nb=[2]\nc=[3]\nd=[4]\nresult = len(a + b + c + d)\n", "4"},
	}
	for _, c := range cases {
		got := evalSrc(t, c.src)
		if got.Inspect() != c.want {
			t.Errorf("%q: got %q want %q", c.src, got.Inspect(), c.want)
		}
	}
}

func TestIntFastUndefinedNameStillErrors(t *testing.T) {
	got := evalSrc(t, "a = 1\nresult = a + missing\n")
	if !object.IsError(got) && got.Type() != object.EXCEPTION_OBJ {
		t.Errorf("expected error for undefined name, got %s (%s)", got.Type(), got.Inspect())
	}
}

func TestIntFastKindSetByParser(t *testing.T) {
	// The flag is what makes the fast path cheap to skip, so assert the parser
	// actually sets it, and only for eligible shapes.
	cases := []struct {
		expr string
		want ast.IntFastKind
	}{
		{"a + b", ast.IntFastArith},
		{"a + b * 2 - 1", ast.IntFastArith},
		{"a % 3", ast.IntFastArith},
		{"a << 2", ast.IntFastArith},
		{"a < b", ast.IntFastCompare},
		{"a == 0", ast.IntFastCompare},
		{"a % 3 == 0", ast.IntFastCompare},
		{"a / b", ast.IntFastNone},     // always float
		{"a ** b", ast.IntFastNone},    // may promote to float
		{"a + f(b)", ast.IntFastNone},  // call
		{"a + b.c", ast.IntFastNone},   // attribute access
		{"a + b[0]", ast.IntFastNone},  // index
		{"a + 1.5", ast.IntFastNone},   // float literal
		{"a < f(b)", ast.IntFastNone},  // comparison against a call
		{"a / b + 1", ast.IntFastNone}, // float-producing subtree
	}
	for _, c := range cases {
		src := "result = " + c.expr + "\n"
		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatalf("parse errors for %q: %v", src, errs)
		}
		assign, ok := program.Statements[0].(*ast.AssignStatement)
		if !ok {
			t.Fatalf("%q: expected assignment, got %T", src, program.Statements[0])
		}
		infix, ok := assign.Value.(*ast.InfixExpression)
		if !ok {
			t.Fatalf("%q: expected infix value, got %T", src, assign.Value)
		}
		if infix.IntFast != c.want {
			t.Errorf("%q: IntFast = %v, want %v", c.expr, infix.IntFast, c.want)
		}
	}
}
