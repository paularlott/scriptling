package scriptling

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/stdlib"
)

// chainSetup defines what the chain steps below use.
const chainSetup = `
import functools
import itertools
import random

class N:
    def __init__(self, p):
        self.p = p
    def get(self):
        return self.p

def g(p):
    yield p
`

// chainSteps each link x to a new object holding the previous one. No counted
// container holds the chain, so only the objects' own accounting can see it.
var chainSteps = map[string]string{
	"instance":     "x = N(x)",
	"tuple":        "x = (x, i)",
	"exception":    "x = Exception(x)",
	"def":          "def h(p=x):\n        return p\n    x = h",
	"lambda":       "x = lambda p=x: p",
	"class":        "class C:\n        prev = x\n    x = C",
	"generator":    "x = g(x)",
	"bound method": "x = N(x).get",
	"filter":       "x = filter(None, [x])",
	"reversed":     "x = reversed([x])",
	"iterator":     "x = iter((x,))",
	"dict view":    "x = {1: x}.values()",
	"partial":      "x = functools.partial(len, x)",
	"property":     "x = property(x)",
	"staticmethod": "x = staticmethod(x)",
	"classmethod":  "x = classmethod(x)",

	// Containers Go code builds out of the objects it is handed.
	"list()":                 "x = list([x])",
	"tuple()":                "x = tuple([x])",
	"list(zip)":              "x = list(zip([x]))",
	"copy builtin":           "x = copy([x])",
	"itertools.chain":        "x = itertools.chain([x])",
	"itertools.islice":       "x = itertools.islice([x], 1)",
	"itertools.accumulate":   "x = itertools.accumulate([x])",
	"itertools.product":      "x = next(iter(itertools.product([x])))",
	"itertools.permutations": "x = next(iter(itertools.permutations([x])))",
	"itertools.combinations": "x = next(iter(itertools.combinations([x], 1)))",
	"itertools.batched":      "x = next(iter(itertools.batched([x], 1)))",
	"random.sample":          "x = random.sample([x], 1)",
	"random.choices":         "x = random.choices([x], k=1)",
	"lru_cache":              "x = functools.lru_cache(x if x is not None else (lambda: 0))",
	"cache":                  "x = functools.cache(x if x is not None else (lambda: 0))",
}

const chainLength = 2000000

func chainScript(step string) string {
	return chainSetup + "\nx = None\ni = 0\nwhile i < " + strconv.Itoa(chainLength) + ":\n    " + step + "\n    i += 1\n"
}

// A memory limit must stop a script building any of these chains while it
// runs: every object is accounted, and going over the limit cancels the run
// at the evaluator's next check rather than when the script ends.
func TestMemoryLimitStopsObjectChainsMidRun(t *testing.T) {
	for name, step := range chainSteps {
		t.Run(name, func(t *testing.T) {
			p := New(WithMemoryLimit(1<<20), WithMaxValueBytes(0))
			stdlib.RegisterAll(p)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			_, err := p.EvalWithContext(ctx, chainScript(step))
			if err == nil || !strings.Contains(err.Error(), ErrMemoryLimitExceeded.Error()) {
				t.Fatalf("expected a memory limit error, got: %v", err)
			}
			if ctx.Err() != nil {
				t.Fatal("the caller's context expired; the limit did not stop the script")
			}
			i, _ := p.GetVar("i")
			if n, ok := i.(int64); !ok || n >= chainLength/2 {
				t.Fatalf("stopped after %v of %d steps: the limit was not enforced while the script ran", i, chainLength)
			}
			if p.ResourceUsage().PeakMemoryBytes == 0 {
				t.Fatal("no memory was accounted for the chain")
			}
		})
	}
}

// Without a memory limit the same chains run to completion.
func TestObjectChainsRunWithoutMemoryLimit(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)
	if _, err := p.Eval(chainSetup + "\nx = None\nfor i in range(1000):\n    x = N(x)\n    x = (x, i)\n    x = Exception(x)\n"); err != nil {
		t.Fatal(err)
	}
}

