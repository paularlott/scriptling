package evaluator

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

// String literals share one immutable *object.String across every evaluation of
// the same AST node (see evalStringLiteral). That is only sound because strings
// are immutable in the language, so these tests cover the ways a shared value
// could otherwise leak between uses.

func evalLit(t *testing.T, src string) (object.Object, *object.Environment) {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors for %q: %v", src, errs)
	}
	env := object.NewEnvironment()
	return EvalWithContext(context.Background(), program, env), env
}

func litResult(t *testing.T, src string) string {
	t.Helper()
	out, env := evalLit(t, src)
	if object.IsError(out) {
		t.Fatalf("unexpected error for %q: %s", src, out.Inspect())
	}
	val, ok := env.Get("result")
	if !ok {
		t.Fatalf("script did not bind `result`: %q", src)
	}
	return val.Inspect()
}

func TestStringLiteralReuseDoesNotLeakBetweenUses(t *testing.T) {
	// A literal evaluated many times, with results kept and mutated around, must
	// not observe changes through the shared value.
	cases := []struct{ name, src, want string }{
		{"concatenation does not mutate the literal", `
acc = ""
for i in range(3):
    acc = acc + "x"
result = acc + "|" + "x"
`, "xxx|x"},
		{"upper does not mutate the literal", `
a = "abc"
b = a.upper()
result = a + "|" + b + "|" + "abc"
`, "abc|ABC|abc"},
		{"replace does not mutate the literal", `
a = "hello"
b = a.replace("l", "L")
result = a + "|" + b
`, "hello|heLLo"},
		{"same literal in a loop keeps its value", `
out = ""
for i in range(3):
    s = "ab"
    out = out + s
result = out
`, "ababab"},
		{"literal used as dict key and value", `
d = {}
d["k"] = "k"
result = d["k"] + "|" + "k"
`, "k|k"},
		{"slicing a literal", `
a = "abcdef"
result = a[1:3] + "|" + "abcdef"
`, "bc|abcdef"},
		{"literal reused across function calls", `
def f(s):
    return s + "!"
result = f("hi") + "|" + f("hi") + "|" + "hi"
`, "hi!|hi!|hi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := litResult(t, c.src); got != c.want {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestStringLiteralAttributeAccessUnaffected(t *testing.T) {
	// Attribute access desugars to indexing by a string literal, which is the
	// main reason the cache exists. Reading and writing many fields must still
	// resolve to the right ones.
	src := `
class P:
    def __init__(self, x, y):
        self.x = x
        self.y = y
    def swap(self):
        t = self.x
        self.x = self.y
        self.y = t

p = P("a", "b")
p.swap()
result = p.x + p.y
`
	if got := litResult(t, src); got != "ba" {
		t.Errorf("got %q want %q", got, "ba")
	}
}

func TestStringLiteralBoxedIsStableAndShared(t *testing.T) {
	// The same AST node must hand back one pointer, and it must carry the right
	// value. Two distinct literals must not collide.
	src := "a = \"alpha\"\nb = \"beta\"\n"
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	litOf := func(i int) *ast.StringLiteral {
		t.Helper()
		assign, ok := program.Statements[i].(*ast.AssignStatement)
		if !ok {
			t.Fatalf("statement %d: expected assignment, got %T", i, program.Statements[i])
		}
		lit, ok := assign.Value.(*ast.StringLiteral)
		if !ok {
			t.Fatalf("statement %d: expected string literal, got %T", i, assign.Value)
		}
		return lit
	}

	alpha, beta := litOf(0), litOf(1)
	if alpha.Boxed() != nil || beta.Boxed() != nil {
		t.Fatal("literals should start with no cached value")
	}

	first := evalStringLiteral(alpha)
	second := evalStringLiteral(alpha)
	if first != second {
		t.Error("evaluating the same literal twice returned different pointers")
	}
	if s, ok := first.(*object.String); !ok || s.StringValue() != "alpha" {
		t.Errorf("cached value = %v, want the string %q", first.Inspect(), "alpha")
	}

	other := evalStringLiteral(beta)
	if other == first {
		t.Error("two different literals shared one cached value")
	}
	if s, ok := other.(*object.String); !ok || s.StringValue() != "beta" {
		t.Errorf("cached value = %v, want the string %q", other.Inspect(), "beta")
	}
}

func TestStringLiteralConcurrentEvaluationIsSafe(t *testing.T) {
	// Literals are shared between goroutines through the program cache, so racing
	// evaluations of one node must be safe. Run with -race for this to mean
	// anything.
	src := "result = \"shared\"\n"
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	assign := program.Statements[0].(*ast.AssignStatement)
	lit := assign.Value.(*ast.StringLiteral)

	const goroutines = 8
	done := make(chan object.Object, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() { done <- evalStringLiteral(lit) }()
	}
	for i := 0; i < goroutines; i++ {
		got := <-done
		if s, ok := got.(*object.String); !ok || s.StringValue() != "shared" {
			t.Errorf("concurrent evaluation produced %v", got.Inspect())
		}
	}
}
