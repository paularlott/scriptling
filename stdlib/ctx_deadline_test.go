package stdlib_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// deadlineTestTimeout is the deadline the cancellation tests run under; each
// attack takes many seconds without the checks.
const deadlineTestTimeout = 300 * time.Millisecond

// deadlineTestSlack is how long past the deadline a call may run before the
// test fails: the checks stop work within milliseconds, the rest allows for
// a loaded machine and the race detector.
const deadlineTestSlack = 1500 * time.Millisecond

// expectDeadline runs script under deadlineTestTimeout and fails unless it
// stops with a timeout error soon after the deadline.
func expectDeadline(t *testing.T, script string) {
	t.Helper()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	ctx, cancel := context.WithTimeout(context.Background(), deadlineTestTimeout)
	defer cancel()
	start := time.Now()
	result, err := p.EvalWithContext(ctx, script)
	elapsed := time.Since(start)
	msg := ""
	if err != nil {
		msg = err.Error()
	} else if e, ok := result.(*object.Error); ok {
		msg = e.Message
	} else {
		t.Fatalf("expected a timeout, script finished in %v with a %s", elapsed, result.Type())
	}
	if !strings.Contains(strings.ToLower(msg), "timeout") {
		t.Fatalf("expected a timeout error, got %q after %v", msg, elapsed)
	}
	if elapsed > deadlineTestTimeout+deadlineTestSlack {
		t.Fatalf("call ran %v past a %v deadline", elapsed-deadlineTestTimeout, deadlineTestTimeout)
	}
}

func TestDifflibRatioHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import difflib\ndifflib.ratio('ab'*60000, 'ba'*60000)\n")
}

func TestDifflibUnifiedDiffHonoursDeadline(t *testing.T) {
	expectDeadline(t, `import difflib
a = "\n".join([str(i) for i in range(20000)])
b = "\n".join([str(i * 7) for i in range(20000)])
difflib.unified_diff(a, b)
`)
}

func TestDifflibOpcodesHonoursDeadline(t *testing.T) {
	expectDeadline(t, `import difflib
a = "\n".join([str(i) for i in range(20000)])
b = "\n".join([str(i * 7) for i in range(20000)])
difflib.opcodes(a, b)
`)
}

func TestDifflibGetCloseMatchesHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import difflib\ndifflib.get_close_matches('ab'*3000, ['ba'*3000]*1000)\n")
}

// The diff table of a large diff is refused for the memory it would take
// rather than built.
func TestDifflibTableRespectsMemoryLimit(t *testing.T) {
	p := scriptling.New(scriptling.WithMemoryLimit(64 << 20))
	stdlib.RegisterAll(p)
	result, err := p.Eval(`import difflib
a = "\n".join([str(i) for i in range(20000)])
b = "\n".join([str(i * 7) for i in range(20000)])
try:
    difflib.unified_diff(a, b)
    r = "built"
except MemoryError as e:
    r = "MemoryError"
r
`)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := result.AsString(); s != "MemoryError" {
		t.Fatalf("got %v, want MemoryError", result.Inspect())
	}
}

func TestItertoolsProductHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import itertools\nfor _ in itertools.product(range(1000), repeat=6): pass\n")
}

func TestItertoolsPermutationsHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import itertools\nlen(list(itertools.permutations(range(12))))\n")
}

func TestItertoolsCombinationsHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import itertools\nlen(itertools.combinations(range(60), 8))\n")
}

func TestItertoolsCombinationsWithReplacementHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import itertools\nlen(itertools.combinations_with_replacement(range(60), 8))\n")
}

func TestItertoolsIsliceSkipHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import itertools\nlist(itertools.islice(itertools.count(), 10**12, 10**12 + 1))\n")
}

// The matches are found before the first callback, which Go's regexp does
// in time linear in the (value-limited) input and cannot be interrupted;
// the callbacks after that are checked. The race detector slows the
// matching tenfold, so it gets a smaller input.
func TestReSubCallbackHonoursDeadline(t *testing.T) {
	size := "4000000"
	if raceDetector {
		size = "400000"
	}
	expectDeadline(t, "import re\nre.sub('a', lambda m: m.group(0).upper() + m.group(0), 'a' * "+size+")\n")
}

// The combinatoric results are held to the value limit as they grow.
func TestItertoolsResultsRespectValueLimit(t *testing.T) {
	p := scriptling.New(scriptling.WithMaxValueBytes(1 << 20))
	stdlib.RegisterAll(p)
	for _, call := range []string{
		"itertools.product(range(1000), repeat=6)",
		"itertools.permutations(range(12))",
		"itertools.combinations(range(60), 8)",
		"itertools.combinations_with_replacement(range(60), 8)",
		"itertools.product([1], repeat=10**12)",
	} {
		result, err := p.Eval("import itertools\ntry:\n    " + call + "\n    r = 'built'\nexcept MemoryError:\n    r = 'MemoryError'\nr\n")
		if err != nil {
			t.Fatalf("%s: %v", call, err)
		}
		if s, _ := result.AsString(); s != "MemoryError" {
			t.Fatalf("%s: got %v, want MemoryError", call, result.Inspect())
		}
	}
}

