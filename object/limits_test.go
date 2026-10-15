package object

import (
	"bytes"
	"context"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"
)

// keepAlive stops the compiler and collector treating test allocations as
// dead before the test has looked at the accounting.
var keepAlive []any

// withDefaults restores the process-wide defaults a test changes.
func withDefaults(t *testing.T) {
	t.Helper()
	mem, value := DefaultMemoryLimit(), DefaultMaxValueBytes()
	t.Cleanup(func() {
		SetDefaultMemoryLimit(mem)
		SetDefaultMaxValueBytes(value)
	})
}

// withSampleInterval makes memory sampling denser for a test.
func withSampleInterval(t *testing.T, n int64) {
	t.Helper()
	old := sampleInterval.Load()
	sampleInterval.Store(n)
	recomputeFastAlloc()
	t.Cleanup(func() {
		sampleInterval.Store(old)
		recomputeFastAlloc()
	})
}

// boundEnv returns an environment whose interpreter has a fresh guard, with
// the calling goroutine holding its lock (and so bound to the guard) until
// the test ends.
func boundEnv(t *testing.T) (*Environment, *ResourceGuard) {
	t.Helper()
	env := NewEnvironment()
	g := NewResourceGuard()
	env.SetResourceGuard(g)
	if !env.EnterGIL() {
		t.Fatal("EnterGIL did not acquire a fresh lock")
	}
	// The lock only binds when a limit needs it; tests set limits after
	// taking the lock, so bind regardless.
	if gil := env.root.gil; !gil.bound {
		gil.boundPrev = bindGuard(goid(), g)
		gil.bound = true
	}
	t.Cleanup(env.ExitGIL)
	return env, g
}

// bindingGuard returns a guard with a value limit of its own, so the lock
// always binds goroutines to it whatever the process-wide accounting state.
// bindingGuard returns a guard whose interpreter binds its goroutines: a
// value limit, far too large to refuse anything, turns value limits on for
// the process, which is what binding follows.
func bindingGuard() *ResourceGuard {
	g := NewResourceGuard()
	g.SetMaxValueBytes(1 << 40)
	return g
}

// waitFor polls cond, forcing collections, until it holds or time runs out.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		runtime.GC()
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDefaultLimitSetters(t *testing.T) {
	withDefaults(t)

	// Nothing is limited until a limit is set.
	if DefaultMemoryLimit() != 0 || DefaultMaxValueBytes() != 0 {
		t.Fatalf("built-in defaults = %d %d, want both 0", DefaultMemoryLimit(), DefaultMaxValueBytes())
	}

	SetDefaultMemoryLimit(-5)
	if got := DefaultMemoryLimit(); got != 0 {
		t.Fatalf("negative memory limit stored as %d, want 0", got)
	}
	SetDefaultMemoryLimit(1 << 30)
	if got := DefaultMemoryLimit(); got != 1<<30 {
		t.Fatalf("memory limit = %d", got)
	}
	if !MemoryAccountingEnabled() {
		t.Fatal("setting a memory limit did not turn accounting on")
	}

	SetDefaultMaxValueBytes(-1)
	if got := DefaultMaxValueBytes(); got != 0 {
		t.Fatalf("negative value limit stored as %d, want 0", got)
	}
	SetDefaultMaxValueBytes(4096)
	if got := DefaultMaxValueBytes(); got != 4096 {
		t.Fatalf("value limit = %d", got)
	}
	if floor := valueFloor.Load(); floor > 4096 {
		t.Fatalf("value floor %d not lowered to the configured limit", floor)
	}
}

func TestResourceGuardInheritsAndOverrides(t *testing.T) {
	withDefaults(t)
	SetDefaultMemoryLimit(1000)
	SetDefaultMaxValueBytes(2000)

	g := NewResourceGuard()
	if g.MemoryLimit() != 1000 || g.MaxValueBytes() != 2000 {
		t.Fatalf("new guard does not follow the defaults: %d %d", g.MemoryLimit(), g.MaxValueBytes())
	}

	// Defaults changed later still reach a guard that inherits.
	SetDefaultMemoryLimit(1001)
	if g.MemoryLimit() != 1001 {
		t.Fatalf("inheriting guard did not see the new default: %d", g.MemoryLimit())
	}

	g.SetMemoryLimit(10)
	g.SetMaxValueBytes(20)
	if g.MemoryLimit() != 10 || g.MaxValueBytes() != 20 {
		t.Fatalf("explicit limits not applied: %d %d", g.MemoryLimit(), g.MaxValueBytes())
	}

	// 0 means no limit for this interpreter, whatever the default.
	g.SetMemoryLimit(0)
	g.SetMaxValueBytes(0)
	if g.MemoryLimit() != 0 || g.MaxValueBytes() != 0 {
		t.Fatalf("0 did not mean no limit: %d %d", g.MemoryLimit(), g.MaxValueBytes())
	}

	// InheritLimit (or any negative value) returns to the default.
	g.SetMemoryLimit(InheritLimit)
	g.SetMaxValueBytes(-7)
	if g.MemoryLimit() != 1001 || g.MaxValueBytes() != 2000 {
		t.Fatalf("InheritLimit did not restore the defaults: %d %d", g.MemoryLimit(), g.MaxValueBytes())
	}
}

func TestNilResourceGuardUsesDefaults(t *testing.T) {
	withDefaults(t)
	SetDefaultMemoryLimit(123)
	SetDefaultMaxValueBytes(456)
	var g *ResourceGuard
	if g.MemoryLimit() != 123 || g.MaxValueBytes() != 456 {
		t.Fatalf("nil guard: %d %d", g.MemoryLimit(), g.MaxValueBytes())
	}
	if _, release := g.StartRun(context.Background()); release == nil {
		t.Fatal("nil guard StartRun returned no release")
	}
}

