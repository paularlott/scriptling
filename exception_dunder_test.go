package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// A raise from a comparison/containment/iteration/formatting/hash dunder must
// propagate to the nearest except rather than being swallowed (returning
// garbage) or spliced into output.
func TestDunderRaisesPropagate(t *testing.T) {
	cases := map[string]string{
		"sorted __lt__": `
class Bad:
    def __lt__(self, other):
        raise ValueError("boom")
sorted([Bad(), Bad()])`,
		"min __lt__": `
class Bad:
    def __lt__(self, other):
        raise ValueError("boom")
min(Bad(), Bad())`,
		"max __lt__": `
class Bad:
    def __lt__(self, other):
        raise ValueError("boom")
max(Bad(), Bad())`,
		"in __contains__": `
class C:
    def __contains__(self, item):
        raise ValueError("boom")
_ = 1 in C()`,
		"for __iter__": `
class C:
    def __iter__(self):
        raise ValueError("boom")
for x in C():
    pass`,
		"fstring __str__": `
class C:
    def __str__(self):
        raise ValueError("boom")
_ = f"{C()}"`,
		"print __str__": `
class C:
    def __str__(self):
        raise ValueError("boom")
print(C())`,
		"dict literal __hash__": `
class C:
    def __hash__(self):
        raise ValueError("boom")
d = {C(): 1}`,
		"set add __hash__": `
class C:
    def __hash__(self):
        raise ValueError("boom")
s = {C()}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			got := evalString(t, `
caught = "no"
try:
`+indent(body)+`
except ValueError:
    caught = "yes"
caught
`)
			if got != "yes" {
				t.Fatalf("%s: got %q, want yes (dunder raise swallowed)", name, got)
			}
		})
	}
}

// indent prefixes each non-empty line of a script body with four spaces so it
// nests correctly inside a try block.
func indent(s string) string {
	out := ""
	for _, line := range splitLines(s) {
		if line == "" {
			out += "\n"
			continue
		}
		out += "    " + line + "\n"
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	lines = append(lines, cur)
	return lines
}

// min/max honor a user __lt__ ordering rather than ignoring it.
func TestMinMaxHonorLt(t *testing.T) {
	got := evalString(t, `
class Ord:
    def __init__(self, v):
        self.v = v
    def __lt__(self, other):
        return self.v < other.v
    def __eq__(self, other):
        return self.v == other.v
"%d|%d" % (min(Ord(5), Ord(2), Ord(8)).v, max(Ord(5), Ord(2), Ord(8)).v)
`)
	if got != "2|8" {
		t.Fatalf("got %q, want %q", got, "2|8")
	}
}

// print() honors a user __str__ (not just the default repr).
func TestPrintHonorsStr(t *testing.T) {
	p := New()
	var out []byte
	p.SetOutputWriter(writerFunc(func(b []byte) (int, error) { out = append(out, b...); return len(b), nil }))
	if _, err := p.Eval(`
class C:
    def __str__(self):
        return "custom"
print(C())
`); err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if string(out) != "custom\n" {
		t.Fatalf("print output = %q, want %q", string(out), "custom\n")
	}
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(b []byte) (int, error) { return w(b) }

// Exceptions are hashable in a set (consistent with dict keys).
func TestExceptionHashableInSet(t *testing.T) {
	got := evalString(t, `
s = {ValueError("v")}
len(s)
`)
	if got != "1" {
		t.Fatalf("got %q, want 1", got)
	}
}

// itertools higher-order functions accept script-defined predicates and
// propagate their raises.
func TestItertoolsAcceptsUserFunctions(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.ItertoolsLibrary)
	result, err := p.Eval(`
import itertools
r1 = itertools.takewhile(lambda x: x < 3, [1, 2, 3, 4])
r2 = itertools.dropwhile(lambda x: x < 3, [1, 2, 3, 4])
r3 = itertools.filterfalse(lambda x: x % 2, [1, 2, 3, 4])
r4 = itertools.starmap(lambda a, b: a + b, [(1, 2), (3, 4)])
"%s|%s|%s|%s" % (str(r1), str(r2), str(r3), str(r4))
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	want := "[1, 2]|[3, 4]|[2, 4]|[3, 7]"
	if got := result.Inspect(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestItertoolsPropagatesPredicateRaise(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.ItertoolsLibrary)
	_, err := p.Eval(`
import itertools
def boom(x):
    raise ValueError("pred")
itertools.takewhile(boom, [1, 2])
`)
	if err == nil {
		t.Fatal("expected the predicate raise to propagate")
	}
}
