package scriptling

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// withLimitDefaults restores the process-wide limit defaults a test changes.
func withLimitDefaults(t *testing.T) {
	t.Helper()
	mem, value := MemoryLimit(), MaxValueBytes()
	t.Cleanup(func() {
		SetMemoryLimit(mem)
		SetMaxValueBytes(value)
	})
}

// A script that keeps more than its memory limit live is stopped, and the
// error names the limit.
func TestMemoryLimitStopsScript(t *testing.T) {
	p := New(WithMemoryLimit(8<<20), WithMaxValueBytes(0))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := p.EvalWithContext(ctx, `
held = []
n = 100000
while True:
    held.append("x" * n)
`)
	if err == nil {
		t.Fatal("script ran to completion holding more than its memory limit")
	}
	if !strings.Contains(err.Error(), ErrMemoryLimitExceeded.Error()) {
		t.Fatalf("error does not name the memory limit: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("the caller's context expired; the limit did not stop the script")
	}
	// Each 100 KB string is checked against the room left before it is made,
	// so the script gets a MemoryError rather than a cancellation.
	if !strings.Contains(err.Error(), "no room for") {
		t.Fatalf("expected the up-front room check to refuse: %v", err)
	}
	if usage := p.ResourceUsage(); usage.PeakMemoryBytes > 8<<20 {
		t.Fatalf("peak %d above the limit despite the up-front check", usage.PeakMemoryBytes)
	}

	// The interpreter is usable again once the script is gone.
	p.Eval("held = None")
	if _, err := p.Eval(`x = len("abc")`); err != nil {
		t.Fatalf("interpreter unusable after a memory cancellation: %v", err)
	}
}

// Small allocations are not checked up front; the sampled accounting catches
// them and cancels the run.
func TestMemoryLimitCancelsSmallAllocations(t *testing.T) {
	p := New(WithMemoryLimit(8<<20), WithMaxValueBytes(0))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := p.EvalWithContext(ctx, `
held = []
n = 1000
i = 0
while True:
    held.append(str(i) * n)
    i += 1
`)
	if err == nil {
		t.Fatal("script ran to completion holding more than its memory limit")
	}
	if !strings.Contains(err.Error(), ErrMemoryLimitExceeded.Error()) {
		t.Fatalf("error does not name the memory limit: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("the caller's context expired; the limit did not stop the script")
	}
	usage := p.ResourceUsage()
	if usage.MemoryLimitCancellations == 0 {
		t.Fatalf("no cancellation counted: %+v", usage)
	}
	if usage.PeakMemoryBytes <= 8<<20 {
		t.Fatalf("peak %d not above the limit", usage.PeakMemoryBytes)
	}
}

// Allocating far more than the limit in total is fine when little of it is
// kept: garbage is collected rather than counted against the script.
func TestMemoryLimitAllowsChurn(t *testing.T) {
	p := New(WithMemoryLimit(16<<20), WithMaxValueBytes(0))
	result, err := p.Eval(`
total = 0
n = 1000000
for i in range(300):
    s = "y" * n
    total += len(s)
total
`)
	if err != nil {
		t.Fatalf("churning script stopped: %v", err)
	}
	if n, _ := result.AsInt(); n != 300000000 {
		t.Fatalf("total = %d", n)
	}
	if c := p.ResourceUsage().MemoryLimitCancellations; c != 0 {
		t.Fatalf("%d cancellations for garbage", c)
	}
	if p.ResourceUsage().AllocatedBytes < 300000000 {
		t.Fatalf("allocated bytes %d below what was allocated", p.ResourceUsage().AllocatedBytes)
	}
}

// One interpreter going over its limit does not affect another running at
// the same time.
func TestMemoryLimitIsPerInterpreter(t *testing.T) {
	hog := New(WithMemoryLimit(8<<20), WithMaxValueBytes(0))
	calm := New(WithMemoryLimit(64<<20), WithMaxValueBytes(0))

	var wg sync.WaitGroup
	var hogErr, calmErr error
	var calmResult object.Object
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, hogErr = hog.Eval(`
held = []
n = 100000
while True:
    held.append("h" * n)
`)
	}()
	go func() {
		defer wg.Done()
		calmResult, calmErr = calm.Eval(`
kept = []
n = 100000
for i in range(40):
    kept.append("c" * n)
    s = "t" * (n * 10)
len(kept)
`)
	}()
	wg.Wait()

	if hogErr == nil || !strings.Contains(hogErr.Error(), ErrMemoryLimitExceeded.Error()) {
		t.Fatalf("hog: %v", hogErr)
	}
	if calmErr != nil {
		t.Fatalf("calm interpreter stopped: %v", calmErr)
	}
	if n, _ := calmResult.AsInt(); n != 40 {
		t.Fatalf("calm result = %v", calmResult)
	}
	if calm.ResourceUsage().MemoryLimitCancellations != 0 {
		t.Fatal("calm interpreter was cancelled")
	}
}

// The default memory limit applies to each interpreter separately.
func TestDefaultMemoryLimitAppliesPerInterpreter(t *testing.T) {
	withLimitDefaults(t)
	SetMemoryLimit(8 << 20)
	SetMaxValueBytes(0)
	a, b := New(), New()
	if _, err := a.Eval("n = 100000\nkept = [\"a\" * n for i in range(50)]"); err != nil {
		t.Fatalf("5 MB in a: %v", err)
	}
	// b has its own 8 MB, not what is left of a's.
	if _, err := b.Eval("n = 100000\nkept = [\"b\" * n for i in range(50)]"); err != nil {
		t.Fatalf("5 MB in b: %v", err)
	}
	if _, err := b.Eval("more = [\"b\" * n for i in range(60)]"); err == nil {
		t.Fatal("11 MB held in b under an 8 MB default")
	}
}

// A script run from inside another interpreter's builtin charges the inner
// interpreter, not the outer one.
func TestNestedInterpreterChargesInner(t *testing.T) {
	outer := New(WithMemoryLimit(512<<20), WithMaxValueBytes(0))
	inner := New(WithMemoryLimit(4<<20), WithMaxValueBytes(0))
	outer.RegisterFunc("run_inner", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		_, err := inner.EvalWithContext(ctx, "n = 100000\nbig = [\"i\" * n for i in range(100)]")
		if err != nil {
			return object.NewString(err.Error())
		}
		return object.NewString("ok")
	})
	result, err := outer.Eval(`run_inner()`)
	if err != nil {
		t.Fatalf("outer: %v", err)
	}
	if !strings.Contains(result.Inspect(), ErrMemoryLimitExceeded.Error()) {
		t.Fatalf("inner script was not stopped by its own limit: %s", result.Inspect())
	}
	if outer.ResourceUsage().MemoryLimitCancellations != 0 {
		t.Fatal("the outer interpreter was charged for the inner one")
	}
}

