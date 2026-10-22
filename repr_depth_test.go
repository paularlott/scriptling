package scriptling

import (
	"strconv"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

func newReprDepthScriptling() *Scriptling {
	p := New()
	stdlib.RegisterAll(p)
	return p
}

// Rendering a value nested far past the Go stack used to overflow it, a fatal
// error that ended the process. Every rendering path now raises a catchable
// RecursionError, as Python does, and the script carries on (CWE-674,
// vuln-0018).
func TestReprDeepValueRaisesRecursionError(t *testing.T) {
	p := newReprDepthScriptling()
	if _, err := p.Eval(`
deep_list = 1
for i in range(1000000):
    deep_list = [deep_list]
deep_tuple = 1
for i in range(1000000):
    deep_tuple = (deep_tuple,)
deep_dict = 1
for i in range(1000000):
    deep_dict = {"a": deep_dict}
`); err != nil {
		t.Fatalf("building deep values: %v", err)
	}
	for _, op := range []string{
		"str(deep_list)",
		"repr(deep_list)",
		"print(deep_list)",
		"f'{deep_list}'",
		"'{}'.format(deep_list)",
		"'%s' % (deep_list,)",
		"str([deep_list])",
		"str({'k': deep_list})",
		"str(deep_tuple)",
		"str(deep_dict)",
		"str({'k': deep_dict}.values())",
	} {
		result, err := p.Eval(`
try:
    ` + op + `
    r = "rendered"
except RecursionError as e:
    r = "RecursionError: " + str(e)
r`)
		if err != nil {
			t.Fatalf("%s: eval: %v", op, err)
		}
		if got := result.Inspect(); !strings.Contains(got, "RecursionError: maximum recursion depth exceeded") {
			t.Errorf("%s: expected a catchable RecursionError, got %s", op, got)
		}
	}
}

// The cap is exact: a value MaxReprDepth containers deep renders, one more
// raises.
func TestReprDepthBoundary(t *testing.T) {
	p := newReprDepthScriptling()
	build := func(n int) string {
		return "x = 1\nfor i in range(" + strconv.Itoa(n) + "):\n    x = [x]\n"
	}
	result, err := p.Eval(build(object.MaxReprDepth) + "len(repr(x))")
	if err != nil {
		t.Fatalf("a value at the cap should render: %v", err)
	}
	if got, want := result.Inspect(), strconv.Itoa(2*object.MaxReprDepth+1); got != want {
		t.Fatalf("repr length at the cap = %s, want %s", got, want)
	}
	if _, err := p.Eval(build(object.MaxReprDepth+1) + "repr(x)"); err == nil || !strings.Contains(err.Error(), "maximum recursion depth exceeded") {
		t.Fatalf("one container past the cap should raise RecursionError, got: %v", err)
	}
}

// Ordinary and self-referential values render as before (cycles keep their
// existing placeholder).
func TestReprShallowAndCyclicUnchanged(t *testing.T) {
	p := newReprDepthScriptling()
	result, err := p.Eval(`
a = [1]
a.append(a)
d = {}
d["self"] = d
[repr([1, (2, 3), {"k": [4]}, {5}]), repr(a), repr(d), f"{[[1]]}"]`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	want := `["[1, (2, 3), {'k': [4]}, {5}]", '[1, <cyclic reference>]', "{'self': <cyclic reference>}", '[[1]]']`
	if got := result.Inspect(); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

// Go-side Inspect() of a deep value cannot raise, but must not crash either.
func TestInspectDeepValueDoesNotCrash(t *testing.T) {
	p := newReprDepthScriptling()
	result, err := p.Eval("x = 1\nfor i in range(1000000):\n    x = [x]\nx")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got := result.Inspect(); !strings.HasPrefix(got, "[[[") {
		t.Fatalf("unexpected Inspect output prefix: %.20q", got)
	}
}
