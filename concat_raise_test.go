package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// An exception raised inside a `+` chain such as `name + ": " + f()` was
// folded into the chain as an operand, so the caller saw "TypeError:
// unsupported operand type(s) for +: 'str' and 'ValueError'" and
// `except ValueError` missed it. The chain now stops and raises what the
// operand raised, wherever it sits. Expectations are CPython 3.14's.
func TestConcatChainPropagatesRaise(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)
	if _, err := p.Eval(`
def boom():
    raise ValueError("bad input")

def outcome(f):
    try:
        return repr(f())
    except ValueError as e:
        return "ValueError: " + str(e)
    except TypeError as e:
        return "TypeError: " + str(e)
    except RecursionError:
        return "RecursionError"

name = "x"
e = ValueError("v")
deep = []
for i in range(100000):
    deep = [deep]
`); err != nil {
		t.Fatalf("setup: %v", err)
	}
	cases := map[string]string{
		`name + boom() + "!"`:            "ValueError: bad input",
		`name + ": " + boom()`:           "ValueError: bad input",
		`boom() + name + "!"`:            "ValueError: bad input",
		`"a" + "b" + boom()`:             "ValueError: bad input",
		`1 + 2 + boom()`:                 "ValueError: bad input",
		`name + ": " + str(int("q"))`:    "ValueError: invalid literal for int() with base 10: 'q'",
		`name + ": " + str([1] < ["a"])`: "TypeError: '<' not supported between instances of 'int' and 'str'",
		`name + ": " + repr(deep)`:       "RecursionError",
		`name + "b" + str(e)`:            "'xbv'",
		`name + "a" + "b"`:               "'xab'",
		`[1] + [2] + list(range(2))`:     "[1, 2, 0, 1]",
		`1 + 2 + int("3")`:               "6",
	}
	for expr, want := range cases {
		result, err := p.Eval(`outcome(lambda: ` + expr + `)`)
		if err != nil {
			t.Errorf("%s: eval error %v", expr, err)
			continue
		}
		if got := result.Inspect(); got != "'"+want+"'" && got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}

	// An exception value that is not being raised is still an ordinary
	// operand: adding it to a string is a TypeError, as in Python.
	for _, expr := range []string{`name + "b" + e`, `e + name + "b"`} {
		result, err := p.Eval(`outcome(lambda: ` + expr + `)`)
		if err != nil {
			t.Fatalf("%s: eval error %v", expr, err)
		}
		if got := result.Inspect(); !strings.HasPrefix(got, "TypeError: unsupported operand") {
			t.Errorf("%s = %s, want a TypeError", expr, got)
		}
	}
}