func TestValueLimitEndToEnd(t *testing.T) {
	p := New(WithMaxValueBytes(1024))
	_, err := p.Eval("n = 2000\ns = 'x' * n")
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("value over the limit: %v", err)
	}
	result, err := p.Eval(`
try:
    n = 2000
    s = "x" * n
    r = "no error"
except MemoryError as e:
    r = "caught: " + str(e)
r
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.Inspect(), "caught: repetition result too large") {
		t.Fatalf("got %s", result.Inspect())
	}
	if _, err := p.Eval(`s = "x" * 1000`); err != nil {
		t.Fatalf("value within the limit: %v", err)
	}
}

func TestOutputCaptureLimitEndToEnd(t *testing.T) {
	p := New(WithMaxValueBytes(100))
	p.EnableOutputCapture()
	_, err := p.Eval(`
for i in range(5):
    print("0123456789")
`)
	if err != nil {
		t.Fatalf("55 bytes of output: %v", err)
	}
	if got := p.GetOutput(); got != strings.Repeat("0123456789\n", 5) {
		t.Fatalf("captured %q", got)
	}
	_, err = p.Eval(`
for i in range(20):
    print("0123456789")
`)
	if err == nil || !strings.Contains(err.Error(), "captured output too large") {
		t.Fatalf("output over the limit: %v", err)
	}
	if got := p.GetOutput(); len(got) > 100 {
		t.Fatalf("captured %d bytes over a 100 byte limit", len(got))
	}
}

// Host calls now get the call-depth guard a full evaluation has, so runaway
// recursion is an error rather than a Go stack overflow.
func TestCallFunctionHasCallDepthLimit(t *testing.T) {
	p := New()
	if _, err := p.Eval("def down(n):\n    return down(n + 1)\n"); err != nil {
		t.Fatal(err)
	}
	_, err := p.CallFunction("down", 0)
	if err == nil || !strings.Contains(err.Error(), "call depth exceeded") {
		t.Fatalf("recursion through CallFunction: %v", err)
	}
}

// Host calls honour the memory limit as evaluations do.
func TestCallFunctionMemoryLimit(t *testing.T) {
	p := New(WithMemoryLimit(8<<20), WithMaxValueBytes(0))
	if _, err := p.Eval("def hog():\n    held = []\n    n = 100000\n    while True:\n        held.append('x' * n)\n"); err != nil {
		t.Fatal(err)
	}
	_, err := p.CallFunction("hog")
	if err == nil || !strings.Contains(err.Error(), ErrMemoryLimitExceeded.Error()) {
		t.Fatalf("CallFunction over the memory limit: %v", err)
	}
}

// Usage is reported once accounting is on.
func TestResourceUsage(t *testing.T) {
	p := New(WithMemoryLimit(1<<40), WithMaxValueBytes(0))
	if _, err := p.Eval("n = 100000\nkept = [\"u\" * n for i in range(20)]"); err != nil {
		t.Fatal(err)
	}
	usage := p.ResourceUsage()
	if usage.MemoryBytes < 2000000 {
		t.Fatalf("2 MB held, %d reported", usage.MemoryBytes)
	}
	if usage.PeakMemoryBytes < usage.MemoryBytes || usage.AllocatedBytes < usage.PeakMemoryBytes {
		t.Fatalf("inconsistent usage: %+v", usage)
	}
}

// With no memory limit the run's context passes straight through.
func TestNoMemoryLimitLeavesContextAlone(t *testing.T) {
	withLimitDefaults(t)
	SetMemoryLimit(0)
	p := New()
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "v")
	var seen any
	p.RegisterFunc("probe", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		seen = ctx.Value(key{})
		return object.NewString("ok")
	})
	if _, err := p.EvalWithContext(ctx, "probe()"); err != nil {
		t.Fatal(err)
	}
	if seen != "v" {
		t.Fatal("context values lost")
	}
}

// A clone keeps its parent's limit settings, with its own accounting.
func TestCloneKeepsLimits(t *testing.T) {
	withLimitDefaults(t)
	p := New(WithMemoryLimit(32<<20), WithMaxValueBytes(1024))
	c := p.Clone()
	if c.MemoryLimit() != 32<<20 || c.MaxValueBytes() != 1024 {
		t.Fatalf("clone limits = %d %d", c.MemoryLimit(), c.MaxValueBytes())
	}
	if c.ResourceGuard() == p.ResourceGuard() {
		t.Fatal("a clone shares its parent's accounting")
	}
	if _, err := c.Eval("n = 2000\ns = 'x' * n"); err == nil {
		t.Fatal("clone ran without its parent's value limit")
	}
	// A parent following a default leaves the clone following it too.
	q := New()
	SetMaxValueBytes(2048)
	if q.Clone().MaxValueBytes() != 2048 {
		t.Fatal("clone of a default-following interpreter does not follow the default")
	}
}

func heapInUse() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// Keeping short slices of long strings must not keep the long strings alive
// while only the slices are charged.
func TestSubstringsDoNotRetainSources(t *testing.T) {
	cases := map[string]string{
		"slice":     "keep.append(s[0:10])",
		"index":     "keep.append(s[5])",
		"split":     "keep.append(s.split('9')[0][:10])",
		"strip":     "keep.append((s[:10] + ' ' * n).strip())",
		"partition": "keep.append((s[:10] + '=' + s).partition('=')[0])",
	}
	for name, keep := range cases {
		t.Run(name, func(t *testing.T) {
			p := New(WithMemoryLimit(1<<40), WithMaxValueBytes(0))
			before := heapInUse()
			src := "keep = []\nn = 1000000\nfor i in range(20):\n    s = str(i % 10) * n\n    " + keep + "\ns = None\n"
			if _, err := p.Eval(src); err != nil {
				t.Fatal(err)
			}
			grown := int64(heapInUse()) - int64(before)
			if grown > 4<<20 {
				t.Fatalf("20 short pieces kept %d MiB of their sources alive", grown>>20)
			}
			runtime.KeepAlive(p)
		})
	}
}

// Library functions that expand their input are held to the value limit.
func TestLibraryResultsHeldToValueLimit(t *testing.T) {
	p := New(WithMaxValueBytes(1024))
	stdlib.RegisterAll(p)
	result, err := p.Eval(`
import base64
import json
ok = len(base64.b64encode(b"x" * 700))
caught = []
for f in [lambda: base64.b64encode(b"x" * 1000), lambda: json.dumps(["x" * 100] * 10)]:
    try:
        f()
        caught.append("no error")
    except MemoryError:
        caught.append("caught")
[ok, caught]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[936, ['caught', 'caught']]" {
		t.Fatalf("got %s", got)
	}
}

