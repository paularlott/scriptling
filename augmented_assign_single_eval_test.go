package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

// Augmented assignment to a subscript or attribute must evaluate the
// container and key expressions exactly once, then read, operate and write
// through those operands, matching Python. Each case counts how often the
// side-effecting sub-expressions run.
func TestAugmentedAssignEvaluatesTargetOnce(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   string // repr of `result`
	}{
		{"dict key", `
calls = 0
def key():
    global calls
    calls += 1
    return "a"
d = {"a": 1}
d[key()] += 5
result = [d["a"], calls]
`, "[6, 1]"},
		{"list index", `
calls = 0
def idx():
    global calls
    calls += 1
    return 1
xs = [10, 20, 30]
xs[idx()] *= 3
result = [xs, calls]
`, "[[10, 60, 30], 1]"},
		{"container expression", `
calls = 0
def box():
    global calls
    calls += 1
    return d
d = {"n": 1}
box()["n"] -= 4
result = [d["n"], calls]
`, "[-3, 1]"},
		{"property getter and setter once each", `
gets = 0
sets = 0
class P:
    def __init__(self):
        self._v = 2
    @property
    def v(self):
        global gets
        gets += 1
        return self._v
    @v.setter
    def v(self, value):
        global sets
        sets += 1
        self._v = value
p = P()
p.v += 3
result = [p._v, gets, sets]
`, "[5, 1, 1]"},
		{"getitem and setitem once each", `
log = []
class Box:
    def __init__(self):
        self.store = {"k": 7}
    def __getitem__(self, k):
        log.append("get")
        return self.store[k]
    def __setitem__(self, k, v):
        log.append("set")
        self.store[k] = v
b = Box()
b["k"] += 1
result = [b.store["k"], log]
`, "[8, ['get', 'set']]"},
		{"nested subscript", `
calls = []
def outer():
    calls.append("o")
    return "x"
def inner():
    calls.append("i")
    return "y"
d = {"x": {"y": 1}}
d[outer()][inner()] += 10
result = [d["x"]["y"], calls]
`, "[11, ['o', 'i']]"},
		{"in-place list extend", `
calls = 0
def key():
    global calls
    calls += 1
    return "l"
d = {"l": [1]}
alias = d["l"]
d[key()] += [2, 3]
result = [alias, calls]
`, "[[1, 2, 3], 1]"},
		{"in-place dict merge", `
calls = 0
def key():
    global calls
    calls += 1
    return "m"
d = {"m": {"a": 1}}
alias = d["m"]
d[key()] |= {"b": 2}
result = [sorted(alias.keys()), calls]
`, "[['a', 'b'], 1]"},
		{"value evaluated after target, once", `
order = []
def key():
    order.append("key")
    return "a"
def val():
    order.append("val")
    return 1
d = {"a": 0}
d[key()] += val()
result = [d["a"], order]
`, "[1, ['key', 'val']]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New()
			if _, err := p.Eval(tc.script); err != nil {
				t.Fatalf("eval: %v", err)
			}
			got, errObj := p.GetVarAsObject("result")
			if errObj != nil {
				t.Fatalf("result: %v", errObj)
			}
			if got.Inspect() != tc.want {
				t.Fatalf("got %s, want %s", got.Inspect(), tc.want)
			}
		})
	}
}

// A 2-D float array element target writes through to the array even though
// reading fa[i] yields a copy of the row, and the index expressions still run
// once each.
func TestAugmentedAssignFloatArray2D(t *testing.T) {
	p := New()
	fa := object.NewFloatArray2D([]float64{1, 2, 3, 4, 5, 6}, 2, 3)
	if err := p.SetObjectVar("fa", fa); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Eval(`
calls = []
def r():
    calls.append("r")
    return 1
def c():
    calls.append("c")
    return 2
fa[r()][c()] += 10
fa[0][0] -= 1
`); err != nil {
		t.Fatalf("eval: %v", err)
	}
	want := []float64{0, 2, 3, 4, 5, 16}
	for i, v := range want {
		if fa.Data[i] != v {
			t.Fatalf("fa.Data = %v, want %v", fa.Data, want)
		}
	}
	calls, _ := p.GetVarAsObject("calls")
	if calls.Inspect() != "['r', 'c']" {
		t.Fatalf("index expressions ran %s, want once each", calls.Inspect())
	}
}
