package object

import (
	"reflect"
	"testing"
)

// basePtr returns the address of a slice's backing array (element 0). Two
// non-empty slices share a backing array iff their base pointers are equal.
func basePtr(s []Object) uintptr {
	return reflect.ValueOf(s).Pointer()
}

func TestAcquireArgsEdgeCases(t *testing.T) {
	env := NewEnvironment()

	if got := AcquireArgs(env, 0); got != nil {
		t.Errorf("AcquireArgs(env, 0) = %v, want nil", got)
	}
	if got := AcquireArgs(env, -3); got != nil {
		t.Errorf("AcquireArgs(env, -3) = %v, want nil", got)
	}
	// nil env must not panic and must allocate.
	got := AcquireArgs(nil, 4)
	if len(got) != 4 {
		t.Errorf("AcquireArgs(nil, 4) len = %d, want 4", len(got))
	}
}

func TestReleaseArgsNoPanics(t *testing.T) {
	env := NewEnvironment()

	ReleaseArgs(nil, []Object{NewInteger(1)}) // nil env
	ReleaseArgs(env, nil)                      // nil slice
	ReleaseArgs(env, []Object{})               // empty slice
	if len(env.freeArgBufs) != 0 {
		t.Errorf("empty/nil releases pooled a buffer; freeArgBufs=%d", len(env.freeArgBufs))
	}

	// Env with no root must not panic and must not pool.
	noRoot := &Environment{}
	ReleaseArgs(noRoot, []Object{NewInteger(1)})
	if noRoot.freeArgBufs != nil && len(noRoot.freeArgBufs) != 0 {
		t.Errorf("nil-root env pooled a buffer; freeArgBufs=%d", len(noRoot.freeArgBufs))
	}
}

func TestArgsPoolRoundTripReuse(t *testing.T) {
	env := NewEnvironment()

	first := AcquireArgs(env, 4)
	if len(first) != 4 {
		t.Fatalf("first acquire len = %d, want 4", len(first))
	}
	if len(env.freeArgBufs) != 0 {
		t.Fatalf("freeArgBufs = %d after acquire, want 0", len(env.freeArgBufs))
	}
	firstCap := cap(first)
	firstPtr := basePtr(first)

	ReleaseArgs(env, first)
	if len(env.freeArgBufs) != 1 {
		t.Fatalf("freeArgBufs = %d after release, want 1", len(env.freeArgBufs))
	}

	second := AcquireArgs(env, 4)
	if len(env.freeArgBufs) != 0 {
		t.Errorf("freeArgBufs = %d after re-acquire, want 0 (should reuse)", len(env.freeArgBufs))
	}
	if cap(second) != firstCap {
		t.Errorf("reused buffer cap = %d, want %d (not the pooled backing)", cap(second), firstCap)
	}
	if basePtr(second) != firstPtr {
		t.Errorf("reused buffer does not share backing with released one; reuse not happening")
	}
}

func TestArgsPoolReleaseClearsSlots(t *testing.T) {
	env := NewEnvironment()

	buf := AcquireArgs(env, 3)
	for i := range buf {
		buf[i] = NewInteger(int64(i + 1))
	}
	ReleaseArgs(env, buf)

	// The pooled entry must be fully cleared so released Objects aren't pinned.
	pooled := env.freeArgBufs[0]
	for i, v := range pooled {
		if v != nil {
			t.Errorf("pooled slot [%d] = %v, want nil (release must clear for GC)", i, v)
		}
	}
}

func TestArgsPoolShrinkReuse(t *testing.T) {
	env := NewEnvironment()

	big := AcquireArgs(env, 4)
	bigPtr := basePtr(big)
	ReleaseArgs(env, big)

	// A smaller subsequent request reuses the larger backing (resliced).
	small := AcquireArgs(env, 2)
	if len(small) != 2 {
		t.Fatalf("small len = %d, want 2", len(small))
	}
	if basePtr(small) != bigPtr {
		t.Errorf("AcquireArgs(env, 2) allocated instead of reusing the larger pooled backing")
	}
	if len(env.freeArgBufs) != 0 {
		t.Errorf("freeArgBufs = %d, want 0 after reuse", len(env.freeArgBufs))
	}
}

