package stdlib_test

import (
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

func dequeInterp(opts ...scriptling.Option) *scriptling.Scriptling {
	p := scriptling.New(opts...)
	stdlib.RegisterAll(p)
	return p
}

// collections.deque grew a Go slice that was never checked against the
// value limit nor reported to the memory accounting, so a deque could hold
// hundreds of megabytes under an 8 MiB limit with the guard seeing nothing
// (CWE-770, strix 4250 vuln-0017). Every way of growing one is now held to
// both limits, like a list, and so is collecting an iterator for extend().
func TestDequeHeldToLimits(t *testing.T) {
	for name, script := range map[string]string{
		"append":      "dq = collections.deque()\nfor i in range(4000000):\n    dq.append(i)",
		"appendleft":  "dq = collections.deque()\nfor i in range(4000000):\n    dq.appendleft(i)",
		"extend":      "dq = collections.deque()\ndq.extend(range(4000000))",
		"extend list": "dq = collections.deque()\nfor i in range(100):\n    dq.extend([0] * 100000)",
		"extendleft":  "dq = collections.deque()\ndq.extendleft(range(4000000))",
		"from range":  "dq = collections.deque(range(4000000))",
		"from string": `dq = collections.deque("x" * 3000000)`,
		"copy":        "dq = collections.deque()\nfor i in range(400000):\n    dq.append(i)\nkeep = [dq.copy() for i in range(40)]",
	} {
		for _, opts := range [][]scriptling.Option{
			{scriptling.WithMemoryLimit(8 << 20), scriptling.WithMaxValueBytes(0)},
			{scriptling.WithMaxValueBytes(1 << 20)},
		} {
			p := dequeInterp(opts...)
			_, err := p.Eval("import collections\n" + script)
			if err == nil || !(strings.Contains(err.Error(), "memory limit exceeded") || strings.Contains(err.Error(), "too large")) {
				t.Errorf("%s: want a limit error, got %v", name, err)
			}
		}
	}

	// Within the limits, and with none, deques behave as before.
	p := dequeInterp(scriptling.WithMemoryLimit(8<<20), scriptling.WithMaxValueBytes(8<<20))
	result, err := p.Eval(`
import collections
d = collections.deque(range(1000), 10)
for i in range(100000):
    d.append(i)
    d.appendleft(i)
len(d)`)
	if err != nil || result.Inspect() != "10" {
		t.Fatalf("bounded deque under limits: %v %v", result, err)
	}
	p = dequeInterp()
	if result, err := p.Eval("import collections\nlen(collections.deque(range(2000000)))"); err != nil || result.Inspect() != "2000000" {
		t.Fatalf("no limits: %v %v", result, err)
	}
}

// appendleft and extendleft copied the whole deque on every element, so
// 40,000 appendlefts took seconds; both ends are now amortised O(1), as in
// Python.
func TestDequeEndsAreConstantTime(t *testing.T) {
	p := dequeInterp()
	start := time.Now()
	if _, err := p.Eval(`
import collections
d = collections.deque()
for i in range(200000):
    d.appendleft(i)
d.extendleft(range(200000))
for i in range(300000):
    d.popleft()
    d.append(i)
d.rotate(123457)
d.rotate(-98765)
`); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("deque operations took %v", elapsed)
	}
}

// Copies never share storage with the original, deepcopy keeps cycles, and
// a negative maxlen is a ValueError as in Python.
func TestDequeCopiesAndMaxlen(t *testing.T) {
	p := dequeInterp()
	result, err := p.Eval(`
import collections, copy
d = collections.deque([1, [2]], 4)
a = d.copy()
b = copy.copy(d)
c = copy.deepcopy(d)
d.append(3)
d[1].append(9)
e = collections.deque([1])
e.append(e)
f = copy.deepcopy(e)
try:
    collections.deque([], -1)
    neg = "accepted"
except ValueError as err:
    neg = "ValueError"
[list(a), list(b), list(c), a.maxlen, c.maxlen, f[1] is f, f[1] is e, neg, str(collections.deque(["x", 1], 3))]`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[[1, [2, 9]], [1, [2, 9]], [1, [2]], 4, 4, True, False, 'ValueError', "deque(['x', 1], maxlen=3)"]`
	if got := result.Inspect(); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}
