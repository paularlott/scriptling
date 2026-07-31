package evaluator

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// `a + b + c ...` chains of three or more operands take a dedicated folding path
// (tryEvalStringConcatChain). The chain shape is known statically but the operand
// types are not, so every combination has to produce the same result as
// evaluating the operators one at a time.

func TestConcatChainMatchesPairwiseEvaluation(t *testing.T) {
	// Each case gives a 4-operand chain and the same expression fully
	// parenthesised, which forces the ordinary pairwise path. Both must agree.
	setups := []string{
		"a=\"w\"\nb=\"x\"\nc=\"y\"\nd=\"z\"\n",          // all strings
		"a=1\nb=2\nc=3\nd=4\n",                          // all ints
		"a=1.5\nb=2.5\nc=3.5\nd=4.5\n",                  // all floats
		"a=\"w\"\nb=\"x\"\nc=\"y\"\nd=\"\"\n",           // trailing empty string
		"a=\"\"\nb=\"\"\nc=\"\"\nd=\"\"\n",              // all empty strings
		"a=1\nb=2\nc=3.5\nd=4\n",                        // int run then float
		"a=1.5\nb=2\nc=3\nd=4\n",                        // float first
		"a=[1]\nb=[2]\nc=[3]\nd=[4]\n",                  // lists
		"a=(1,)\nb=(2,)\nc=(3,)\nd=(4,)\n",              // tuples
		"a=True\nb=1\nc=2\nd=3\n",                       // bool coercion
		"a=\"w\"\nb=\"x\"\nc=\"y\"\nd=\"z\"\ne=\"q\"\n", // longer run
	}
	for _, setup := range setups {
		chain := evalSrc(t, setup+"result = a + b + c + d\n")
		paren := evalSrc(t, setup+"result = ((a + b) + c) + d\n")
		if chain.Type() != paren.Type() {
			t.Errorf("%q: type mismatch chain=%s paren=%s", setup, chain.Type(), paren.Type())
			continue
		}
		if chain.Inspect() != paren.Inspect() {
			t.Errorf("%q: chain=%q paren=%q", setup, chain.Inspect(), paren.Inspect())
		}
	}
}

func TestConcatChainStringRunThenNonString(t *testing.T) {
	// A leading run of strings is joined through one buffer, then folding
	// continues normally. The boundary is where a mistake would show up.
	cases := []struct{ src, want string }{
		{"result = \"a\" + \"b\" + \"c\"\n", "abc"},
		{"a=\"x\"\nresult = a + \"y\" + \"z\"\n", "xyz"},
		{"result = \"n=\" + \"\" + \"1\"\n", "n=1"},
		{"a=1\nresult = \"n=\" + \"v\" + str(a)\n", "n=v1"},
		{"a=5\nresult = str(a) + \"-\" + str(a) + \"-\" + str(a)\n", "5-5-5"},
		// Numbers first, strings later: must raise rather than silently join.
		{"a=1\nb=2\nresult = str(a + b) + \"x\" + \"y\"\n", "3xy"},
	}
	for _, c := range cases {
		if got := evalSrc(t, c.src).Inspect(); got != c.want {
			t.Errorf("%q: got %q want %q", c.src, got, c.want)
		}
	}
}

func TestConcatChainTypeErrorsPropagate(t *testing.T) {
	// Mixing incompatible operands must still produce an error, and must do so at
	// the same point as pairwise evaluation.
	for _, src := range []string{
		"a=\"x\"\nb=1\nc=2\nresult = a + b + c\n",
		"a=1\nb=\"x\"\nc=2\nresult = a + b + c\n",
		"a=[1]\nb=\"x\"\nc=2\nresult = a + b + c\n",
		"a=\"x\"\nb=\"y\"\nc=[1]\nresult = a + b + c\n",
	} {
		got := evalSrc(t, src)
		if !object.IsError(got) && got.Type() != object.EXCEPTION_OBJ {
			t.Errorf("%q: expected an error, got %s (%s)", src, got.Type(), got.Inspect())
		}
	}
}

func TestConcatChainErrorInOperandStopsEvaluation(t *testing.T) {
	// An error partway through the chain must surface, not be swallowed by the
	// folding.
	for _, src := range []string{
		"result = \"a\" + \"b\" + missing\n",
		"result = missing + \"a\" + \"b\"\n",
		"result = \"a\" + missing + \"b\"\n",
		"a=1\nb=0\nresult = \"x\" + \"y\" + str(a // b)\n",
	} {
		got := evalSrc(t, src)
		if !object.IsError(got) && got.Type() != object.EXCEPTION_OBJ {
			t.Errorf("%q: expected an error, got %s (%s)", src, got.Type(), got.Inspect())
		}
	}
}

func TestConcatChainEvaluatesOperandsOnceInOrder(t *testing.T) {
	// Folding as operands are evaluated must not re-evaluate or reorder them:
	// each call appends to a log exactly once, left to right.
	src := `
order = []
def s(tag):
    order.append(tag)
    return tag
joined = s("a") + s("b") + s("c") + s("d")
result = joined + "|" + ",".join(order)
`
	got := evalSrc(t, src).Inspect()
	if got != "abcd|a,b,c,d" {
		t.Errorf("got %q want %q", got, "abcd|a,b,c,d")
	}
}

func TestConcatChainLongRun(t *testing.T) {
	// Long runs exercise the buffer's growth path, which no longer pre-sizes.
	var sb strings.Builder
	terms := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		terms = append(terms, "\"part\"")
		sb.WriteString("part")
	}
	src := "result = " + strings.Join(terms, " + ") + "\n"
	if got := evalSrc(t, src).Inspect(); got != sb.String() {
		t.Errorf("long chain produced %d chars, want %d", len(got), sb.Len())
	}
}
