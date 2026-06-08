package object

import (
	"runtime"
	"testing"
	"time"
)

type gcReleaseProbe struct {
	value string
}

func TestSetGCReleaseHookRunsWhenTargetIsCollected(t *testing.T) {
	done := make(chan struct{}, 1)

	func() {
		target := &gcReleaseProbe{value: "release-me"}
		if err := SetGCReleaseHook(target, func() {
			done <- struct{}{}
		}); err != nil {
			t.Fatalf("SetGCReleaseHook returned error: %v", err)
		}
	}()

	waitForGCReleaseHook(t, done)
}

func TestClearGCReleaseHookPreventsHook(t *testing.T) {
	done := make(chan struct{}, 1)

	func() {
		target := &gcReleaseProbe{value: "clear-me"}
		if err := SetGCReleaseHook(target, func() {
			done <- struct{}{}
		}); err != nil {
			t.Fatalf("SetGCReleaseHook returned error: %v", err)
		}
		if err := ClearGCReleaseHook(target); err != nil {
			t.Fatalf("ClearGCReleaseHook returned error: %v", err)
		}
	}()

	deadline := time.After(150 * time.Millisecond)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-done:
			t.Fatal("release hook ran after it was cleared")
		case <-tick.C:
			runtime.GC()
			runtime.Gosched()
		case <-deadline:
			return
		}
	}
}

func TestSetGCReleaseHookValidatesTarget(t *testing.T) {
	if err := SetGCReleaseHook(nil, func() {}); err == nil {
		t.Fatal("expected nil target error")
	}
	if err := SetGCReleaseHook(gcReleaseProbe{}, func() {}); err == nil {
		t.Fatal("expected non-pointer target error")
	}
	var target *gcReleaseProbe
	if err := SetGCReleaseHook(target, func() {}); err == nil {
		t.Fatal("expected nil pointer target error")
	}
	if err := SetGCReleaseHook(&gcReleaseProbe{}, nil); err == nil {
		t.Fatal("expected nil hook error")
	}
}

func waitForGCReleaseHook(t *testing.T, done <-chan struct{}) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-done:
			return
		case <-tick.C:
			runtime.GC()
			runtime.Gosched()
		case <-deadline:
			t.Fatal("release hook did not run before timeout")
		}
	}
}