// Memory already held over a lowered limit stops the next call before it
// runs, even if that call allocates nothing.
func TestLoweredMemoryLimitAppliesToHeldMemory(t *testing.T) {
	p := New(WithMemoryLimit(1<<40), WithMaxValueBytes(0))
	if _, err := p.Eval("n = 100000\nheld = [str(i) * n for i in range(50)]"); err != nil {
		t.Fatal(err)
	}
	p.SetMemoryLimit(1 << 20)
	_, err := p.Eval("x = 1")
	if err == nil || !strings.Contains(err.Error(), ErrMemoryLimitExceeded.Error()) {
		t.Fatalf("call under a limit already exceeded by held memory: %v", err)
	}
	if p.ResourceUsage().MemoryLimitCancellations == 0 {
		t.Fatal("no cancellation counted")
	}
	// Releasing the memory makes the interpreter usable again.
	p.SetMemoryLimit(1 << 40)
	if _, err := p.Eval("held = None"); err != nil {
		t.Fatal(err)
	}
	p.SetMemoryLimit(1 << 20)
	// Forced collections are rate limited, so the release is seen by the
	// first call after the interval.
	var err2 error
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if _, err2 = p.Eval("x = 2"); err2 == nil {
			break
		}
	}
	if err2 != nil {
		t.Fatalf("call after the held memory was released: %v", err2)
	}
}

