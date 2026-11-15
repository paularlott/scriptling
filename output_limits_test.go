package scriptling_test

import (
	"context"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/toml"
	"github.com/paularlott/scriptling/extlibs/yaml"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// Output built from a value with shared references: a = [0]*1000,
// b = [a]*1000, c = [b]*N holds a few thousand slots, but renders to N MB.
// Under an 8 MB value limit and a 256 MB memory limit (the sandbox an
// embedder runs model-written scripts in), every conversion must stop as
// soon as its output passes the limit, allocating a small multiple of it,
// rather than measuring the finished output.

const (
	sandboxMemoryLimit = 256 << 20
	sandboxValueLimit  = 8 << 20
	// sandboxAllocBudget is the most one refused conversion may allocate:
	// a few times the value limit, for buffer growth and copies; the Go value
	// a TOML conversion builds is bounded by the memory left as well.
	sandboxAllocBudget = 128 << 20
)

const sharedRefsPrelude = `a = [0] * 1000
b = [a] * 1000
c = [b] * 100
`

func sandboxInterp() *scriptling.Scriptling {
	p := scriptling.New(scriptling.WithMemoryLimit(sandboxMemoryLimit), scriptling.WithMaxValueBytes(sandboxValueLimit))
	stdlib.RegisterAll(p)
	yaml.Register(p)
	toml.Register(p)
	p.SetOutputWriter(io.Discard)
	return p
}

func totalAlloc() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.TotalAlloc
}

// expectBoundedRefusal runs prelude, then measures script: it must fail
// with a MemoryError, within the allocation budget and promptly.
func expectBoundedRefusal(t *testing.T, prelude, script string) {
	t.Helper()
	name := strings.ReplaceAll(script, "\n", "; ")
	p := sandboxInterp()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := p.EvalWithContext(ctx, prelude); err != nil {
		t.Fatalf("prelude: %v", err)
	}
	runtime.GC()
	before := totalAlloc()
	start := time.Now()
	result, err := p.EvalWithContext(ctx, script)
	elapsed := time.Since(start)
	allocated := totalAlloc() - before
	msg := ""
	if err != nil {
		msg = err.Error()
	} else if e, ok := result.(*object.Error); ok {
		msg = e.Message
	} else if ex, ok := result.(*object.Exception); ok {
		msg = ex.Inspect()
	} else {
		t.Fatalf("%s: expected a MemoryError, got a %s after allocating %d MB", name, result.Type(), allocated>>20)
	}
	if !strings.Contains(msg, "MemoryError") && !strings.Contains(msg, "too large") && !strings.Contains(msg, "memory limit") {
		t.Fatalf("%s: expected a MemoryError, got %q", name, msg)
	}
	if allocated > sandboxAllocBudget {
		t.Fatalf("%s: allocated %d MB before refusing (budget %d MB): %s", name, allocated>>20, sandboxAllocBudget>>20, msg)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("%s: took %v to refuse", name, elapsed)
	}
	t.Logf("%-40s refused after %4d MB, %v: %s", name, allocated>>20, elapsed.Round(time.Millisecond), msg)
}

func TestOutputLimitSharedReferences(t *testing.T) {
	for _, script := range []string{
		`print(c)`,
		`str(c)`,
		`repr(c)`,
		`f"{c}"`,
		`f"{c!r}"`,
		`"{}".format(c)`,
		`"%s" % (c,)`,
		`"%r" % (c,)`,
		`format(c)`,
		`print(b, b, b, b, b, b, b, b, b, b)`,
		`print(*([b] * 50))`,
		`print(*([b] * 50), sep="")`,
		`str((c,))`,
		`str({1: c})`,
		`str({"k": b, "j": b, "l": b, "m": b, "n": b})`,
		`str({tuple([tuple(a)] * 3000)})`,
		`str(c[0:50])`,
		`str({"x": c}.values())`,
		`",".join([str(b)] * 10)`,
		`import json
json.dumps(c)`,
		`import json
json.dumps(c, indent=2)`,
		`import json
json.dumps({"k": c}, sort_keys=True)`,
		`import yaml
yaml.dump(c)`,
		`import yaml
yaml.dump({"k": c, "z": [c]})`,
		`import toml
toml.dumps({"k": c})`,
		`import io
s = io.StringIO()
print(c, file=s)`,
		`import collections
str(collections.deque([b] * 50))`,
		`import collections
str(collections.Counter({"k": 1}) and [b] * 50)`,
		`import collections
P = collections.namedtuple("P", ["x", "y"])
repr(P(c, c))`,
		`from dataclasses import dataclass
@dataclass
class D:
    x: list
repr(D(c))`,
		`import copy
str(copy.deepcopy(c))`,
		`import collections
str(collections.Counter({tuple([tuple(a)] * 3000): 1}))`,
		`import yaml
yaml.dump({"x" * 200: c})`,
		`import re
re.sub("a", "x" * 1000000, "a" * 1000)`,
		`import re
re.sub("(a)", "\\1" * 100000, "a" * 1000, count=500)`,
		`import csv, io
s = io.StringIO()
csv.writer(s).writerows([a] * 10000)`,
		`import msgpack
msgpack.packb(c)`,
		`import collections
P = collections.namedtuple("P", ["x"])
P(1)._replace(x=c).__repr__()`,
	} {
		t.Run(script, func(t *testing.T) { expectBoundedRefusal(t, sharedRefsPrelude, script) })
	}
}

