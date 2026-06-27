package gossip

import (
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/scriptling/object"
)

// goID returns the current goroutine's ID by parsing the runtime stack header.
// Test-only; used to assert which goroutine a handler executed on.
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

// pump(0) runs already-queued jobs and returns immediately.
func TestDispatcherPumpPoll(t *testing.T) {
	d := newDispatcher()
	if n := d.pump(0); n != 0 {
		t.Fatalf("empty poll: want 0, got %d", n)
	}

	got := 0
	d.post(func() { got++ })
	d.post(func() { got++ })
	if n := d.pump(0); n != 2 {
		t.Fatalf("poll: want 2 processed, got %d", n)
	}
	if got != 2 {
		t.Fatalf("handlers not run: got=%d", got)
	}
}

// Jobs run in FIFO order on the pumping goroutine.
func TestDispatcherFIFO(t *testing.T) {
	d := newDispatcher()
	var order []int
	for i := 0; i < 5; i++ {
		i := i
		d.post(func() { order = append(order, i) })
	}
	d.pump(0)
	for i, v := range order {
		if v != i {
			t.Fatalf("out of order at %d: %v", i, order)
		}
	}
}

// pump(timeout>0) blocks up to the timeout then returns 0 when nothing arrives.
func TestDispatcherPumpTimeout(t *testing.T) {
	d := newDispatcher()
	start := time.Now()
	n := d.pump(80 * time.Millisecond)
	elapsed := time.Since(start)
	if n != 0 {
		t.Fatalf("want 0 processed, got %d", n)
	}
	if elapsed < 60*time.Millisecond {
		t.Fatalf("returned too early: %v", elapsed)
	}
}

// pump(timeout>0) wakes as soon as a job is posted from another goroutine.
func TestDispatcherPumpWakesOnPost(t *testing.T) {
	d := newDispatcher()
	ran := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		d.post(func() { close(ran) })
	}()
	start := time.Now()
	n := d.pump(2 * time.Second)
	if n != 1 {
		t.Fatalf("want 1 processed, got %d", n)
	}
	select {
	case <-ran:
	default:
		t.Fatal("handler did not run")
	}
	if time.Since(start) > time.Second {
		t.Fatal("pump did not wake promptly on post")
	}
}

// call blocks the caller until the pumping goroutine runs the job, and returns
// its result. The job must execute on the pump goroutine, not the caller's.
func TestDispatcherCallResult(t *testing.T) {
	d := newDispatcher()

	var pumpGID, jobGID int
	pumpReady := make(chan struct{})
	var got object.Object
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pumpGID = goID()
		close(pumpReady)
		// Block until the call below enqueues a job.
		d.pump(2 * time.Second)
	}()

	<-pumpReady
	got = d.call(func() object.Object {
		jobGID = goID()
		return object.NewInteger(42)
	})
	wg.Wait()

	if got == nil {
		t.Fatal("call returned nil")
	}
	if iv, _ := got.AsInt(); iv != 42 {
		t.Fatalf("want 42, got %s", got.Inspect())
	}
	if jobGID != pumpGID {
		t.Fatalf("job ran on goroutine %d, expected pump goroutine %d", jobGID, pumpGID)
	}
}

// close releases goroutines blocked in pump and call.
func TestDispatcherCloseUnblocks(t *testing.T) {
	d := newDispatcher()

	done := make(chan struct{})
	go func() {
		d.pump(-1) // block until a job or close
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	d.close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pump(-1) not released by close")
	}

	// call on a closed dispatcher returns nil rather than blocking forever.
	if r := d.call(func() object.Object { return object.NewInteger(1) }); r != nil {
		t.Fatalf("call after close: want nil, got %s", r.Inspect())
	}
	// close is idempotent.
	d.close()
}
