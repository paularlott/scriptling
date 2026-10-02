package scriptling

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A tiny limit is exceeded by any process heap, so the guard must cancel a
// running script and the evaluation error must name the limit.
func TestMemoryLimitCancelsRunningScript(t *testing.T) {
	setMemoryGuardInterval(10 * time.Millisecond)
	defer setMemoryGuardInterval(DefaultMemoryLimitCheckInterval)
	SetMemoryLimit(1)
	defer SetMemoryLimit(0)

	before := GetMemoryLimitStats().Cancellations

	p := New()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := p.EvalWithContext(ctx, "while True:\n    pass\n")
	if err == nil {
		t.Fatal("script ran to completion under a 1-byte memory limit")
	}
	if !strings.Contains(err.Error(), ErrMemoryLimitExceeded.Error()) {
		t.Fatalf("error does not name the memory limit: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("the caller's context expired; the guard did not stop the script")
	}

	stats := GetMemoryLimitStats()
	if stats.Cancellations <= before {
		t.Fatalf("cancellation count did not increase: %+v", stats)
	}
	if stats.Running != 0 {
		t.Fatalf("script still registered after returning: %+v", stats)
	}
	if stats.HeapBytes <= 0 {
		t.Fatalf("heap was not sampled: %+v", stats)
	}
}

// With no limit the guard is inert: scripts run, nothing is registered and the
// context is passed through untouched.
func TestMemoryLimitDisabledIsNoop(t *testing.T) {
	SetMemoryLimit(0)
	if MemoryLimit() != 0 {
		t.Fatal("limit should be 0")
	}
	ctx := context.Background()
	got, release := memGuard.track(ctx)
	defer release()
	if got != ctx {
		t.Fatal("track wrapped the context while disabled")
	}

	p := New()
	if _, err := p.Eval("x = 1 + 1"); err != nil {
		t.Fatalf("eval failed with the guard disabled: %v", err)
	}
	if stats := GetMemoryLimitStats(); stats.Running != 0 || stats.Limit != 0 {
		t.Fatalf("unexpected stats while disabled: %+v", stats)
	}
}

// The guard sheds the most recently started script first, so long-running
// work that was within budget survives a newcomer that is not.
func TestMemoryLimitCancelsNewestFirst(t *testing.T) {
	SetMemoryLimit(1 << 40) // enabled, but far above any real heap so the watchdog never fires
	defer SetMemoryLimit(0)

	ctxA, releaseA := memGuard.track(context.Background())
	defer releaseA()
	ctxB, releaseB := memGuard.track(context.Background())
	defer releaseB()
	ctxC, releaseC := memGuard.track(context.Background())
	defer releaseC()

	if !memGuard.cancelNewest() {
		t.Fatal("cancelNewest found nothing to cancel")
	}
	if ctxC.Err() == nil {
		t.Fatal("newest run was not cancelled")
	}
	if ctxA.Err() != nil || ctxB.Err() != nil {
		t.Fatal("an older run was cancelled instead of the newest")
	}
	if cause := context.Cause(ctxC); cause != ErrMemoryLimitExceeded {
		t.Fatalf("cause = %v, want ErrMemoryLimitExceeded", cause)
	}

	// Released runs are forgotten; the next cancel takes the newest remaining.
	releaseB()
	if !memGuard.cancelNewest() || ctxA.Err() == nil {
		t.Fatal("after releasing B, A should be the newest and get cancelled")
	}
	if memGuard.cancelNewest() {
		t.Fatal("nothing should remain to cancel")
	}
}

// Negative limits disable the guard, and toggling it starts and stops the
// watchdog cleanly.
func TestMemoryLimitNegativeDisables(t *testing.T) {
	SetMemoryLimit(-5)
	if MemoryLimit() != 0 {
		t.Fatalf("negative limit stored as %d", MemoryLimit())
	}
	SetMemoryLimit(1 << 40)
	SetMemoryLimit(1 << 40) // idempotent while running
	if MemoryLimit() != 1<<40 {
		t.Fatal("limit not stored")
	}
	SetMemoryLimit(0)
	SetMemoryLimit(0) // idempotent while stopped
}