// A script cannot catch the memory-limit cancellation and keep allocating.
func TestMemoryLimitCancellationIsNotCatchable(t *testing.T) {
	p := New(WithMemoryLimit(1<<20), WithMaxValueBytes(0))
	stdlib.RegisterAll(p)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := p.EvalWithContext(ctx, chainSetup+`
x = None
caught = 0
for attempt in range(50):
    try:
        for i in range(100000):
            x = N(x)
    except Exception:
        caught += 1
    except:
        caught += 1
`)
	if err == nil || !strings.Contains(err.Error(), ErrMemoryLimitExceeded.Error()) {
		t.Fatalf("expected a memory limit error, got: %v", err)
	}
	if caught, _ := p.GetVar("caught"); caught != int64(0) {
		t.Fatalf("the script caught the cancellation %v times", caught)
	}
	if ctx.Err() != nil {
		t.Fatal("the caller's context expired; the limit did not stop the script")
	}
}

// A with block whose __exit__ suppresses everything, contextlib.suppress
// included, cannot swallow the memory-limit stop either.
func TestMemoryLimitCancellationIsNotSuppressed(t *testing.T) {
	for name, wrap := range map[string]string{
		"__exit__ returns True": "class Swallow:\n    def __enter__(self):\n        return self\n    def __exit__(self, t, v, tb):\n        return True\n",
		"contextlib.suppress":   "import contextlib\n",
	} {
		t.Run(name, func(t *testing.T) {
			ctxExpr := "Swallow()"
			if name == "contextlib.suppress" {
				ctxExpr = "contextlib.suppress(Exception)"
			}
			p := New(WithMemoryLimit(1<<20), WithMaxValueBytes(0))
			stdlib.RegisterAll(p)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			_, err := p.EvalWithContext(ctx, chainSetup+wrap+`
x = None
survived = 0
for attempt in range(50):
    with `+ctxExpr+`:
        for i in range(100000):
            x = N(x)
    survived += 1
`)
			if err == nil || !strings.Contains(err.Error(), ErrMemoryLimitExceeded.Error()) {
				t.Fatalf("expected a memory limit error, got: %v", err)
			}
			if survived, _ := p.GetVar("survived"); survived != int64(0) {
				t.Fatalf("the with block suppressed the memory stop %v times", survived)
			}
		})
	}
}

// An unbounded memoising cache grows with every distinct call even though no
// chain is involved: it must be charged, so the limit stops a script filling it.
// A bounded cache holds its size however many calls go through it.
func TestMemoryLimitCountsLRUCacheGrowth(t *testing.T) {
	p := New(WithMemoryLimit(1<<20), WithMaxValueBytes(0))
	stdlib.RegisterAll(p)
	_, err := p.Eval(`
import functools
f = functools.cache(lambda n: n)
i = 0
while i < 3000000:
    f(i)
    i += 1
`)
	if err == nil || !strings.Contains(err.Error(), ErrMemoryLimitExceeded.Error()) {
		t.Fatalf("expected a memory limit error, got: %v", err)
	}
	if i, _ := p.GetVar("i"); i.(int64) >= 1500000 {
		t.Fatalf("the unbounded cache filled for %v calls before the limit stopped it", i)
	}

	q := New(WithMemoryLimit(1<<20), WithMaxValueBytes(0))
	stdlib.RegisterAll(q)
	if _, err := q.Eval(`
import functools
f = functools.lru_cache(maxsize=128)(lambda n: n)
i = 0
while i < 500000:
    f(i)
    i += 1
`); err != nil {
		t.Fatalf("a bounded cache should run under the limit: %v", err)
	}
}

// After a memory cancellation the interpreter runs scripts again once the
// chain is gone, and the limit still applies to the next one.
func TestInterpreterUsableAfterChainCancellation(t *testing.T) {
	p := New(WithMemoryLimit(1<<20), WithMaxValueBytes(0))
	stdlib.RegisterAll(p)
	if _, err := p.Eval(chainScript(chainSteps["instance"])); err == nil {
		t.Fatal("expected a memory limit error")
	}
	// Drop the chain, then wait past the forced-collection interval so the
	// guard can see the memory released.
	p.SetVar("x", nil)
	time.Sleep(300 * time.Millisecond)
	result, err := p.Eval(chainSetup + "N(5).get()")
	if err != nil {
		t.Fatalf("interpreter unusable after a memory cancellation: %v", err)
	}
	if got := result.Inspect(); got != "5" {
		t.Fatalf("N(5).get() = %s, want 5", got)
	}
	if _, err := p.Eval(chainScript(chainSteps["tuple"])); err == nil {
		t.Fatal("the limit stopped applying after the first cancellation")
	}
}
