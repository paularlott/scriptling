package evaluator

import (
	"context"
	"testing"
	"time"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

// evalCompat runs src with a timeout (so an eager consumer of an infinite
// iterator fails instead of hanging) and returns the final value.
func evalCompat(t *testing.T, src string) object.Object {
	t.Helper()
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors for %q: %v", src, errs)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return EvalWithContext(ctx, program, object.NewEnvironment())
}

// TestIntBaseKeyword: int() accepts base= as a keyword (Python's
// int(x, base=10)), plus base 0 prefix auto-detection and signed prefixes.
func TestIntBaseKeyword(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{`int("101", base=2)`, "5"},
		{`int("ff", base=16)`, "255"},
		{`int("0xff", base=16)`, "255"},
		{`int("z", base=36)`, "35"},
		{`int("101", 2)`, "5"},
		{`int("0x1f", 0)`, "31"},
		{`int("0o17", base=0)`, "15"},
		{`int("-0b11", 0)`, "-3"},
		{`int("42", base=0)`, "42"},
		{`int(" 42 ")`, "42"},
	}
	for _, tt := range tests {
		got := evalCompat(t, tt.src)
		if object.IsError(got) || got.Inspect() != tt.want {
			t.Errorf("%s = %s, want %s", tt.src, got.Inspect(), tt.want)
		}
	}
	for _, src := range []string{
		`int("101", 2, base=2)`,
		`int("101", bogus=2)`,
		`int(5, base=10)`,
		`int("012", 0)`,
		`int("12", base=1)`,
	} {
		if got := evalCompat(t, src); !object.IsError(got) && got.Type() != object.EXCEPTION_OBJ {
			t.Errorf("%s should raise, got %s", src, got.Inspect())
		}
	}
}

// TestEnumerateZipInfiniteIterator: enumerate() and zip() pull iterator
// inputs lazily, so an infinite iterator can be consumed with break or
// bounded by a shorter zip partner (they previously materialized it and
// never returned).
func TestEnumerateZipInfiniteIterator(t *testing.T) {
	const gen = `
class Naturals:
    def __init__(self):
        self.i = -1
    def __iter__(self):
        return self
    def __next__(self):
        self.i += 1
        return self.i
def naturals():
    return iter(Naturals())
`
	tests := []struct {
		src  string
		want string
	}{
		{gen + `
out = []
for i, x in enumerate(naturals(), 10):
    if i > 12:
        break
    out.append((i, x))
out`, "[(10, 0), (11, 1), (12, 2)]"},
		{gen + `list(zip("abc", naturals()))`, "[('a', 0), ('b', 1), ('c', 2)]"},
		{gen + `list(zip(naturals(), [7, 8]))`, "[(0, 7), (1, 8)]"},
		{gen + `
it = naturals()
e = enumerate(it)
[next(e), next(e), next(it)]`, "[(0, 0), (1, 1), 2]"},
		{`list(enumerate("ab"))`, "[(0, 'a'), (1, 'b')]"},
		{`list(zip([1, 2, 3], "ab"))`, "[(1, 'a'), (2, 'b')]"},
		{`list(zip())`, "[]"},
	}
	for _, tt := range tests {
		got := evalCompat(t, tt.src)
		if object.IsError(got) || got.Inspect() != tt.want {
			t.Errorf("%s\n got %s, want %s", tt.src, got.Inspect(), tt.want)
		}
	}
}

// TestEnumerateZipPropagateIteratorRaise: a raise inside a lazily consumed
// iterator still propagates out of enumerate()/zip().
func TestEnumerateZipPropagateIteratorRaise(t *testing.T) {
	for _, src := range []string{
		`
class Bad:
    def __init__(self):
        self.n = 0
    def __iter__(self):
        return self
    def __next__(self):
        self.n += 1
        if self.n > 1:
            raise ValueError("boom")
        return 1
list(enumerate(Bad()))`,
		`
class Bad:
    def __init__(self):
        self.n = 0
    def __iter__(self):
        return self
    def __next__(self):
        self.n += 1
        if self.n > 1:
            raise ValueError("boom")
        return 1
for a, b in zip(iter(Bad()), [1, 2, 3]):
    pass`,
		`
class It:
    def __iter__(self):
        return self
    def __next__(self):
        raise ValueError("boom")
list(zip(It()))`,
	} {
		got := evalCompat(t, src)
		if !object.IsError(got) && got.Type() != object.EXCEPTION_OBJ {
			t.Errorf("expected ValueError to propagate, got %s\n%s", got.Inspect(), src)
		}
	}
}
