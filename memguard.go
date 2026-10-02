package scriptling

import (
	"context"
	stderrors "errors"
	"runtime"
	"runtime/metrics"
	"sync"
	"sync/atomic"
	"time"
)

// ErrMemoryLimitExceeded is the cancellation cause attached to a script's
// context when the memory guard stops it. Hosts can test for it with
// errors.Is(context.Cause(ctx), scriptling.ErrMemoryLimitExceeded).
var ErrMemoryLimitExceeded = stderrors.New("memory limit exceeded")

// DefaultMemoryLimitCheckInterval is how often the memory guard samples the
// heap while a limit is set.
const DefaultMemoryLimitCheckInterval = 250 * time.Millisecond

// heapObjectsMetric is the live-plus-unswept heap: every byte currently held
// by heap objects. It is the cheapest honest signal of "how much memory are
// scripts holding right now", and runtime/metrics reads it without stopping
// the world.
const heapObjectsMetric = "/memory/classes/heap/objects:bytes"

// MemoryLimitStats is a snapshot of the memory guard.
type MemoryLimitStats struct {
	Limit         int64  // configured ceiling in bytes; 0 means disabled
	HeapBytes     int64  // heap object bytes at the most recent check
	Running       int    // scripts currently being evaluated
	Cancellations uint64 // scripts the guard has stopped since process start
}

// memoryGuard is the process-wide memory watchdog.
//
// Go cannot attribute heap memory to one interpreter without accounting on
// every allocation, which would tax the evaluator's hot path. The guard
// therefore works at process level and costs nothing per instruction: each
// EvalWithContext registers itself (one map insert) while a limit is set, a
// single goroutine samples the heap on a timer, and when the heap stays above
// the limit after a collection the most recently started script is cancelled.
// Shedding newest-first protects long-running work that was already within
// budget from a newcomer that is not.
type memoryGuard struct {
	limit    atomic.Int64 // bytes; 0 disables
	interval atomic.Int64 // check interval in nanoseconds
	heap     atomic.Int64 // last sampled heap bytes
	cancels  atomic.Uint64

	mu     sync.Mutex
	runs   map[uint64]*guardedRun
	nextID uint64
	stop   chan struct{} // non-nil while the watchdog goroutine is running
}

// guardedRun is one in-flight EvalWithContext.
type guardedRun struct {
	id     uint64
	cancel context.CancelCauseFunc
}

var memGuard = newMemoryGuard()

func newMemoryGuard() *memoryGuard {
	g := &memoryGuard{runs: make(map[uint64]*guardedRun)}
	g.interval.Store(int64(DefaultMemoryLimitCheckInterval))
	return g
}

// SetMemoryLimit sets a process-wide ceiling, in bytes, on heap memory held by
// objects. While the heap stays above the ceiling after a garbage collection,
// the most recently started script is cancelled on each check (every
// DefaultMemoryLimitCheckInterval) and its evaluation returns an error that
// names the limit. A limit of 0 disables the guard; negative values are
// treated as 0.
//
// The ceiling applies to the whole process, not to one interpreter, so set it
// well above the host's own baseline heap: a limit below the baseline cancels
// every script as soon as it starts. Set it once at startup; changing it
// later is safe. The guard adds no per-instruction cost to scripts, and while
// disabled its only cost is one atomic load per evaluation.
func SetMemoryLimit(limitBytes int64) {
	if limitBytes < 0 {
		limitBytes = 0
	}
	memGuard.setLimit(limitBytes)
}

// MemoryLimit returns the configured memory ceiling in bytes, 0 when disabled.
func MemoryLimit() int64 {
	return memGuard.limit.Load()
}

// GetMemoryLimitStats returns a snapshot of the memory guard.
func GetMemoryLimitStats() MemoryLimitStats {
	return memGuard.snapshot()
}

// setMemoryGuardInterval changes how often the heap is sampled. Tests use it
// to make the guard react quickly; hosts should not need it.
func setMemoryGuardInterval(d time.Duration) {
	if d <= 0 {
		d = DefaultMemoryLimitCheckInterval
	}
	memGuard.interval.Store(int64(d))
}

func (g *memoryGuard) setLimit(limit int64) {
	g.limit.Store(limit)
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case limit > 0 && g.stop == nil:
		g.stop = make(chan struct{})
		go g.watch(g.stop)
	case limit == 0 && g.stop != nil:
		close(g.stop)
		g.stop = nil
	}
}

// track registers an evaluation with the guard when a limit is set. It
// returns the context the script must run under and a release function the
// caller defers. With no limit it returns ctx unchanged and a no-op release,
// costing one atomic load.
func (g *memoryGuard) track(ctx context.Context) (context.Context, func()) {
	if g.limit.Load() == 0 {
		return ctx, func() {}
	}
	ctx, cancel := context.WithCancelCause(ctx)
	g.mu.Lock()
	g.nextID++
	id := g.nextID
	g.runs[id] = &guardedRun{id: id, cancel: cancel}
	g.mu.Unlock()
	return ctx, func() {
		g.mu.Lock()
		delete(g.runs, id)
		g.mu.Unlock()
		cancel(nil)
	}
}

// watch samples the heap until stop is closed.
func (g *memoryGuard) watch(stop chan struct{}) {
	timer := time.NewTimer(time.Duration(g.interval.Load()))
	defer timer.Stop()
	for {
		select {
		case <-stop:
			return
		case <-timer.C:
		}
		g.check()
		timer.Reset(time.Duration(g.interval.Load()))
	}
}

// check is one watchdog tick: sample, collect if over, cancel if still over.
func (g *memoryGuard) check() {
	limit := g.limit.Load()
	if limit == 0 {
		return
	}
	heap := readHeapObjectBytes()
	g.heap.Store(heap)
	if heap <= limit {
		return
	}
	// Above the ceiling. Much of the heap may be garbage the collector has not
	// reached yet, so collect before judging; a forced collection is only paid
	// while under pressure.
	runtime.GC()
	heap = readHeapObjectBytes()
	g.heap.Store(heap)
	if heap <= limit {
		return
	}
	if g.cancelNewest() {
		g.cancels.Add(1)
	}
}

// cancelNewest stops the most recently started script, if any is running.
func (g *memoryGuard) cancelNewest() bool {
	g.mu.Lock()
	var newest *guardedRun
	for _, r := range g.runs {
		if newest == nil || r.id > newest.id {
			newest = r
		}
	}
	if newest != nil {
		delete(g.runs, newest.id)
	}
	g.mu.Unlock()
	if newest == nil {
		return false
	}
	newest.cancel(ErrMemoryLimitExceeded)
	return true
}

func (g *memoryGuard) snapshot() MemoryLimitStats {
	g.mu.Lock()
	running := len(g.runs)
	g.mu.Unlock()
	return MemoryLimitStats{
		Limit:         g.limit.Load(),
		HeapBytes:     g.heap.Load(),
		Running:       running,
		Cancellations: g.cancels.Load(),
	}
}

// readHeapObjectBytes returns the bytes currently held by heap objects.
func readHeapObjectBytes() int64 {
	sample := []metrics.Sample{{Name: heapObjectsMetric}}
	metrics.Read(sample)
	if sample[0].Value.Kind() != metrics.KindUint64 {
		return 0
	}
	return int64(sample[0].Value.Uint64())
}
