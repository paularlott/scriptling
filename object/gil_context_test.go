package object

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

// holder parks holding the interpreter lock until release is closed.
func holdGIL(t *testing.T, env *Environment) (release chan struct{}) {
	t.Helper()
	release = make(chan struct{})
	acquired := make(chan struct{})
	go func() {
		if env.EnterGIL() {
			close(acquired)
			<-release
			env.ExitGIL()
		} else {
			close(acquired) // re-entrant on same goroutine: still "held"
			<-release
		}
	}()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("holder never acquired the lock")
	}
	return release
}

// TestEnterGILWithContextWakesOnUnlock pins the wakeup contract: a contended
// context-aware acquire completes the moment the lock frees (broadcast on
// unlock), not on a polling interval. Deterministic by construction — the
// waiter only finishes after release — with a generous timeout that would
// still catch a lost-wakeup deadlock.
func TestEnterGILWithContextWakesOnUnlock(t *testing.T) {
	env := NewEnvironment()
	release := holdGIL(t, env)

	// Cancellable context so the context-aware park runs: Background's nil
	// Done channel would silently take the plain blocking path.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan bool, 1)
	go func() {
		acquired, entered := env.EnterGILWithContext(ctx)
		if entered && acquired {
			env.ExitGIL()
		}
		done <- entered && acquired
	}()

	// Still contended: the waiter must not have entered.
	select {
	case <-done:
		t.Fatal("waiter entered the lock while it was held")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("waiter reported it could not enter after release")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter did not wake on unlock: wakeup lost")
	}
}

// TestEnterGILWithContextCancelledWhileContended pins that a cancelled caller
// abandons a contended wait without acquiring.
func TestEnterGILWithContextCancelledWhileContended(t *testing.T) {
	env := NewEnvironment()
	release := holdGIL(t, env)
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	acquired, entered := env.EnterGILWithContext(ctx)
	if entered || acquired {
		t.Fatalf("cancelled waiter entered the lock: acquired=%v entered=%v", acquired, entered)
	}
}

// TestEnterGILWithContextManyWaiters exercises the generation channel across
// several concurrent waiters: every one must acquire in turn, with no lost
// wakeups between generations.
func TestEnterGILWithContextManyWaiters(t *testing.T) {
	env := NewEnvironment()
	release := holdGIL(t, env)

	const waiters = 8
	var wg sync.WaitGroup
	started := make(chan struct{}, waiters)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for i := 0; i < waiters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started <- struct{}{}
			acquired, entered := env.EnterGILWithContext(ctx)
			if entered && acquired {
				// Brief hold so generations overlap, then hand off.
				env.ExitGIL()
			} else if entered {
				// Re-entrant case cannot happen: fresh goroutine.
				t.Error("waiter reported re-entrant acquisition")
			} else {
				t.Error("waiter failed to enter after release")
			}
		}()
	}
	for i := 0; i < waiters; i++ {
		<-started
	}
	close(release)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("not all waiters acquired: a wakeup was lost")
	}
}

// TestEnterGILWithContextFastPaths pins the cheap paths: cancellation-free
// contexts use the blocking acquire, an already-cancelled context refuses
// before touching the lock, and re-entrant entry by the holder does not
// deadlock.
func TestEnterGILWithContextFastPaths(t *testing.T) {
	env := NewEnvironment()

	acquired, entered := env.EnterGILWithContext(context.Background())
	if !acquired || !entered {
		t.Fatalf("uncontended acquire failed: %v %v", acquired, entered)
	}
	// Re-entrant from the holding goroutine.
	acquired, entered = env.EnterGILWithContext(context.Background())
	if acquired || !entered {
		t.Fatalf("re-entrant acquire should report already-held: %v %v", acquired, entered)
	}
	env.ExitGIL()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	acquired, entered = env.EnterGILWithContext(cancelled)
	if acquired || entered {
		t.Fatalf("pre-cancelled context should refuse: %v %v", acquired, entered)
	}
}

// TestEnterGILWithContextCancelledWhileParked pins the abandonment path the
// pre-cancelled test cannot reach: a waiter that has already registered and
// parked must leave on cancellation alone, with no unlock ever arriving.
// waiters==1 (same package, white-box) proves the waiter is past registration
// and inside the select before cancel fires — no sleeps, no ordering luck.
// The lock must remain acquirable afterwards: an abandoned waiter leaks
// nothing.
func TestEnterGILWithContextCancelledWhileParked(t *testing.T) {
	env := NewEnvironment()
	g := env.root.gil

	if !env.EnterGIL() {
		t.Fatal("holder could not enter")
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan bool, 1) // entered?
	go func() {
		acquired, entered := env.EnterGILWithContext(ctx)
		if entered && acquired {
			env.ExitGIL() // best-effort race: acquired despite cancellation
		}
		result <- entered
	}()

	// Deterministically wait until the waiter is parked, then cancel without
	// unlocking: ctx.Done() is the only thing that can wake it.
	for i := 0; i < 10000; i++ {
		if g.waiters.Load() > 0 {
			break
		}
		runtime.Gosched()
	}
	if g.waiters.Load() == 0 {
		t.Fatal("waiter never registered as parked")
	}
	cancel()

	select {
	case entered := <-result:
		if entered {
			t.Fatal("parked waiter entered the lock after cancellation with no unlock")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parked waiter ignored cancellation: abandon path broken")
	}

	// Lock health: the abandoned registration left nothing behind.
	env.ExitGIL()
	if acquired, entered := env.EnterGILWithContext(context.Background()); !acquired || !entered {
		t.Fatalf("lock unusable after an abandoned waiter: acquired=%v entered=%v", acquired, entered)
	}
	env.ExitGIL()
}

// TestEnterGILWithContextCancelRacesUnlock hammers the simultaneity window
// the best-effort semantics allow: cancellation lands while the unlock
// broadcast is in flight. Every waiter must either acquire (and release) or
// report not-entered — never hang, panic, or leak the lock — and the lock
// must stay healthy across all interleavings.
func TestEnterGILWithContextCancelRacesUnlock(t *testing.T) {
	env := NewEnvironment()
	g := env.root.gil
	const rounds = 500
	for i := 0; i < rounds; i++ {
		if !env.EnterGIL() {
			t.Fatalf("holder could not enter in round %d", i)
		}

		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan bool, 1)
		go func() {
			acquired, entered := env.EnterGILWithContext(ctx)
			if entered && acquired {
				env.ExitGIL()
			}
			result <- entered && acquired
		}()

		// Wait until parked, then cancel and unlock with no ordering: this
		// is the race being exercised.
		for g.waiters.Load() == 0 {
			runtime.Gosched()
		}
		cancel()
		env.ExitGIL()

		select {
		case <-result:
		case <-time.After(2 * time.Second):
			t.Fatalf("round %d: waiter hung in the cancel/unlock race", i)
		}
		// Drain any other pending state before the next round.
		for g.waiters.Load() != 0 {
			runtime.Gosched()
		}
	}

	// Lock healthy after 500 interleavings.
	if acquired, entered := env.EnterGILWithContext(context.Background()); !acquired || !entered {
		t.Fatalf("lock unhealthy after cancel/unlock stress: %v %v", acquired, entered)
	}
	env.ExitGIL()
}