func TestLimitDefaultsAndOptions(t *testing.T) {
	withLimitDefaults(t)

	// Nothing is limited until a limit is set.
	if MemoryLimit() != 0 || MaxValueBytes() != 0 {
		t.Fatalf("built-in defaults = %d %d, want both 0", MemoryLimit(), MaxValueBytes())
	}
	if p := New(); p.MemoryLimit() != 0 || p.MaxValueBytes() != 0 {
		t.Fatalf("New() limits = %d %d, want both 0", p.MemoryLimit(), p.MaxValueBytes())
	}
	SetMemoryLimit(32 << 20)
	SetMaxValueBytes(1 << 20)
	if MemoryLimit() != 32<<20 || MaxValueBytes() != 1<<20 {
		t.Fatalf("defaults = %d %d", MemoryLimit(), MaxValueBytes())
	}

	// An interpreter without its own limits follows the defaults, including
	// changes made after it was created.
	p := New()
	if p.MemoryLimit() != 32<<20 || p.MaxValueBytes() != 1<<20 {
		t.Fatalf("New() limits = %d %d", p.MemoryLimit(), p.MaxValueBytes())
	}
	SetMemoryLimit(64 << 20)
	if p.MemoryLimit() != 64<<20 {
		t.Fatalf("interpreter did not follow the new default: %d", p.MemoryLimit())
	}

	// Options set the interpreter's own limits.
	q := New(WithMemoryLimit(8<<20), WithMaxValueBytes(4096))
	if q.MemoryLimit() != 8<<20 || q.MaxValueBytes() != 4096 {
		t.Fatalf("option limits = %d %d", q.MemoryLimit(), q.MaxValueBytes())
	}
	SetMemoryLimit(128 << 20)
	if q.MemoryLimit() != 8<<20 {
		t.Fatal("an interpreter's own limit followed the default")
	}

	// 0 is no limit whatever the default; InheritLimit goes back to it.
	q.SetMemoryLimit(0)
	q.SetMaxValueBytes(0)
	if q.MemoryLimit() != 0 || q.MaxValueBytes() != 0 {
		t.Fatalf("0 limits = %d %d", q.MemoryLimit(), q.MaxValueBytes())
	}
	q.SetMemoryLimit(InheritLimit)
	q.SetMaxValueBytes(InheritLimit)
	if q.MemoryLimit() != 128<<20 || q.MaxValueBytes() != 1<<20 {
		t.Fatalf("inherited limits = %d %d", q.MemoryLimit(), q.MaxValueBytes())
	}

	// Negative defaults mean no limit.
	SetMemoryLimit(-1)
	SetMaxValueBytes(-1)
	if MemoryLimit() != 0 || MaxValueBytes() != 0 {
		t.Fatalf("negative defaults = %d %d", MemoryLimit(), MaxValueBytes())
	}
}