func TestBindingFollowsTheInterpreterLock(t *testing.T) {
	if CurrentGuard() != nil {
		t.Fatal("goroutine bound before any lock was taken")
	}

	outer := NewEnvironment()
	outerGuard := bindingGuard()
	outer.SetResourceGuard(outerGuard)
	inner := NewEnvironment()
	innerGuard := bindingGuard()
	inner.SetResourceGuard(innerGuard)

	if !outer.EnterGIL() {
		t.Fatal("outer EnterGIL did not acquire")
	}
	if CurrentGuard() != outerGuard {
		t.Fatal("holder not bound to the outer guard")
	}
	// Re-entrant acquisition does not rebind.
	if outer.EnterGIL() {
		t.Fatal("re-entrant EnterGIL acquired again")
	}
	if CurrentGuard() != outerGuard {
		t.Fatal("re-entrant acquisition changed the binding")
	}

	// A nested interpreter on the same goroutine charges the inner one, and
	// releasing it restores the outer binding.
	if !inner.EnterGIL() {
		t.Fatal("inner EnterGIL did not acquire")
	}
	if CurrentGuard() != innerGuard {
		t.Fatal("nested lock did not bind the inner guard")
	}
	inner.ExitGIL()
	if CurrentGuard() != outerGuard {
		t.Fatal("releasing the nested lock did not restore the outer guard")
	}
	outer.ExitGIL()
	if CurrentGuard() != nil {
		t.Fatal("binding left behind after the lock was released")
	}

	// Environments without a guard do not bind.
	plain := NewEnvironment()
	plain.EnterGIL()
	if CurrentGuard() != nil {
		t.Fatal("an environment without a guard bound the goroutine")
	}
	plain.ExitGIL()
}

func TestBindingWithContextAcquire(t *testing.T) {
	env := NewEnvironment()
	g := bindingGuard()
	env.SetResourceGuard(g)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	acquired, entered := env.EnterGILWithContext(ctx)
	if !acquired || !entered {
		t.Fatalf("acquire = %v, entered = %v", acquired, entered)
	}
	if CurrentGuard() != g {
		t.Fatal("EnterGILWithContext did not bind")
	}
	env.ExitGIL()
	if CurrentGuard() != nil {
		t.Fatal("ExitGIL did not unbind")
	}

	// The contended path binds too.
	env.EnterGIL()
	done := make(chan *ResourceGuard)
	go func() {
		acquired, _ := env.EnterGILWithContext(ctx)
		bound := CurrentGuard()
		if acquired {
			env.ExitGIL()
		}
		done <- bound
	}()
	time.Sleep(20 * time.Millisecond)
	env.ExitGIL()
	if got := <-done; got != g {
		t.Fatal("contended EnterGILWithContext did not bind the waiter")
	}
}

// While the holder blocks with the lock released it stays bound, and another
// goroutine taking and releasing the lock meanwhile must not disturb the
// binding the holder restores afterwards.
func TestRunUnlockedKeepsBinding(t *testing.T) {
	outer := NewEnvironment()
	outerGuard := bindingGuard()
	outer.SetResourceGuard(outerGuard)
	env := NewEnvironment()
	g := bindingGuard()
	env.SetResourceGuard(g)

	outer.EnterGIL()
	env.EnterGIL()
	env.RunUnlocked(func() {
		if CurrentGuard() != g {
			t.Error("binding lost while blocked")
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			env.EnterGIL()
			if CurrentGuard() != g {
				t.Error("other goroutine not bound")
			}
			env.ExitGIL()
			if CurrentGuard() != nil {
				t.Error("other goroutine left bound")
			}
		}()
		wg.Wait()
	})
	if CurrentGuard() != g {
		t.Fatal("binding changed after RunUnlocked")
	}
	env.ExitGIL()
	if CurrentGuard() != outerGuard {
		t.Fatal("outer binding not restored after RunUnlocked and ExitGIL")
	}
	outer.ExitGIL()
	if CurrentGuard() != nil {
		t.Fatal("binding left behind")
	}
}

// A guard set while the lock is held must not unbind a binding it did not
// make.
func TestGuardSetWhileHeldDoesNotUnbindOthers(t *testing.T) {
	outer := NewEnvironment()
	outerGuard := bindingGuard()
	outer.SetResourceGuard(outerGuard)
	outer.EnterGIL()
	defer outer.ExitGIL()

	env := NewEnvironment()
	env.EnterGIL()
	env.SetResourceGuard(bindingGuard())
	env.ExitGIL()
	if CurrentGuard() != outerGuard {
		t.Fatal("releasing a lock that never bound removed the outer binding")
	}
}

// The lock binds only when a limit needs it: memory accounting, or a value
// limit of the interpreter's own.
func TestNeedsBinding(t *testing.T) {
	wasAccounting, wasValue := accountingOn.Load(), valueLimitsOn.Load()
	accountingOn.Store(false)
	valueLimitsOn.Store(false)
	defer func() {
		accountingOn.Store(wasAccounting)
		valueLimitsOn.Store(wasValue)
	}()

	g := NewResourceGuard()
	if g.needsBinding() {
		t.Fatal("a guard needs binding with no limits anywhere")
	}
	env := NewEnvironment()
	env.SetResourceGuard(g)
	env.EnterGIL()
	bound := CurrentGuard()
	env.ExitGIL()
	if bound != nil {
		t.Fatal("the lock bound a guard that did not need it")
	}

	// Any value limit in the process binds every interpreter, so one with no
	// limit is not checked against another's.
	valueLimitsOn.Store(true)
	if !g.needsBinding() {
		t.Fatal("a value limit exists, but the guard does not need binding")
	}
	valueLimitsOn.Store(false)
	accountingOn.Store(true)
	if !g.needsBinding() {
		t.Fatal("accounting on, but the guard does not need binding")
	}
}

