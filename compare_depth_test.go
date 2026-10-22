package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

func newCompareScriptling(t *testing.T) *Scriptling {
	t.Helper()
	p := New()
	stdlib.RegisterAll(p)
	if _, err := p.Eval(`
import collections
n = float("nan")
x = [1]
x.append(x)
y = [1]
y.append(y)
d = {}
d["a"] = d
e = {}
e["a"] = e
P = collections.namedtuple("P", ["a", "b"])

def outcome(f):
    try:
        return repr(f())
    except RecursionError:
        return "RecursionError"
    except TypeError:
        return "TypeError"
    except ValueError:
        return "ValueError"
`); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return p
}

func expectOutcomes(t *testing.T, p *Scriptling, cases map[string]string) {
	t.Helper()
	for expr, want := range cases {
		result, err := p.Eval(`outcome(lambda: ` + expr + `)`)
		if err != nil {
			t.Errorf("%s: eval error %v", expr, err)
			continue
		}
		if got := result.Inspect(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// Comparing a container that holds itself recursed until the Go stack
// overflowed, a fatal error that ended the whole process (CWE-674, strix
// 4250 vuln-0014). Elements that are the same object now compare equal
// without recursing, and distinct cyclic values raise a catchable
// RecursionError. Every expectation is CPython 3.14's.
func TestCompareCyclicMatchesPython(t *testing.T) {
	expectOutcomes(t, newCompareScriptling(t), map[string]string{
		`x == x`:                   "True",
		`x != x`:                   "False",
		`x < x`:                    "False",
		`x <= x`:                   "True",
		`sorted([x, x]) == [x, x]`: "True",
		`max([x, x]) is x`:         "True",
		`min(x, x) is x`:           "True",
		`x in [x]`:                 "True",
		`[x, x].count(x)`:          "2",
		`[1, x].index(x)`:          "1",
		`(1, x).index(x)`:          "1",
		`d == d`:                   "True",
		`[d] == [d]`:               "True",
		`(x, d) == (x, d)`:         "True",
		`x == y`:                   "RecursionError",
		`x != y`:                   "RecursionError",
		`x < y`:                    "RecursionError",
		`d == e`:                   "RecursionError",
		`[1, x].index(y)`:          "RecursionError",
		`[x].count(y)`:             "RecursionError",
		`y in [x]`:                 "RecursionError",
		`sorted([x, y])`:           "RecursionError",
		`max(x, y)`:                "RecursionError",
		`x == [1, 2]`:              "False",
		`[0, x] < [1, y]`:          "True",
	})
}

// A value nested far past the cap, built in a loop, raises RecursionError
// from every comparison path instead of overflowing the stack; one within
// the cap compares as before.
func TestCompareDeepRaisesRecursionError(t *testing.T) {
	p := newCompareScriptling(t)
	if _, err := p.Eval(`
def nest(k, wrap):
    v = 1
    for i in range(k):
        v = wrap(v)
    return v
a = nest(1000000, lambda v: [v])
b = nest(1000000, lambda v: [v])
ta = nest(1000000, lambda v: (v,))
tb = nest(1000000, lambda v: (v,))
da = nest(1000000, lambda v: {"k": v})
db = nest(1000000, lambda v: {"k": v})
pa = P(1, nest(1000000, lambda v: (v,)))
pb = P(1, nest(1000000, lambda v: (v,)))
sa = nest(9000, lambda v: [v])
sb = nest(9000, lambda v: [v])
`); err != nil {
		t.Fatalf("building deep values: %v", err)
	}
	expectOutcomes(t, p, map[string]string{
		`a == b`:                       "RecursionError",
		`a != b`:                       "RecursionError",
		`a < b`:                        "RecursionError",
		`a >= b`:                       "RecursionError",
		`ta == tb`:                     "RecursionError",
		`ta < tb`:                      "RecursionError",
		`da == db`:                     "RecursionError",
		`a in [b]`:                     "RecursionError",
		`[b].count(a)`:                 "RecursionError",
		`[b].index(a)`:                 "RecursionError",
		`sorted([a, b])`:               "RecursionError",
		`max([a, b])`:                  "RecursionError",
		`min(a, b)`:                    "RecursionError",
		`pa < pb`:                      "RecursionError",
		`a == a`:                       "True",
		`sa == sb`:                     "True",
		`sa < sb`:                      "False",
		`sorted([sa, sb]) == [sa, sb]`: "True",
	})
}

// Elements that are the same object match in container equality and
// searches, as in Python, so a NaN held twice compares equal inside a
// container while NaN == NaN stays False.
func TestCompareIdentityNaN(t *testing.T) {
	expectOutcomes(t, newCompareScriptling(t), map[string]string{
		`n == n`:                           "False",
		`[n] == [n]`:                       "True",
		`(n,) == (n,)`:                     "True",
		`{"a": n} == {"a": n}`:             "True",
		`[float("nan")] == [float("nan")]`: "False",
		`n in [n]`:                         "True",
		`[n].count(n)`:                     "1",
		`[n].index(n)`:                     "0",
		`(n,).count(n)`:                    "1",
		`(1, n).index(n)`:                  "1",
		`[n, 1] < [n, 2]`:                  "True",
	})
	p := newCompareScriptling(t)
	result, err := p.Eval(`
l = [n, 1]
l.remove(n)
l`)
	if err != nil || result.Inspect() != "[1]" {
		t.Fatalf("[n, 1].remove(n) -> %v, %v", result, err)
	}
}

// min() and max() order values as sorted() does, so incomparable values
// raise TypeError as in Python instead of silently picking one.
func TestMinMaxIncomparableRaises(t *testing.T) {
	expectOutcomes(t, newCompareScriptling(t), map[string]string{
		`max([1, "a"])`:                    "TypeError",
		`min([[1, "a"], [1, 2]])`:          "TypeError",
		`max([None, None])`:                "TypeError",
		`max([[1], (1,)])`:                 "TypeError",
		`max([(1, 2), (1, 3)])`:            "(1, 3)",
		`min([[2], [1, 5]])`:               "[1, 5]",
		`max(1, 2.5, True)`:                "2.5",
		`min("b", "a")`:                    "'a'",
		`max([None])`:                      "None",
		`min([3, 1, 2], key=lambda v: -v)`: "3",
	})
}