// A value shared by many containers converts to Go once per reference. The
// limited conversion stops once what it builds passes the value limit; the
// plain one is unchanged.
func TestToGoLimitedBoundsSharedExpansion(t *testing.T) {
	p := New(WithMaxValueBytes(1 << 20))
	if _, err := p.Eval("a = [0] * 100\nb = [a] * 100\nc = [b] * 100\nd = [c] * 100\nsmall = [a] * 10"); err != nil {
		t.Fatal(err)
	}
	huge, _ := p.GetVarAsObject("d")
	small, _ := p.GetVarAsObject("small")

	// Bind this goroutine to the interpreter, as a builtin running for it is.
	env := object.NewEnvironment()
	env.SetResourceGuard(p.ResourceGuard())
	env.EnterGIL()
	defer env.ExitGIL()

	if _, errObj := conversion.ToGoLimited(small); errObj != nil {
		t.Fatalf("1,000 values refused: %v", errObj)
	}
	start := time.Now()
	_, errObj := conversion.ToGoLimited(huge)
	if errObj == nil || errObj.ExceptionType != object.ExceptionTypeMemoryError {
		t.Fatalf("100 million values under a 1 MiB limit: %v", errObj)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("the limit was slow to stop the conversion")
	}
	if got := conversion.ToGo(small).([]interface{}); len(got) != 10 {
		t.Fatalf("plain ToGo = %d elements", len(got))
	}
}

// The limited conversion settles its periodic check at the end, so a value
// just past the limit is still refused.
func TestToGoLimitedSettlesRemainder(t *testing.T) {
	p := New()
	if _, err := p.Eval("v = list(range(1500))\nok = list(range(1100))"); err != nil {
		t.Fatal(err)
	}
	p.SetMaxValueBytes(1200 * 16) // from the next call on; the lists already exist
	over, _ := p.GetVarAsObject("v")
	within, _ := p.GetVarAsObject("ok")
	env := object.NewEnvironment()
	env.SetResourceGuard(p.ResourceGuard())
	env.EnterGIL()
	defer env.ExitGIL()
	if _, errObj := conversion.ToGoLimited(within); errObj != nil {
		t.Fatalf("1,100 values under a 1,200 value limit: %v", errObj)
	}
	if _, errObj := conversion.ToGoLimited(over); errObj == nil {
		t.Fatal("1,500 values passed a 1,200 value limit (the remainder after the periodic check)")
	}
}