// An interpreter with no limits run from inside a limited one is not held to
// the outer interpreter's value limit.
func TestNestedUnlimitedGuardNotCheckedAgainstOuter(t *testing.T) {
	withDefaults(t)
	outerEnv := NewEnvironment()
	outer := NewResourceGuard()
	outer.SetMaxValueBytes(1024)
	outerEnv.SetResourceGuard(outer)
	if !outerEnv.EnterGIL() {
		t.Fatal("EnterGIL did not acquire")
	}
	defer outerEnv.ExitGIL()

	innerEnv := NewEnvironment()
	inner := NewResourceGuard()
	innerEnv.SetResourceGuard(inner)
	if !innerEnv.EnterGIL() {
		t.Fatal("EnterGIL did not acquire the inner lock")
	}
	if errObj := CheckAlloc(5000); errObj != nil {
		t.Fatalf("inner interpreter without a limit refused 5000 bytes: %s", errObj.Inspect())
	}
	innerEnv.ExitGIL()
	if errObj := CheckAlloc(5000); errObj == nil {
		t.Fatal("outer interpreter's limit not restored after the inner lock was released")
	}
}

func TestSampleWeightAndDistance(t *testing.T) {
	r := SampleInterval
	if w := sampleWeight(1); w < r || w > r+1 {
		t.Fatalf("weight of a tiny allocation = %d, want about %d", w, r)
	}
	if w := sampleWeight(r * 4); w < r*4 {
		t.Fatalf("weight %d below the allocation size", w)
	}
	for range 1000 {
		if d := nextSampleDistance(); d < 1 {
			t.Fatalf("sample distance %d", d)
		}
	}
}

// Allocations at least as large as the sample interval are charged exactly,
// and the charge is credited back once the object is collected.
func TestLargeAllocationChargedAndCredited(t *testing.T) {
	withDefaults(t)
	SetDefaultMemoryLimit(1 << 40) // accounting on, limit unreachable
	_, g := boundEnv(t)

	big := NewString(strings.Repeat("x", 1<<20))
	usage := g.Usage()
	if usage.MemoryBytes < 1<<20 {
		t.Fatalf("1 MiB string charged %d bytes", usage.MemoryBytes)
	}
	if usage.PeakMemoryBytes < usage.MemoryBytes || usage.AllocatedBytes < usage.MemoryBytes {
		t.Fatalf("inconsistent usage: %+v", usage)
	}
	runtime.KeepAlive(big)
	big = nil

	waitFor(t, "the charge to be credited", func() bool { return g.Usage().MemoryBytes < 1<<20 })
	if g.Usage().AllocatedBytes < 1<<20 {
		t.Fatal("allocated bytes went down; it must only grow")
	}
}

// Small allocations are sampled; the estimate of what is retained should be
// close to the truth.
func TestSampledEstimateIsCloseToRetained(t *testing.T) {
	withDefaults(t)
	SetDefaultMemoryLimit(1 << 40)
	withSampleInterval(t, 4<<10)
	_, g := boundEnv(t)

	before := g.Usage().MemoryBytes
	const n, size = 20000, 200
	retained := make([]*String, n)
	payload := strings.Repeat("y", size-int(stringObjectOverhead))
	for i := range retained {
		retained[i] = NewString(payload)
	}
	got := g.Usage().MemoryBytes - before
	want := int64(n * size)
	if got < want*7/10 || got > want*13/10 {
		t.Fatalf("estimate %d bytes for %d retained, want within 30%%", got, want)
	}
	keepAlive = append(keepAlive, retained)
	keepAlive = nil
}

func TestNoChargeWithoutBinding(t *testing.T) {
	withDefaults(t)
	SetDefaultMemoryLimit(1 << 40)
	if CurrentGuard() != nil {
		t.Fatal("goroutine unexpectedly bound")
	}
	// Must not panic or charge anything when no interpreter is running.
	s := NewString(strings.Repeat("z", 1<<20))
	runtime.KeepAlive(s)
}

func TestContainerGrowthIsCharged(t *testing.T) {
	withDefaults(t)
	SetDefaultMemoryLimit(1 << 40)
	withSampleInterval(t, 1<<10)
	_, g := boundEnv(t)

	before := g.Usage().AllocatedBytes
	d := NewDict()
	for i := range 5000 {
		k := "k" + strconv.Itoa(i)
		d.Store(k, &String{value: k}, &Null{})
	}
	afterDict := g.Usage().AllocatedBytes
	if afterDict-before < 5000*dictEntryWeight/2 {
		t.Fatalf("dict of 5000 entries charged only %d bytes", afterDict-before)
	}

	s := NewSet()
	for i := range 5000 {
		s.add(NewInteger(int64(i + 100000)))
	}
	if s.charged < 5000 {
		t.Fatalf("set charged for %d elements, has %d", s.charged, len(s.Elements))
	}
	afterSet := g.Usage().AllocatedBytes
	if afterSet-afterDict < 5000*setEntryWeight/2 {
		t.Fatalf("set of 5000 elements charged only %d bytes", afterSet-afterDict)
	}
	k := NewSet()
	for i := range 100 {
		k.AddKeyed(strconv.Itoa(i), &Null{})
	}
	if k.charged < 100 {
		t.Fatalf("AddKeyed did not charge growth: %d", k.charged)
	}

	inst := NewInstance(&Class{Name: "C"})
	for i := range 2000 {
		inst.SetField("f"+strconv.Itoa(i), &Null{})
	}
	if g.Usage().AllocatedBytes-afterSet < 2000*dictEntryWeight/2 {
		t.Fatal("instance attribute growth not charged")
	}

	b := NewBytes(make([]byte, 1<<20))
	bs := NewBytesFromString(strings.Repeat("b", 1<<20))
	if g.Usage().MemoryBytes < 2<<20 {
		t.Fatal("bytes not charged")
	}
	keepAlive = append(keepAlive, d, s, k, inst, b, bs)
	keepAlive = nil
}