// A chain of doubling references renders to 2**60 elements: without a bound
// on output the work, not only the memory, is unbounded.
func TestOutputLimitExponentialDAG(t *testing.T) {
	prelude := `x = [0]
for i in range(60):
    x = [x, x]
`
	for _, script := range []string{
		`str(x)`, `repr(x)`, `print(x)`, `f"{x}"`,
		`import json
json.dumps(x)`,
		`import yaml
yaml.dump(x)`,
		`import toml
toml.dumps({"x": x})`,
	} {
		t.Run(script, func(t *testing.T) { expectBoundedRefusal(t, prelude, script) })
	}
}

// Within the limits output is unchanged.
func TestOutputLimitAllowsOutputWithinLimit(t *testing.T) {
	p := sandboxInterp()
	result, err := p.Eval(`import json
a = [0] * 1000
b = [a] * 100
s = str(b)
j = json.dumps(b)
len(s) == 100 * len(str(a)) + 2 * 99 + 2 and len(j) == len(s)
`)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := result.AsBool(); !ok {
		t.Fatalf("output within the limit changed: %s", result.Inspect())
	}
}

// Tuples built from shared tuples key dicts and sets by a digest once their
// spelled-out key grows long, and each distinct tuple is keyed once, so a
// tuple standing for 2**60 elements keys in microseconds, with no limits set.
func TestSharedTupleDictKeysAreBounded(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	before := totalAlloc()
	result, err := p.EvalWithContext(ctx, `x = (0,)
y = (0,)
for i in range(60):
    x = (x, x)
    y = (y, y)
d = {x: 1}
s = {x, (1, 2)}
[d[y], y in s, (1, 2) in s, len({x: 1, y: 2})]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[1, True, True, 1]" {
		t.Fatalf("got %s", got)
	}
	if allocated := totalAlloc() - before; allocated > 64<<20 {
		t.Fatalf("keying shared tuples allocated %d MB", allocated>>20)
	}
}

// Short tuples keep their spelled-out keys.
func TestTupleDictKeyShortForm(t *testing.T) {
	if got := object.DictKey(&object.Tuple{Elements: []object.Object{object.NewInteger(1), object.NewString("a")}}); got != "t:(n:1,a)" {
		t.Fatalf("got %q", got)
	}
}

// Indentation grows with depth as well as size: a deeply nested value
// indents to many times its compact length.
func TestOutputLimitDeepIndent(t *testing.T) {
	prelude := `x = 1
for i in range(5000):
    x = [x]
d = 1
for i in range(5000):
    d = {"k": d}
`
	for _, script := range []string{
		`import json
json.dumps(x, indent="xxxx")`,
		`import json
json.dumps(d, indent=8)`,
		`import yaml
yaml.dump(d)`,
	} {
		t.Run(script, func(t *testing.T) { expectBoundedRefusal(t, prelude, script) })
	}
}

// Under a value limit, Inspect (which cannot fail) stops just past the limit.
func TestInspectBoundedUnderLimit(t *testing.T) {
	p := sandboxInterp()
	result, err := p.Eval(sharedRefsPrelude + "c")
	if err != nil {
		t.Fatal(err)
	}
	release := object.BindGuard(p.ResourceGuard())
	defer release()
	before := totalAlloc()
	s := result.Inspect()
	if allocated := totalAlloc() - before; allocated > sandboxAllocBudget {
		t.Fatalf("Inspect allocated %d MB", allocated>>20)
	}
	if len(s) <= sandboxValueLimit || len(s) > 2*sandboxValueLimit || !strings.HasSuffix(s, "...") {
		t.Fatalf("Inspect gave %d bytes, want just past the %d limit ending in ...", len(s), sandboxValueLimit)
	}
}

// Breaking a long word decodes only the piece broken off: it used to decode
// the rest of the word for every piece, quadratic in the word's length.
func TestTextwrapLongWordLinear(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	start := time.Now()
	result, err := p.Eval(`import textwrap
len(textwrap.wrap("x" * 300000, width=3))`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Inspect() != "100000" {
		t.Fatalf("got %s", result.Inspect())
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("wrapping a long word took %v", elapsed)
	}
}
