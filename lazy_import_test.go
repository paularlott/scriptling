package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// PEP 810 "lazy import" is accepted and treated as an eager import.

func lazyEvalInt(t *testing.T, p *Scriptling, script string) int64 {
	t.Helper()
	if _, err := p.Eval(script); err != nil {
		t.Fatalf("Eval: %v\n%s", err, script)
	}
	v, objErr := p.GetVarAsInt("result")
	if objErr != nil {
		t.Fatalf("result: %v", objErr)
	}
	return v
}

func TestLazyImportForms(t *testing.T) {
	cases := map[string]string{
		"import":        "lazy import math\nresult = int(math.sqrt(16))",
		"import as":     "lazy import math as m\nresult = int(m.sqrt(16))",
		"import list":   "lazy import math, json\nresult = int(math.sqrt(16)) + len(json.dumps([]))",
		"from":          "lazy from math import sqrt\nresult = int(sqrt(16))",
		"from as":       "lazy from math import sqrt as root\nresult = int(root(16))",
		"in function":   "def f():\n    lazy import math\n    return int(math.sqrt(16))\nresult = f()",
		"script lib":    "lazy import helper\nresult = helper.four()",
		"from scriptlb": "lazy from helper import four\nresult = four()",
	}
	want := map[string]int64{"import list": 6}
	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			p.RegisterScriptLibrary("helper", "def four():\n    return 4\n")
			expected := int64(4)
			if w, ok := want[name]; ok {
				expected = w
			}
			if got := lazyEvalInt(t, p, script); got != expected {
				t.Fatalf("result = %d, want %d", got, expected)
			}
		})
	}
}

// "lazy" is a soft keyword: it stays an ordinary name everywhere else.
func TestLazyIsStillAnIdentifier(t *testing.T) {
	cases := map[string]string{
		"assign":    "lazy = 4\nresult = lazy",
		"augmented": "lazy = 3\nlazy += 1\nresult = lazy",
		"call":      "def lazy(x):\n    return x * 2\nresult = lazy(2)",
		"subscript": "lazy = [4]\nresult = lazy[0]",
		"attribute": "class C:\n    v = 4\nlazy = C()\nresult = lazy.v",
		"unpack":    "lazy, other = 4, 0\nresult = lazy + other",
		"param":     "def f(lazy):\n    return lazy\nresult = f(4)",
		"kwarg":     "def f(lazy=0):\n    return lazy\nresult = f(lazy=4)",
	}
	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			if got := lazyEvalInt(t, New(), script); got != 4 {
				t.Fatalf("result = %d, want 4", got)
			}
		})
	}
}

// The hint is ignored, so a missing module fails at the import statement,
// where it can be caught, not at first use.
func TestLazyImportFailsEagerly(t *testing.T) {
	p := New()
	script := "result = 0\ntry:\n    lazy import no_such_module\nexcept ImportError:\n    result = 4\n"
	if got := lazyEvalInt(t, p, script); got != 4 {
		t.Fatalf("result = %d, want 4 (ImportError caught at import site)", got)
	}

	_, err := New().Eval("x = 1\nlazy from no_such_module import thing\n")
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("want import error at line 2, got %v", err)
	}
}

func TestLazyOnOwnLineIsAName(t *testing.T) {
	script := "result = 0\ntry:\n    lazy\n    import math\nexcept NameError:\n    result = 4\n"
	if got := lazyEvalInt(t, New(), script); got != 4 {
		t.Fatalf("result = %d, want 4 (bare lazy raises NameError)", got)
	}
}