func TestTrackHelpers(t *testing.T) {
	withDefaults(t)
	SetDefaultMemoryLimit(1 << 40)
	_, g := boundEnv(t)

	before := g.Usage().AllocatedBytes
	s := make([]Object, 0, 1<<17) // 1 MiB of slots
	TrackSlice(s)
	if got := g.Usage().AllocatedBytes - before; got < 1<<20 {
		t.Fatalf("TrackSlice charged %d bytes for 1 MiB", got)
	}
	before = g.Usage().AllocatedBytes
	w := make([]byte, 0, 1<<16)
	TrackSliceWeighted(w, 32) // 2 MiB
	if got := g.Usage().AllocatedBytes - before; got < 2<<20 {
		t.Fatalf("TrackSliceWeighted charged %d bytes", got)
	}
	before = g.Usage().AllocatedBytes
	p := new([1 << 20]byte)
	TrackAlloc(p, 1<<20)
	if got := g.Usage().AllocatedBytes - before; got < 1<<20 {
		t.Fatalf("TrackAlloc charged %d bytes", got)
	}
	// Empty and non-positive sizes are ignored.
	TrackSlice([]Object(nil))
	TrackAlloc(p, 0)
	keepAlive = append(keepAlive, s, w, p)
	keepAlive = nil
}

func TestCheckAllocValueLimit(t *testing.T) {
	withDefaults(t)
	_, g := boundEnv(t)
	g.SetMaxValueBytes(1000)

	if err := CheckAlloc(1000); err != nil {
		t.Fatalf("size at the limit refused: %v", err)
	}
	err := CheckAlloc(1001)
	if err == nil {
		t.Fatal("size over the limit accepted")
	}
	e, ok := err.(*Error)
	if !ok || e.ExceptionType != ExceptionTypeMemoryError {
		t.Fatalf("want a MemoryError, got %#v", err)
	}
	if !strings.Contains(e.Message, "1001") || !strings.Contains(e.Message, "1000") {
		t.Fatalf("message does not give the sizes: %q", e.Message)
	}

	// No limit for this interpreter.
	g.SetMaxValueBytes(0)
	if err := CheckAlloc(1 << 40); err != nil {
		t.Fatalf("unlimited interpreter refused: %v", err)
	}
}

func TestCheckAllocUsesDefaultWithoutInterpreter(t *testing.T) {
	withDefaults(t)
	SetDefaultMaxValueBytes(5000)
	if err := CheckAlloc(5000); err != nil {
		t.Fatalf("at the default limit: %v", err)
	}
	if err := CheckAlloc(5001); err == nil {
		t.Fatal("over the default limit accepted with no interpreter running")
	}
	if MaxValueBytes() != 5000 {
		t.Fatalf("MaxValueBytes = %d", MaxValueBytes())
	}
}

func TestCheckAllocMemoryHeadroom(t *testing.T) {
	withDefaults(t)
	_, g := boundEnv(t)
	g.SetMaxValueBytes(0)
	g.SetMemoryLimit(4 << 20)

	if err := CheckAlloc(5 << 20); err == nil || !strings.Contains(err.(*Error).Message, "memory limit") {
		t.Fatalf("a value larger than the whole memory limit: %v", err)
	}
	if err := CheckAlloc(1 << 20); err != nil {
		t.Fatalf("room for 1 MiB refused: %v", err)
	}

	// Hold 3 MiB live: there is no longer room for 2 MiB, even after a
	// collection.
	held := NewString(strings.Repeat("h", 3<<20))
	if err := CheckAlloc(2 << 20); err == nil || err.(*Error).ExceptionType != ExceptionTypeMemoryError {
		t.Fatalf("over the remaining room: %v", err)
	}
	runtime.KeepAlive(held)
	held = nil

	// Once the held string is garbage, a collection makes room again.
	waitFor(t, "room after the string is collected", func() bool { return CheckAlloc(2<<20) == nil })
}

func TestCheckElements(t *testing.T) {
	withDefaults(t)
	_, g := boundEnv(t)
	g.SetMaxValueBytes(800)
	if err := CheckElements(0); err != nil {
		t.Fatal(err)
	}
	// An element slot is an interface value: 16 bytes on 64-bit platforms.
	slot := int64(unsafe.Sizeof(Object(nil)))
	if err := CheckElements(800 / slot); err != nil {
		t.Fatalf("%d elements refused: %v", 800/slot, err)
	}
	if err := CheckElements(800/slot + 1); err == nil {
		t.Fatal("one element too many accepted under an 800 byte limit")
	}
	if err := CheckElements(1 << 62); err == nil {
		t.Fatal("overflowing element count accepted")
	}
}

func TestStartRunAndMemoryCancellation(t *testing.T) {
	withDefaults(t)
	g := NewResourceGuard()

	// No limit: the context passes through untouched.
	ctx := context.Background()
	got, release := g.StartRun(ctx)
	if got != ctx {
		t.Fatal("StartRun wrapped the context with no memory limit")
	}
	release()

	g.SetMemoryLimit(1 << 20)
	runCtx, release := g.StartRun(ctx)
	defer release()
	other, releaseOther := g.StartRun(ctx)
	defer releaseOther()

	// Hold more than the limit live and charge it: both runs are cancelled.
	env := NewEnvironment()
	env.SetResourceGuard(g)
	env.EnterGIL()
	held := NewString(strings.Repeat("m", 2<<20))
	env.ExitGIL()

	for _, c := range []context.Context{runCtx, other} {
		select {
		case <-c.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("run not cancelled over the memory limit")
		}
		if !stderrors.Is(context.Cause(c), ErrMemoryLimitExceeded) {
			t.Fatalf("cancellation cause = %v", context.Cause(c))
		}
	}
	if n := g.Usage().MemoryLimitCancellations; n != 2 {
		t.Fatalf("cancellations = %d, want 2", n)
	}
	runtime.KeepAlive(held)

	// A new run clears the cancelling state.
	_, releaseNew := g.StartRun(ctx)
	if g.cancelling.Load() {
		t.Fatal("new run did not clear the cancelling state")
	}
	releaseNew()
}