// A callable replacement's output is held to the value limit as it grows.
func TestReSubCallbackRespectsValueLimit(t *testing.T) {
	p := scriptling.New(scriptling.WithMaxValueBytes(1 << 20))
	stdlib.RegisterAll(p)
	big := strings.Repeat("x", 4096)
	result, err := p.Eval("import re\nbig = '" + big + "'\ntry:\n    re.sub('a', lambda m: big, 'a' * 100000)\n    r = 'built'\nexcept MemoryError:\n    r = 'MemoryError'\nr\n")
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := result.AsString(); s != "MemoryError" {
		t.Fatalf("got %v, want MemoryError", result.Inspect())
	}
}

// evalUnderLimits runs script with knot-like limits and a deadline, failing
// if it takes longer than the slack allows; it returns the result.
func evalUnderLimits(t *testing.T, script string) object.Object {
	t.Helper()
	p := scriptling.New(scriptling.WithMemoryLimit(256<<20), scriptling.WithMaxValueBytes(8<<20))
	stdlib.RegisterAll(p)
	ctx, cancel := context.WithTimeout(context.Background(), deadlineTestTimeout)
	defer cancel()
	start := time.Now()
	result, err := p.EvalWithContext(ctx, script)
	if elapsed := time.Since(start); elapsed > deadlineTestTimeout+deadlineTestSlack {
		t.Fatalf("took %v", elapsed)
	}
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// math.comb knows a result is too large without computing a number of up
// to n bits first.
func TestMathCombHugeIsQuick(t *testing.T) {
	r := evalUnderLimits(t, "import math\ntry:\n    math.comb(10**12, 5*10**11)\n    r = 'computed'\nexcept Exception as e:\n    r = str(e)\nr\n")
	if s, _ := r.AsString(); !strings.Contains(s, "too large") {
		t.Fatalf("got %q", s)
	}
	// The largest results still compute: C(66, 33) is the largest central
	// binomial in int64 range.
	r = evalUnderLimits(t, "import math\n[math.comb(66, 33), math.comb(10**6, 3), math.comb(10**12, 999999999999)]\n")
	if got := r.Inspect(); got != "[7219428434016265740, 166666166667000000, 1000000000000]" {
		t.Fatalf("got %s", got)
	}
}

// random.choices refuses a k past the value limit before allocating, and
// picks by binary search.
func TestRandomChoicesBounded(t *testing.T) {
	r := evalUnderLimits(t, "import random\ntry:\n    random.choices([1, 2], k=10**9)\n    r = 'built'\nexcept MemoryError:\n    r = 'MemoryError'\nr\n")
	if s, _ := r.AsString(); s != "MemoryError" {
		t.Fatalf("got %v", r.Inspect())
	}
	r = evalUnderLimits(t, "import random\nlen(random.choices(list(range(500000)), k=500000))\n")
	if n, _ := r.AsInt(); n != 500000 {
		t.Fatalf("got %v", r.Inspect())
	}
	r = evalUnderLimits(t, "import random\ntry:\n    random.sample(range(10**12), 10**9)\n    r = 'built'\nexcept MemoryError:\n    r = 'MemoryError'\nr\n")
	if s, _ := r.AsString(); s != "MemoryError" {
		t.Fatalf("got %v", r.Inspect())
	}
}

// Choices keep picking the same items for a seed: the first index whose
// cumulative weight passes the draw.
func TestRandomChoicesWeightedPicks(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	r, err := p.Eval("import random\nrandom.seed(7)\nc = random.choices(['a', 'b', 'c'], weights=[0, 1, 0], k=20)\nc.count('b')\n")
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := r.AsInt(); n != 20 {
		t.Fatalf("zero-weight items were picked: %v", r.Inspect())
	}
}

func TestTextwrapIndentRespectsValueLimit(t *testing.T) {
	r := evalUnderLimits(t, "import textwrap\ntry:\n    textwrap.indent('a\\n' * 1000000, '  ' * 1000)\n    r = 'built'\nexcept MemoryError:\n    r = 'MemoryError'\nr\n")
	if s, _ := r.AsString(); s != "MemoryError" {
		t.Fatalf("got %v", r.Inspect())
	}
}

// An outer product of two long vectors is refused for its size, and a large
// square product honours the deadline.
func TestMathMatmulBounded(t *testing.T) {
	r := evalUnderLimits(t, "import math\na = [[1.0]] * 100000\nb = [[1.0] * 100000]\ntry:\n    math.matmul(a, b)\n    r = 'built'\nexcept MemoryError:\n    r = 'MemoryError'\nr\n")
	if s, _ := r.AsString(); s != "MemoryError" {
		t.Fatalf("got %v", r.Inspect())
	}
	expectDeadline(t, "import math\nm = [[1.0] * 2000] * 2000\nmath.matmul(m, m)\n")
}