func TestArgsPoolGrowKeepsSmallTop(t *testing.T) {
	env := NewEnvironment()

	small := AcquireArgs(env, 2)
	smallPtr := basePtr(small)
	smallCap := cap(small)
	ReleaseArgs(env, small)
	if len(env.freeArgBufs) != 1 {
		t.Fatalf("freeArgBufs = %d after release, want 1", len(env.freeArgBufs))
	}

	// Request bigger than the pooled buffer: it must NOT use the too-small top,
	// and the small buffer must remain pooled for a future small request.
	big := AcquireArgs(env, smallCap + 2)
	if cap(big) < smallCap+2 {
		t.Errorf("big cap = %d, want >= %d", cap(big), smallCap+2)
	}
	if len(env.freeArgBufs) != 1 {
		t.Errorf("freeArgBufs = %d after grow-acquire, want 1 (small buffer should stay pooled)", len(env.freeArgBufs))
	}

	// Now a small request should still get the original small backing.
	again := AcquireArgs(env, 2)
	if basePtr(again) != smallPtr {
		t.Errorf("small buffer was not retained for reuse after a grow-acquire")
	}
}

func TestArgsPoolOutstandingNoAlias(t *testing.T) {
	env := NewEnvironment()

	// Two acquires outstanding at once (the nesting case) must not alias.
	a := AcquireArgs(env, 3)
	b := AcquireArgs(env, 3)
	if len(env.freeArgBufs) != 0 {
		t.Errorf("freeArgBufs = %d during outstanding acquires, want 0", len(env.freeArgBufs))
	}
	if basePtr(a) == basePtr(b) {
		t.Fatalf("two outstanding acquires share a backing (would corrupt nested calls)")
	}

	a[0] = NewInteger(111)
	b[0] = NewInteger(222)
	if a[0].(*Integer).IntValue() != 111 || b[0].(*Integer).IntValue() != 222 {
		t.Errorf("outstanding buffers interfered: a[0]=%v b[0]=%v", a[0], b[0])
	}

	// LIFO: the last released is the first reused.
	bPtr := basePtr(b)
	ReleaseArgs(env, a)
	ReleaseArgs(env, b)
	if len(env.freeArgBufs) != 2 {
		t.Fatalf("freeArgBufs = %d after two releases, want 2", len(env.freeArgBufs))
	}
	next := AcquireArgs(env, 3)
	if basePtr(next) != bPtr {
		t.Errorf("LIFO violated: re-acquired buffer is not the most-recently-released one")
	}
}

func TestArgsPoolMaxCap(t *testing.T) {
	env := NewEnvironment()

	// Acquire many distinct buffers without releasing, then release them all.
	const n = maxFreeArgBufs + 5
	bufs := make([][]Object, n)
	for i := range bufs {
		bufs[i] = AcquireArgs(env, 2)
	}
	for _, b := range bufs {
		ReleaseArgs(env, b)
	}
	if got := len(env.freeArgBufs); got != maxFreeArgBufs {
		t.Errorf("freeArgBufs = %d after releasing %d, want capped at %d", got, n, maxFreeArgBufs)
	}
}

func TestArgsPoolNoStaleDataAfterReuse(t *testing.T) {
	env := NewEnvironment()

	first := AcquireArgs(env, 3)
	for i := range first {
		first[i] = NewString("stale")
	}
	ReleaseArgs(env, first) // clears slots

	// A reused buffer must present nil slots to the new caller, not stale data.
	second := AcquireArgs(env, 3)
	for i, v := range second {
		if v != nil {
			t.Errorf("reused slot [%d] = %v, want nil (stale data leaked across calls)", i, v)
		}
	}
}