// Garbage that takes the estimate over the limit is collected rather than
// treated as live: runs are not cancelled.
func TestGarbageOverLimitIsReclaimed(t *testing.T) {
	withDefaults(t)
	g := NewResourceGuard()
	g.SetMemoryLimit(8 << 20)
	runCtx, release := g.StartRun(context.Background())
	defer release()

	env := NewEnvironment()
	env.SetResourceGuard(g)
	env.EnterGIL()
	for range 64 {
		s := NewString(strings.Repeat("g", 1<<20)) // 64 MiB churned, ~1 MiB live
		runtime.KeepAlive(s)
	}
	env.ExitGIL()

	if runCtx.Err() != nil {
		t.Fatalf("run cancelled for garbage: %v", context.Cause(runCtx))
	}
	if g.Usage().MemoryLimitCancellations != 0 {
		t.Fatal("cancellation counted for garbage")
	}
}

func TestReclaimUntil(t *testing.T) {
	if !reclaimUntil(func() bool { return true }) {
		t.Fatal("reclaimUntil reported false for a condition that holds")
	}
	oldPolls, oldInterval := reclaimPolls, reclaimPollInterval
	reclaimPolls, reclaimPollInterval = 2, time.Millisecond
	defer func() { reclaimPolls, reclaimPollInterval = oldPolls, oldInterval }()
	if reclaimUntil(func() bool { return false }) {
		t.Fatal("reclaimUntil reported true for a condition that never holds")
	}
}

