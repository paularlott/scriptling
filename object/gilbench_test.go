package object

import (
	"context"
	"testing"
	"time"
)

// measureWakeLatency times unlock-to-acquire for a waiter that is provably
// parked: it signals that it started, then the holder waits well past two
// full poll periods of the old implementation before releasing. The waiter
// reports its acquisition timestamp on the shared monotonic clock, so the
// measurement excludes goroutine start and park time.
func measureWakeLatency(t *testing.T, env *Environment, trials int) (mean, worst time.Duration) {
	t.Helper()
	total := time.Duration(0)
	for i := 0; i < trials; i++ {
		if !env.EnterGIL() {
			t.Fatal("holder could not enter")
		}
		// A cancellable context is essential: context.Background() has a nil
		// Done channel and would take the plain blocking fast path instead of
		// the context-aware park this probe exists to measure.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		started := make(chan struct{})
		acquiredAt := make(chan time.Time, 1)
		go func() {
			close(started)
			acquired, entered := env.EnterGILWithContext(ctx)
			at := time.Now()
			if entered && acquired {
				env.ExitGIL()
			}
			acquiredAt <- at
		}()
		<-started
		time.Sleep(2 * time.Millisecond) // waiter is parked by now
		unlockAt := time.Now()
		env.ExitGIL()
		wake := (<-acquiredAt).Sub(unlockAt)
		if wake < 0 {
			wake = 0
		}
		total += wake
		if wake > worst {
			worst = wake
		}
	}
	mean = total / time.Duration(trials)
	t.Logf("trials=%d mean=%v worst=%v", trials, mean, worst)
	return mean, worst
}

func TestGILWakeLatencyProbe(t *testing.T) {
	env := NewEnvironment()
	mean, worst := measureWakeLatency(t, env, 300)

	// The old implementation polled TryLock on a 1ms ticker, so a waiter that
	// missed the unlock instant paid up to a full period (its worst case here
	// measured 1.003ms — exactly one tick). The broadcast implementation has
	// no poll period: its tail is scheduler latency, not a timer. Assert on
	// the tail, not the mean: on macOS the holder's sleep timer and the old
	// waiter's tick timer coalesce into the same timer batch, so the old
	// MEAN also looked microseconds-fast even while its worst exposed the
	// polling.
	if !raceEnabled && worst > 900*time.Microsecond {
		t.Fatalf("worst wake latency %v (mean %v): a waiter waited a poll period — broadcast wakeup is not working", worst, mean)
	}
	t.Logf("wake latency: mean=%v worst=%v (race=%v)", mean, worst, raceEnabled)
}
