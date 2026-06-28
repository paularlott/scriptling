package console

import (
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func goID() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	if idx := strings.IndexByte(s, ' '); idx > 0 {
		id, _ := strconv.Atoi(s[:idx])
		return id
	}
	return 0
}

// All handler jobs must run on a single goroutine and never overlap, even when
// enqueued concurrently from many goroutines (mimicking submit/escape/command
// callbacks firing from the TUI input goroutine).
func TestTUIHandlersSerialized(t *testing.T) {
	w := newTUIWrapper()
	defer w.stop()

	const n = 200
	var (
		inFlight int32
		ran      int32
		gidMu    sync.Mutex
		distinct = map[int]bool{}
		wg       sync.WaitGroup
	)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.enqueue(func() {
				// Detect overlap: if two jobs run at once this exceeds 1.
				if atomic.AddInt32(&inFlight, 1) != 1 {
					t.Errorf("handlers overlapped")
				}
				gidMu.Lock()
				distinct[goID()] = true
				gidMu.Unlock()
				time.Sleep(time.Millisecond)
				atomic.AddInt32(&ran, 1)
				atomic.AddInt32(&inFlight, -1)
			})
		}()
	}
	wg.Wait()

	// Wait for the queue to drain.
	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt32(&ran) < n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&ran); got != n {
		t.Fatalf("ran %d/%d jobs", got, n)
	}
	gidMu.Lock()
	defer gidMu.Unlock()
	if len(distinct) != 1 {
		t.Fatalf("handlers ran on %d goroutines, want 1: %v", len(distinct), distinct)
	}
}

// stop() releases the handler goroutine and is idempotent.
func TestTUIWrapperStopIdempotent(t *testing.T) {
	w := newTUIWrapper()
	w.stop()
	w.stop() // must not panic
	// enqueue after stop must not block.
	done := make(chan struct{})
	go func() { w.enqueue(func() {}); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("enqueue blocked after stop")
	}
}