func TestReadAllLimited(t *testing.T) {
	withDefaults(t)
	_, g := boundEnv(t)
	g.SetMaxValueBytes(10)

	data, limitErr, err := ReadAllLimited(strings.NewReader("0123456789"), "body")
	if err != nil || limitErr != nil || string(data) != "0123456789" {
		t.Fatalf("at the limit: %q %v %v", data, limitErr, err)
	}
	data, limitErr, err = ReadAllLimited(strings.NewReader("0123456789A"), "body")
	if err != nil || data != nil || limitErr == nil {
		t.Fatalf("over the limit: %q %v %v", data, limitErr, err)
	}
	if e := limitErr.(*Error); e.ExceptionType != ExceptionTypeMemoryError || !strings.Contains(e.Message, "body too large") {
		t.Fatalf("limit error = %#v", e)
	}
	_, _, err = ReadAllLimited(io.MultiReader(strings.NewReader("ab"), errReader{}), "body")
	if err == nil {
		t.Fatal("read error not returned")
	}

	g.SetMaxValueBytes(0)
	big := bytes.Repeat([]byte("q"), 1000)
	data, limitErr, err = ReadAllLimited(bytes.NewReader(big), "body")
	if err != nil || limitErr != nil || len(data) != 1000 {
		t.Fatalf("unlimited: %d %v %v", len(data), limitErr, err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, stderrors.New("boom") }

func TestReadFileLimited(t *testing.T) {
	withDefaults(t)
	_, g := boundEnv(t)
	g.SetMaxValueBytes(10)
	dir := t.TempDir()
	small := filepath.Join(dir, "small")
	large := filepath.Join(dir, "large")
	if err := os.WriteFile(small, []byte("tiny"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(large, []byte("much too large"), 0o600); err != nil {
		t.Fatal(err)
	}

	data, limitErr, err := ReadFileLimited(small)
	if err != nil || limitErr != nil || string(data) != "tiny" {
		t.Fatalf("small file: %q %v %v", data, limitErr, err)
	}
	_, limitErr, err = ReadFileLimited(large)
	if err != nil || limitErr == nil || !strings.Contains(limitErr.(*Error).Message, "file too large") {
		t.Fatalf("large file: %v %v", limitErr, err)
	}
	if _, _, err := ReadFileLimited(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing file did not fail")
	}
}

func TestOutputCaptureLimit(t *testing.T) {
	env, g := boundEnv(t)
	g.SetMaxValueBytes(10)
	env.EnableOutputCapture()
	w := env.GetWriter()

	if _, err := io.WriteString(w, "12345"); err != nil {
		t.Fatalf("write within the limit: %v", err)
	}
	if _, err := w.Write([]byte("67890")); err != nil {
		t.Fatalf("write up to the limit: %v", err)
	}
	if _, err := io.WriteString(w, "!"); !stderrors.Is(err, ErrOutputLimitExceeded) {
		t.Fatalf("write over the limit: %v", err)
	}
	if _, err := w.Write([]byte("!")); !stderrors.Is(err, ErrOutputLimitExceeded) {
		t.Fatalf("byte write over the limit: %v", err)
	}
	if got := env.GetOutput(); got != "1234567890" {
		t.Fatalf("captured %q", got)
	}
	// GetOutput resets, so there is room again.
	if _, err := io.WriteString(w, "again"); err != nil {
		t.Fatalf("write after GetOutput: %v", err)
	}
	if got := env.GetOutput(); got != "again" {
		t.Fatalf("captured %q", got)
	}

	// A plain strings.Builder set as the writer is still read by GetOutput.
	var sb strings.Builder
	env.SetOutputWriter(&sb)
	io.WriteString(&sb, "plain")
	if got := env.GetOutput(); got != "plain" {
		t.Fatalf("GetOutput from a strings.Builder = %q", got)
	}
}

func TestNewMemoryError(t *testing.T) {
	e := NewMemoryError("value of %d bytes", 5)
	if e.ExceptionType != ExceptionTypeMemoryError || e.Message != "value of 5 bytes" {
		t.Fatalf("%#v", e)
	}
}

func TestReadLimitedErrVariants(t *testing.T) {
	withDefaults(t)
	_, g := boundEnv(t)
	g.SetMaxValueBytes(10)

	data, err := ReadAllLimitedErr(strings.NewReader("0123456789"), "body")
	if err != nil || string(data) != "0123456789" {
		t.Fatalf("within the limit: %q %v", data, err)
	}
	_, err = ReadAllLimitedErr(strings.NewReader("0123456789A"), "body")
	if !stderrors.Is(err, ErrValueTooLarge) || !strings.Contains(err.Error(), "body too large") {
		t.Fatalf("over the limit: %v", err)
	}
	if _, err := ReadAllLimitedErr(errReader{}, "body"); err == nil || stderrors.Is(err, ErrValueTooLarge) {
		t.Fatalf("read error: %v", err)
	}

	dir := t.TempDir()
	small, large := filepath.Join(dir, "small"), filepath.Join(dir, "large")
	os.WriteFile(small, []byte("tiny"), 0o600)
	os.WriteFile(large, []byte("much too large"), 0o600)
	if data, err := ReadFileLimitedErr(small); err != nil || string(data) != "tiny" {
		t.Fatalf("small file: %q %v", data, err)
	}
	if _, err := ReadFileLimitedErr(large); !stderrors.Is(err, ErrValueTooLarge) {
		t.Fatalf("large file: %v", err)
	}
	if _, err := ReadFileLimitedErr(filepath.Join(dir, "missing")); err == nil || stderrors.Is(err, ErrValueTooLarge) {
		t.Fatalf("missing file: %v", err)
	}
}

func TestFileEquals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	os.WriteFile(path, []byte("abc"), 0o600)
	if !FileEquals(path, []byte("abc")) {
		t.Fatal("identical content reported different")
	}
	if FileEquals(path, []byte("abd")) {
		t.Fatal("same size, different content reported equal")
	}
	if FileEquals(path, []byte("abcd")) {
		t.Fatal("different size reported equal")
	}
	if FileEquals(filepath.Join(dir, "missing"), nil) {
		t.Fatal("missing file reported equal")
	}
	if FileEquals(dir, nil) {
		t.Fatal("directory reported equal")
	}
}

func TestNewSubstringDetaches(t *testing.T) {
	src := strings.Repeat("s", 4096)
	sub := src[:10]

	was := accountingOn.Load()
	defer func() {
		accountingOn.Store(was)
		recomputeFastAlloc()
	}()

	accountingOn.Store(false)
	if got := NewSubstring(sub, src).value; unsafe.StringData(got) != unsafe.StringData(src) {
		t.Fatal("copied a substring with accounting off")
	}

	accountingOn.Store(true)
	got := NewSubstring(sub, src).value
	if got != sub {
		t.Fatalf("content changed: %q", got)
	}
	if unsafe.StringData(got) == unsafe.StringData(src) {
		t.Fatal("a short piece of a long string still shares its memory")
	}
	// Little waste: kept as is.
	near := src[:4000]
	if got := NewSubstring(near, src).value; unsafe.StringData(got) != unsafe.StringData(src) {
		t.Fatal("copied a substring that wastes little")
	}
	if got := NewSubstring(src, src).value; unsafe.StringData(got) != unsafe.StringData(src) {
		t.Fatal("copied the whole string")
	}
}

func TestByteString(t *testing.T) {
	for i := range 256 {
		s := ByteString(byte(i))
		if len(s) != 1 || s[0] != byte(i) {
			t.Fatalf("ByteString(%d) = %q", i, s)
		}
	}
	if testing.AllocsPerRun(100, func() { _ = ByteString('a') }) != 0 {
		t.Fatal("ByteString allocates")
	}
}

func TestBindGuard(t *testing.T) {
	g := bindingGuard()
	done := make(chan *ResourceGuard, 2)
	go func() {
		release := BindGuard(g)
		done <- CurrentGuard()
		release()
		done <- CurrentGuard()
	}()
	if got := <-done; got != g {
		t.Fatal("BindGuard did not bind the goroutine")
	}
	if got := <-done; got != nil {
		t.Fatal("release did not unbind")
	}
	BindGuard(nil)() // a nil guard binds nothing and must not panic
	if CurrentGuard() != nil {
		t.Fatal("binding a nil guard changed the binding")
	}
}

func TestCheckValue(t *testing.T) {
	withDefaults(t)
	_, g := boundEnv(t)
	g.SetMaxValueBytes(64)
	slot := int64(unsafe.Sizeof(Object(nil)))

	cases := []struct {
		obj  Object
		over bool
	}{
		{NewString(strings.Repeat("x", 64)), false},
		{NewString(strings.Repeat("x", 65)), true},
		{NewBytes(make([]byte, 64)), false},
		{NewBytes(make([]byte, 65)), true},
		{&List{Elements: make([]Object, 64/slot)}, false},
		{&List{Elements: make([]Object, 64/slot+1)}, true},
		{&Tuple{Elements: make([]Object, 64/slot)}, false},
		{&Tuple{Elements: make([]Object, 64/slot+1)}, true},
		{NewInteger(1 << 40), false},
		{NewDict(), false},
	}
	for i, tc := range cases {
		err := CheckValue(tc.obj)
		if (err != nil) != tc.over {
			t.Errorf("case %d (%s): over=%v, err=%v", i, tc.obj.Type(), tc.over, err)
		}
		if err != nil && err.(*Error).ExceptionType != ExceptionTypeMemoryError {
			t.Errorf("case %d: not a MemoryError", i)
		}
	}
	g.SetMaxValueBytes(0)
	if CheckValue(NewString(strings.Repeat("x", 1000))) != nil {
		t.Fatal("no limit, but a value was refused")
	}
}

// DetachRun steps out of every run a context belongs to, keeping the host's
// context, its values and its cancellation.
func TestDetachRun(t *testing.T) {
	type key struct{}
	host, cancelHost := context.WithCancel(context.WithValue(context.Background(), key{}, "host"))
	defer cancelHost()
	if DetachRun(host) != host {
		t.Fatal("a context outside any run changed")
	}

	g := NewResourceGuard()
	g.SetMemoryLimit(1 << 40)
	outer, releaseOuter := g.StartRun(host)
	inner, releaseInner := g.StartRun(context.WithValue(outer, key{}, "inside"))

	detached := DetachRun(inner)
	if detached != host {
		t.Fatal("DetachRun did not return the host context")
	}
	releaseInner()
	releaseOuter()
	if outer.Err() == nil {
		t.Fatal("ending a run did not cancel its context")
	}
	if detached.Err() != nil {
		t.Fatal("the detached context was cancelled with the runs")
	}
	if detached.Value(key{}) != "host" {
		t.Fatal("host values lost")
	}
	cancelHost()
	if detached.Err() == nil {
		t.Fatal("the host's cancellation does not reach the detached context")
	}
}

// With only a memory limit, a read stops once its buffer no longer fits, and
// what it reads is charged.
func TestReadAllLimitedMemoryOnly(t *testing.T) {
	withDefaults(t)
	_, g := boundEnv(t)
	g.SetMaxValueBytes(0)
	g.SetMemoryLimit(4 << 20)

	data, limitErr, err := ReadAllLimited(bytes.NewReader(make([]byte, 1<<20)), "body")
	if err != nil || limitErr != nil || len(data) != 1<<20 {
		t.Fatalf("1 MiB under a 4 MiB limit: %d %v %v", len(data), limitErr, err)
	}
	if g.Usage().AllocatedBytes < 1<<20 {
		t.Fatal("the read was not charged")
	}
	_, limitErr, err = ReadAllLimited(bytes.NewReader(make([]byte, 16<<20)), "body")
	if err != nil || limitErr == nil || !strings.Contains(limitErr.(*Error).Message, "memory limit exceeded") {
		t.Fatalf("16 MiB under a 4 MiB limit: %v %v", limitErr, err)
	}
	runtime.KeepAlive(data)
}

// With only a memory limit, captured output stops once it no longer fits.
func TestOutputCaptureMemoryOnly(t *testing.T) {
	withDefaults(t)
	env, g := boundEnv(t)
	g.SetMaxValueBytes(0)
	g.SetMemoryLimit(2 << 20)
	env.EnableOutputCapture()
	w := env.GetWriter()
	chunk := strings.Repeat("o", 256<<10)
	var err error
	for range 16 {
		if _, err = io.WriteString(w, chunk); err != nil {
			break
		}
	}
	if !stderrors.Is(err, ErrOutputNoRoom) || !stderrors.Is(err, ErrOutputLimitExceeded) {
		t.Fatalf("4 MiB of output under a 2 MiB limit: %v", err)
	}
	if got := len(env.GetOutput()); got > 2<<20 {
		t.Fatalf("captured %d bytes over a 2 MiB limit", got)
	}
}

func TestCopyLimits(t *testing.T) {
	withDefaults(t)
	from := NewResourceGuard()
	from.SetMemoryLimit(100) // explicit; the value limit follows the default
	to := NewResourceGuard()
	to.SetMaxValueBytes(5)
	to.CopyLimits(from)
	if to.MemoryLimit() != 100 {
		t.Fatalf("explicit limit not copied: %d", to.MemoryLimit())
	}
	if to.maxValue.Load() != InheritLimit {
		t.Fatal("an inherited limit was not copied as inherited")
	}
	SetDefaultMaxValueBytes(777)
	if to.MaxValueBytes() != 777 {
		t.Fatal("copied guard does not follow the default for an inherited limit")
	}
	to.CopyLimits(nil) // must not panic
	if to.Usage().MemoryBytes != 0 {
		t.Fatal("accounting was copied")
	}
}

func TestGrowthRoom(t *testing.T) {
	if got := GrowthRoom(0, 100); got != 100 {
		t.Fatalf("empty buffer growing to 100 = %d", got)
	}
	if got := GrowthRoom(1000, 1001); got != 3000 {
		t.Fatalf("1000 growing by one = %d, want old 1000 + doubled 2000", got)
	}
	if got := GrowthRoom(1000, 5000); got != 6000 {
		t.Fatalf("1000 growing to 5000 = %d, want old 1000 + 5000", got)
	}
}

// A write that fits the limit as output but not alongside the old backing
// array during growth is refused.
func TestOutputCaptureGrowthRoom(t *testing.T) {
	withDefaults(t)
	env, g := boundEnv(t)
	g.SetMaxValueBytes(0)
	g.SetMemoryLimit(1 << 20)
	env.EnableOutputCapture()
	w := env.GetWriter()
	// 300 KiB fits; growing from 300 KiB to 600 KiB needs 300 + 600 KiB.
	if _, err := io.WriteString(w, strings.Repeat("a", 300<<10)); err != nil {
		t.Fatalf("300 KiB: %v", err)
	}
	// Hold 400 KiB elsewhere: 300 + 600 + 400 KiB is over 1 MiB.
	held := NewString(strings.Repeat("h", 400<<10))
	if _, err := io.WriteString(w, strings.Repeat("b", 300<<10)); !stderrors.Is(err, ErrOutputNoRoom) {
		t.Fatalf("growth that needs 900 KiB with 400 KiB held under 1 MiB: %v", err)
	}
	runtime.KeepAlive(held)
}

// SlotsWithin lets a container literal skip its check and tracking with one
// atomic load, but only while accounting is off and the literal is within
// every value limit configured so far (the floor only ever lowers, so the
// test works relative to it).
func TestSlotsWithin(t *testing.T) {
	withDefaults(t)
	was := accountingOn.Load()
	defer func() {
		accountingOn.Store(was)
		recomputeFastAlloc()
	}()

	accountingOn.Store(false)
	recomputeFastAlloc()
	slot := int(unsafe.Sizeof(Object(nil)))
	within := int(valueFloor.Load()) / slot
	if !SlotsWithin(0) || !SlotsWithin(within) {
		t.Fatalf("a literal of %d elements is not within a floor of %d bytes", within, valueFloor.Load())
	}
	if SlotsWithin(within + 1) {
		t.Fatalf("%d elements passed a floor of %d bytes", within+1, valueFloor.Load())
	}
	SetDefaultMaxValueBytes(64)
	if SlotsWithin(5) {
		t.Fatal("5 elements (80 bytes) passed a 64 byte value limit")
	}

	// With accounting on, every literal takes the checked path so its
	// backing array is tracked.
	accountingOn.Store(true)
	recomputeFastAlloc()
	if SlotsWithin(1) {
		t.Fatal("a literal skipped tracking with accounting on")
	}
}

// A converted Go map is charged for as long as it lives and credited back
// once it is collected.
func TestTrackMapLifetime(t *testing.T) {
	withDefaults(t)
	SetDefaultMemoryLimit(1 << 40)
	_, g := boundEnv(t)

	before := g.Usage().MemoryBytes
	m := make(map[string]int, 4096)
	TrackMap(m, 4096*64)
	if g.Usage().MemoryBytes-before < 4096*64 {
		t.Fatalf("map charged %d bytes", g.Usage().MemoryBytes-before)
	}
	runtime.KeepAlive(m)
	m = nil
	waitFor(t, "the map's charge to be credited", func() bool { return g.Usage().MemoryBytes-before < 4096*64 })

	TrackMap(nil, 100)                 // nil map: nothing to attach to
	TrackMap(map[string]int(nil), 100) // typed nil: same
	TrackMap("not a map", 100)         // ignored
	TrackMap(map[int]int{1: 1}, 0)     // nothing to charge
}

// A refusal that a forced collection did not fix is not answered with
// another collection within forcedGCMinInterval.
func TestForcedGCRateLimited(t *testing.T) {
	numGC := func() uint32 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.NumGC
	}
	never := func() bool { return false }
	g := NewResourceGuard()

	before := numGC()
	if g.reclaim(never) {
		t.Fatal("reclaim reported success for a condition that never holds")
	}
	if numGC() == before {
		t.Fatal("the first refusal did not force a collection")
	}
	if g.lastForcedGC.Load() == 0 {
		t.Fatal("an unhelpful collection was not recorded")
	}

	// A natural collection can land between two readings; the point is that
	// reclaim itself forces none, so one quiet attempt is enough.
	quiet := false
	for range 5 {
		before = numGC()
		g.reclaim(never)
		if numGC() == before {
			quiet = true
			break
		}
	}
	if !quiet {
		t.Fatal("a refusal within forcedGCMinInterval forced another collection")
	}

	g.lastForcedGC.Store(time.Now().Add(-2 * forcedGCMinInterval).UnixNano())
	before = numGC()
	g.reclaim(never)
	if numGC() == before {
		t.Fatal("a refusal after forcedGCMinInterval did not force a collection")
	}

	// A collection that helps clears the backoff; a condition that already
	// holds needs no collection.
	g.lastForcedGC.Store(time.Now().Add(-2 * forcedGCMinInterval).UnixNano())
	calls := 0
	if !g.reclaim(func() bool { calls++; return calls > 1 }) {
		t.Fatal("reclaim did not report the condition holding after a collection")
	}
	if g.lastForcedGC.Load() != 0 {
		t.Fatal("a helpful collection did not clear the backoff")
	}
	if !g.reclaim(func() bool { return true }) {
		t.Fatal("reclaim failed a condition that holds")
	}
}

// A FloatArray's data is charged to the interpreter that creates it.
func TestFloatArrayCharged(t *testing.T) {
	withDefaults(t)
	SetDefaultMemoryLimit(1 << 40)
	_, g := boundEnv(t)
	before := g.Usage().MemoryBytes
	a := NewFloatArray1D(make([]float64, 1<<17))
	b := NewFloatArray2D(make([]float64, 1<<17), 1<<10, 1<<7)
	if charged := g.Usage().MemoryBytes - before; charged < 2<<20 {
		t.Fatalf("two 1 MiB float arrays charged %d bytes", charged)
	}
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
}

// Deep copies are charged like the values they copy.
func TestCloneObjectCharged(t *testing.T) {
	withDefaults(t)
	SetDefaultMemoryLimit(1 << 40)
	_, g := boundEnv(t)
	elems := make([]Object, 1<<16)
	for i := range elems {
		elems[i] = NewInteger(int64(i))
	}
	src := &List{Elements: []Object{
		NewFloatArray1D(make([]float64, 1<<17)),
		&List{Elements: elems},
		&Tuple{Elements: elems},
	}}
	before := g.Usage().MemoryBytes
	clone := CloneObject(src)
	// 1 MiB of floats, two 1 MiB element arrays.
	if charged := g.Usage().MemoryBytes - before; charged < 3<<20 {
		t.Fatalf("deep copy of 3 MiB charged %d bytes", charged)
	}
	runtime.KeepAlive(clone)
	runtime.KeepAlive(src)
}
