package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// Parenthesized "from m import (a, b)" lets the name list span lines.

func TestFromImportParenthesized(t *testing.T) {
	cases := map[string]string{
		"single line":    "from math import (sqrt, floor)\nresult = int(sqrt(16)) + floor(0.5)",
		"single name":    "from math import (sqrt)\nresult = int(sqrt(16))",
		"multi line":     "from math import (\n    sqrt,\n    floor\n)\nresult = int(sqrt(16)) + floor(0.5)",
		"trailing comma": "from math import (\n    sqrt,\n    floor,\n)\nresult = int(sqrt(16)) + floor(0.5)",
		"aliases":        "from math import (\n    sqrt as root,\n    floor as fl,\n)\nresult = int(root(16)) + fl(0.5)",
		"odd layout":     "from math import (sqrt,\n                  floor)\nresult = int(sqrt(16)) + floor(0.5)",
		"in function":    "def f():\n    from math import (\n        sqrt,\n        floor,\n    )\n    x = int(sqrt(16))\n    return x + floor(0.5)\nresult = f()",
		"in if block":    "if True:\n    from math import (\n        sqrt,\n    )\n    result = int(sqrt(16))",
		"lazy":           "lazy from math import (\n    sqrt,\n)\nresult = int(sqrt(16))",
		"script lib":     "from helper import (\n    four,\n    zero,\n)\nresult = four() + zero()",
	}
	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			p.RegisterScriptLibrary("helper", "def four():\n    return 4\n\ndef zero():\n    return 0\n")
			if got := lazyEvalInt(t, p, script); got != 4 {
				t.Fatalf("result = %d, want 4", got)
			}
		})
	}
}

func TestFromImportParenthesizedRelative(t *testing.T) {
	p := New()
	p.RegisterScriptLibrary("pkg.a", "ONE = 1\nTWO = 2\n")
	p.RegisterScriptLibrary("pkg.b", "from .a import (\n    ONE,\n    TWO,\n)\nTOTAL = ONE + TWO + 1\n")
	if got := lazyEvalInt(t, p, "import pkg.b\nresult = pkg.b.TOTAL"); got != 4 {
		t.Fatalf("result = %d, want 4", got)
	}
}

// Forms CPython rejects stay syntax errors.
func TestFromImportParenthesizedErrors(t *testing.T) {
	bad := map[string]string{
		"empty":            "from math import ()",
		"unclosed":         "from math import (sqrt, floor\nx = 1",
		"double comma":     "from math import (sqrt,, floor)",
		"leading comma":    "from math import (, sqrt)",
		"bare trailing":    "from math import sqrt,",
		"only comma":       "from math import (,)",
		"missing names":    "from math import",
		"stray close":      "from math import sqrt)",
		"alias no name":    "from math import (sqrt as)",
		"nested parens":    "from math import ((sqrt))",
		"close then names": "from math import (sqrt) floor",
	}
	for name, script := range bad {
		t.Run(name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			if _, err := p.Eval(script); err == nil {
				t.Fatalf("expected a syntax error for %q", script)
			}
		})
	}
}