// FloatArray conversions are counted and tracked like other containers.
func TestToGoLimitedFloatArray(t *testing.T) {
	p := New(WithMaxValueBytes(1000 * 16))
	env := object.NewEnvironment()
	env.SetResourceGuard(p.ResourceGuard())
	env.EnterGIL()
	defer env.ExitGIL()
	if _, errObj := conversion.ToGoLimited(object.NewFloatArray1D(make([]float64, 900))); errObj != nil {
		t.Fatalf("900 floats under a 1,000 value limit: %v", errObj)
	}
	if _, errObj := conversion.ToGoLimited(object.NewFloatArray1D(make([]float64, 5000))); errObj == nil {
		t.Fatal("5,000 floats passed a 1,000 value limit")
	}
	if _, errObj := conversion.ToGoLimited(object.NewFloatArray2D(make([]float64, 100*100), 100, 100)); errObj == nil {
		t.Fatal("a 100x100 array passed a 1,000 value limit")
	}
}

// Results of container methods are checked against the value limit like
// results of builtins and string methods.
func TestContainerMethodResultsLimited(t *testing.T) {
	p := New()
	if _, err := p.Eval(`
d = {i: i for i in range(500)}
l = list(range(500))
s = set(range(500))
`); err != nil {
		t.Fatal(err)
	}
	p.SetMaxValueBytes(1024)
	// keys(), values() and items() are views that allocate nothing; the list
	// one is made into is checked.
	result, err := p.Eval(`
out = []
for call in [lambda: list(d.keys()), lambda: list(d.values()), lambda: list(d.items()), lambda: l.copy(), lambda: s.copy(), lambda: s.union(s), lambda: d.copy()]:
    try:
        out.append(len(call()))
    except MemoryError:
        out.append("caught")
small = {1: 2}
out.append([list(small.keys()), [1, 2].copy(), {1}.union({2}), len(d), len(d.keys()), len(d.items())])
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['caught', 'caught', 'caught', 'caught', 'caught', 'caught', 'caught', [[1], [1, 2], {1, 2}, 500, 500, 500]]" {
		t.Fatalf("got %s", got)
	}
}

// An interpreter with no limits run from a builtin of a limited one keeps
// its own (absent) limits, and the outer limit is back afterwards.
func TestNestedInterpreterKeepsOwnLimits(t *testing.T) {
	outer := New(WithMaxValueBytes(1024))
	inner := New()
	var innerErr error
	var innerResult object.Object
	outer.RegisterFunc("run_inner", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		innerResult, innerErr = inner.Eval(`len("a" * 5000)`)
		return object.NewInteger(0)
	})
	result, err := outer.Eval(`
run_inner()
try:
    r = len("a" * 5000)
except MemoryError:
    r = "caught"
r
`)
	if err != nil {
		t.Fatal(err)
	}
	if innerErr != nil {
		t.Fatalf("inner interpreter without a limit was held to the outer limit: %v", innerErr)
	}
	if innerResult.Inspect() != "5000" || result.Inspect() != "caught" {
		t.Fatalf("inner %s, outer %s", innerResult.Inspect(), result.Inspect())
	}
}

// Converting script data for a library is held to the value limit as the one
// value it becomes: data the script could build converts, while sharing that
// would expand a small value past the limit is refused.
func TestConversionBudgetMatchesValueLimit(t *testing.T) {
	p := New(WithMaxValueBytes(1 << 20))
	if _, err := p.Eval(`data = {str(i): [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] for i in range(10000)}`); err != nil {
		t.Fatal(err)
	}
	data, err := p.GetVarAsObject("data")
	if err != nil {
		t.Fatal(err)
	}
	// Libraries convert on a goroutine bound to the interpreter's guard.
	defer object.BindGuard(p.ResourceGuard())()
	if _, convErr := conversion.ToGoLimited(data); convErr != nil {
		t.Fatalf("110,000 nodes the script built under a 1 MiB value limit refused: %s", convErr.Message)
	}
	if _, err := p.Eval("a = [0] * 100\nb = [a] * 100\nc = [b] * 100\nd = [c] * 100"); err != nil {
		t.Fatal(err)
	}
	shared, _ := p.GetVarAsObject("d")
	if _, convErr := conversion.ToGoLimited(shared); convErr == nil {
		t.Fatal("100 million shared nodes converted under a 1 MiB value limit")
	}
}
