package object

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// Resource limits and memory accounting for interpreters.
//
// Every interpreter (an environment tree sharing one interpreter lock) can
// carry a ResourceGuard holding two limits:
//
//   - a memory limit: the estimated memory held by the strings, bytes,
//     container storage and objects the interpreter has created;
//   - a value limit: the largest single string, bytes value or container one
//     operation may build.
//
// Each limit either follows the process-wide default (InheritLimit) or is set
// for that interpreter alone. Every default is off: nothing is limited until a
// limit is set. Environments without a guard use the defaults for the value
// limit and are not memory-accounted.
//
// Go cannot attribute heap memory to an interpreter, so memory is estimated.
// The allocation points that let a script hold memory (string and bytes
// creation, list backing arrays, dict, set and attribute storage, and the
// objects that can reference each other outside any container: instances,
// functions, lambdas, classes, iterators, exceptions, partials, method
// wrappers and memoising caches, plus the lists and tuples that Go builtins and
// library functions build out of the objects they are handed) report their
// size; one allocation in roughly every SampleInterval bytes is sampled, in
// the same way Go's own heap profiler samples, and the sample is charged to
// the interpreter running on the current goroutine. A runtime cleanup attached
// to the sampled object credits the charge back once the object is collected.
// The estimate is unbiased: each byte has the same chance of being sampled and
// a script cannot choose which allocation is.
//
// Small scalars (ints, floats) and objects created outside those points are
// not counted directly. They are only retained in quantity through the
// objects that are.

// InheritLimit makes an interpreter's limit follow the process-wide default.
const InheritLimit int64 = -1

// SampleInterval is the mean number of allocated bytes between memory
// samples. Allocations of at least this size are always charged exactly.
const SampleInterval int64 = 64 << 10

// ErrMemoryLimitExceeded is the cancellation cause attached to a run's context
// when its interpreter goes over its memory limit.
var ErrMemoryLimitExceeded = stderrors.New("memory limit exceeded")

// TaskBytes is the memory a live background task is charged against its
// interpreter's memory limit. No allocation point reports a task's cloned
// environment, evaluator state or goroutine stack, which together measure
// about 45 KB, so without the charge a script could start tasks until the
// process ran out of memory while the guard saw almost nothing.
const TaskBytes int64 = 48 << 10

// ErrTaskLimitExceeded is the error StartTask returns when the interpreter
// already runs as many background tasks as its limit allows.
var ErrTaskLimitExceeded = stderrors.New("background task limit reached")

var (
	defaultMemoryLimit atomic.Int64
	defaultMaxValue    atomic.Int64
	defaultMaxTasks    atomic.Int64

	// accountingOn turns memory accounting on for the whole process. It is
	// sticky: once any memory limit has been configured the allocation points
	// keep reporting, which costs a few nanoseconds per string. While it is
	// off the allocation points cost one atomic load.
	accountingOn atomic.Bool

	// valueFloor is the smallest value limit configured anywhere. A size at or
	// below it is within every interpreter's limit, so CheckAlloc can accept
	// it without finding the current interpreter.
	valueFloor atomic.Int64

	// sampleInterval is SampleInterval, variable so tests can sample densely.
	sampleInterval atomic.Int64

	// fastAllocLimit is the largest size CheckAlloc accepts without finding
	// the current interpreter: within every value limit and, while memory
	// accounting is on, below the sample interval (larger sizes also need the
	// memory headroom check). Kept as one value so the check inlines.
	fastAllocLimit atomic.Int64

	// slotsFast is fastAllocLimit while accounting is off and 0 once it is
	// on, so a container literal can learn with one atomic load that it needs
	// neither a size check nor tracking (see SlotsWithin).
	slotsFast atomic.Int64

	// valueLimitsOn is set once any value limit has been configured, so the
	// result check on every builtin and method call is one atomic load
	// until then.
	valueLimitsOn atomic.Bool
)

// recomputeFastAlloc refreshes fastAllocLimit after the value floor, the
// accounting switch or the sample interval changes.
func recomputeFastAlloc() {
	limit := valueFloor.Load()
	if accountingOn.Load() {
		limit = min(limit, sampleInterval.Load()-1)
	}
	fastAllocLimit.Store(limit)
	if accountingOn.Load() {
		slotsFast.Store(0)
	} else {
		slotsFast.Store(limit)
	}
}

// SlotsWithin reports whether a container of n elements is within every
// value limit and needs no tracking, in one atomic load. When it returns
// false the caller checks with CheckElements and tracks the backing array.
func SlotsWithin(n int) bool {
	return int64(n)*int64(unsafe.Sizeof(Object(nil))) <= slotsFast.Load()
}

// enableAccounting turns memory accounting on for the rest of the process.
func enableAccounting() {
	if !accountingOn.Swap(true) {
		recomputeFastAlloc()
	}
}

func init() {
	// No limits until one is set: the floor starts above any real size.
	valueFloor.Store(math.MaxInt64)
	sampleInterval.Store(SampleInterval)
	recomputeFastAlloc()
	for i := range sampleShards {
		sampleShards[i].countdown.Store(nextSampleDistance())
	}
}

// SetDefaultMemoryLimit sets the process-wide default memory limit in bytes
// for interpreters that do not set their own. 0 (or a negative value) means
// no limit.
func SetDefaultMemoryLimit(n int64) {
	if n < 0 {
		n = 0
	}
	defaultMemoryLimit.Store(n)
	if n > 0 {
		enableAccounting()
	}
}

// DefaultMemoryLimit returns the process-wide default memory limit, 0 when
// there is none.
func DefaultMemoryLimit() int64 { return defaultMemoryLimit.Load() }

// SetDefaultMaxValueBytes sets the process-wide default for the largest single
// value one operation may create. 0 (or a negative value) means no limit.
func SetDefaultMaxValueBytes(n int64) {
	if n < 0 {
		n = 0
	}
	defaultMaxValue.Store(n)
	lowerValueFloor(n)
}

// DefaultMaxValueBytes returns the process-wide default value limit, 0 when
// there is none.
func DefaultMaxValueBytes() int64 { return defaultMaxValue.Load() }

// SetDefaultMaxBackgroundTasks sets the process-wide default for how many
// background tasks one interpreter may have queued or running at once. 0 (or a
// negative value) means no limit.
func SetDefaultMaxBackgroundTasks(n int64) {
	defaultMaxTasks.Store(max(n, 0))
}

// DefaultMaxBackgroundTasks returns the process-wide default background task
// limit, 0 when there is none.
func DefaultMaxBackgroundTasks() int64 { return defaultMaxTasks.Load() }

// MemoryAccountingEnabled reports whether memory accounting is on.
func MemoryAccountingEnabled() bool { return accountingOn.Load() }

func lowerValueFloor(n int64) {
	if n <= 0 {
		return
	}
	valueLimitsOn.Store(true)
	for {
		cur := valueFloor.Load()
		if n >= cur {
			return
		}
		if valueFloor.CompareAndSwap(cur, n) {
			recomputeFastAlloc()
			return
		}
	}
}

// ResourceUsage is a snapshot of one interpreter's resource accounting.
type ResourceUsage struct {
	// MemoryBytes is the estimated memory currently held.
	MemoryBytes int64
	// PeakMemoryBytes is the highest MemoryBytes seen.
	PeakMemoryBytes int64
	// AllocatedBytes is the estimated total allocated since creation,
	// including memory since freed.
	AllocatedBytes int64
	// MemoryLimitCancellations counts runs stopped for exceeding the memory
	// limit.
	MemoryLimitCancellations uint64
	// BackgroundTasks is the number of background tasks queued or running.
	BackgroundTasks int64
}

// ResourceGuard holds one interpreter's limits and memory accounting.
type ResourceGuard struct {
	memLimit atomic.Int64
	maxValue atomic.Int64
	maxTasks atomic.Int64

	tasks atomic.Int64 // background tasks queued or running

	used          atomic.Int64
	peak          atomic.Int64
	allocated     atomic.Int64
	cancellations atomic.Uint64

	// cancelling is set once the guard has cancelled its runs and cleared
	// when a new run starts, so a script that keeps allocating between being
	// cancelled and noticing does not trigger a collection per sample.
	cancelling atomic.Bool
	reclaimMu  sync.Mutex
	// lastForcedGC is when this guard last forced a collection that did not
	// make room (UnixNano). A script that keeps asking for values it cannot
	// have (catching MemoryError in a loop) would otherwise force a
	// stop-the-world collection on every refusal.
	lastForcedGC atomic.Int64

	// Runs are cancelled by generation: a run remembers the generation it
	// started in and is cancelled once the guard has moved past it. No run
	// holds a channel or a cancel function of its own.
	cancelGen atomic.Uint64
	active    atomic.Int64
	// causeCtx is a cancelled context carrying ErrMemoryLimitExceeded, made
	// on the first cancellation so context.Cause works on a cancelled run.
	causeOnce sync.Once
	causeCtx  context.Context
}

// NewResourceGuard returns a guard whose limits all follow the process
// defaults.
func NewResourceGuard() *ResourceGuard {
	g := &ResourceGuard{}
	g.memLimit.Store(InheritLimit)
	g.maxValue.Store(InheritLimit)
	g.maxTasks.Store(InheritLimit)
	return g
}

// SetMemoryLimit sets this interpreter's memory limit in bytes. 0 means no
// limit and InheritLimit (or any negative value) follows the process default.
//
// Set limits between calls. A changed limit is enforced for calls that start
// after the change; a call already running when no memory limit was in force
// is not registered for cancellation, and a lower limit is checked against
// memory the interpreter already holds when the next call starts.
func (g *ResourceGuard) SetMemoryLimit(n int64) {
	if n < 0 {
		n = InheritLimit
	}
	g.memLimit.Store(n)
	if n > 0 {
		enableAccounting()
	}
}

// SetMaxValueBytes sets this interpreter's value limit in bytes. 0 means no
// limit and InheritLimit (or any negative value) follows the process default.
func (g *ResourceGuard) SetMaxValueBytes(n int64) {
	if n < 0 {
		n = InheritLimit
	}
	g.maxValue.Store(n)
	lowerValueFloor(n)
}

// SetMaxBackgroundTasks sets how many background tasks this interpreter may
// have queued or running at once. 0 means no limit and InheritLimit (or any
// negative value) follows the process default.
func (g *ResourceGuard) SetMaxBackgroundTasks(n int64) {
	if n < 0 {
		n = InheritLimit
	}
	g.maxTasks.Store(n)
}

// MaxBackgroundTasks returns the background task limit in force, 0 when there
// is none.
func (g *ResourceGuard) MaxBackgroundTasks() int64 {
	if g == nil {
		return defaultMaxTasks.Load()
	}
	if v := g.maxTasks.Load(); v >= 0 {
		return v
	}
	return defaultMaxTasks.Load()
}

// StartTask accounts for a background task the interpreter is about to queue
// or start, and returns the function that ends it, to be called exactly once
// when the task is no longer queued or running. The task counts against the
// background task limit, and TaskBytes is held against the memory limit for
// its lifetime. It refuses with ErrTaskLimitExceeded when the task limit is
// reached and with ErrMemoryLimitExceeded when the charge takes the
// interpreter over its memory limit (its runs are then cancelled, as for any
// allocation over the limit).
func (g *ResourceGuard) StartTask() (end func(), err error) {
	if g == nil {
		return func() {}, nil
	}
	n := g.tasks.Add(1)
	if lim := g.MaxBackgroundTasks(); lim > 0 && n > lim {
		g.tasks.Add(-1)
		return nil, fmt.Errorf("%w: %d tasks are queued or running, the limit is %d", ErrTaskLimitExceeded, n-1, lim)
	}
	var charged int64
	if lim := g.MemoryLimit(); lim > 0 {
		g.charge(TaskBytes)
		charged = TaskBytes
		if g.used.Load() > lim {
			g.used.Add(-charged)
			g.tasks.Add(-1)
			return nil, ErrMemoryLimitExceeded
		}
	}
	return func() {
		g.used.Add(-charged)
		g.tasks.Add(-1)
	}, nil
}

// needsBinding reports whether goroutines running this interpreter must be
// bound to it: once memory accounting or any value limit exists in the
// process. Binding an interpreter with no limits of its own matters too,
// or a nested interpreter would be checked against the guard the goroutine
// was bound to before. With no limits anywhere the lock skips the binding
// and costs nothing extra.
func (g *ResourceGuard) needsBinding() bool {
	return accountingOn.Load() || valueLimitsOn.Load()
}

// CopyLimits gives g the same limit settings as from: explicit limits are
// copied and limits following the defaults keep following them. Accounting is
// not copied.
func (g *ResourceGuard) CopyLimits(from *ResourceGuard) {
	if from == nil {
		return
	}
	g.SetMemoryLimit(from.memLimit.Load())
	g.SetMaxValueBytes(from.maxValue.Load())
	g.SetMaxBackgroundTasks(from.maxTasks.Load())
}

// MemoryLimit returns the memory limit in force, 0 when there is none.
func (g *ResourceGuard) MemoryLimit() int64 {
	if g == nil {
		return defaultMemoryLimit.Load()
	}
	if v := g.memLimit.Load(); v >= 0 {
		return v
	}
	return defaultMemoryLimit.Load()
}

// MaxValueBytes returns the value limit in force, 0 when there is none.
func (g *ResourceGuard) MaxValueBytes() int64 {
	if g == nil {
		return defaultMaxValue.Load()
	}
	if v := g.maxValue.Load(); v >= 0 {
		return v
	}
	return defaultMaxValue.Load()
}

// Usage returns a snapshot of the guard's accounting.
func (g *ResourceGuard) Usage() ResourceUsage {
	return ResourceUsage{
		MemoryBytes:              max(g.used.Load(), 0),
		PeakMemoryBytes:          g.peak.Load(),
		AllocatedBytes:           g.allocated.Load(),
		MemoryLimitCancellations: g.cancellations.Load(),
		BackgroundTasks:          g.tasks.Load(),
	}
}

// StartRun registers a top-level run so the guard can cancel it if the
// interpreter goes over its memory limit. It returns the context the run must
// use and the run to end when it is over. With no memory limit in force it
// returns ctx unchanged and a nil run, and costs nothing more.
func (g *ResourceGuard) StartRun(ctx context.Context) (context.Context, *Run) {
	if g == nil || g.MemoryLimit() <= 0 {
		return ctx, nil
	}
	g.cancelling.Store(false)
	r := &Run{parent: ctx, guard: g, gen: g.cancelGen.Load()}
	g.active.Add(1)
	// Memory retained from earlier calls may already be over the limit (it
	// was lowered, or set after the memory was allocated). A call that never
	// allocates would not notice, so check before it starts. Rate limited
	// like every forced collection: a host calling an interpreter whose
	// globals sit over its limit must not pay a collection per call. Memory
	// the host has released is seen by the next check after the interval.
	if lim := g.MemoryLimit(); g.used.Load() > lim && !g.reclaim(func() bool { return g.used.Load() <= lim }) {
		r.cancelled.Store(true)
		g.cancellations.Add(1)
	}
	return r, r
}

// Run is one top-level execution registered on a guard, and the context the
// execution runs under: the context it was started from plus the guard's
// cancellation. The guard cancels a run by moving its generation on, which
// the evaluator sees at its next periodic check and Err reports; Done is the
// parent's channel, so work blocked on it finishes its current operation
// before it is stopped. That keeps a run to one small allocation.
type Run struct {
	parent    context.Context
	guard     *ResourceGuard
	gen       uint64
	cancelled atomic.Bool
	ended     atomic.Bool
}

// runKey is the context key under which a run finds itself.
type runKey struct{}

// Deadline is the parent's.
func (r *Run) Deadline() (time.Time, bool) { return r.parent.Deadline() }

// Done is the parent's channel: the guard's cancellation is reported by Err
// and seen by the evaluator's periodic check, not by a channel.
func (r *Run) Done() <-chan struct{} { return r.parent.Done() }

// Err reports the parent's error, or context.Canceled once the guard has
// cancelled the run or the run has ended.
func (r *Run) Err() error {
	if err := r.parent.Err(); err != nil {
		return err
	}
	if r.Cancelled() || r.ended.Load() {
		return context.Canceled
	}
	return nil
}

// Value finds the run itself, the memory-limit cause once the run is
// cancelled for it, and otherwise the parent's values.
func (r *Run) Value(key any) any {
	if key == (runKey{}) {
		return r
	}
	if r.Cancelled() {
		if v := r.guard.memoryLimitCause().Value(key); v != nil {
			return v
		}
	}
	return r.parent.Value(key)
}

// Cancelled reports whether the guard has cancelled the run for its memory
// limit.
func (r *Run) Cancelled() bool {
	if r == nil {
		return false
	}
	return r.cancelled.Load() || r.guard.cancelGen.Load() != r.gen
}

// End ends the run. A nil run (no memory limit) ends nothing.
func (r *Run) End() {
	if r == nil {
		return
	}
	if !r.ended.Swap(true) {
		r.guard.active.Add(-1)
	}
}

// RunFromContext returns the run ctx belongs to, nil when it belongs to none.
func RunFromContext(ctx context.Context) *Run {
	r, _ := ctx.Value(runKey{}).(*Run)
	return r
}

// memoryLimitCause returns a cancelled context whose cause is
// ErrMemoryLimitExceeded, so context.Cause on a cancelled run reports it.
func (g *ResourceGuard) memoryLimitCause() context.Context {
	g.causeOnce.Do(func() {
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(ErrMemoryLimitExceeded)
		g.causeCtx = ctx
	})
	return g.causeCtx
}

// fits reports whether n more bytes keep the estimate within lim, without
// the overflow that adding them to the estimate could cause.
func (g *ResourceGuard) fits(n, lim int64) bool {
	used := max(g.used.Load(), 0)
	return used <= lim && n <= lim-used
}

// DetachRun returns ctx without the cancellation of any run it belongs to: a
// run's context is cancelled when the run ends, so work that should outlive
// it (a fire-and-forget background task) starts from the context the
// outermost run was started from. Its values and the host's cancellation are
// kept; values added inside the run are not.
func DetachRun(ctx context.Context) context.Context {
	for {
		r := RunFromContext(ctx)
		if r == nil {
			return ctx
		}
		ctx = r.parent
	}
}

// noRun is the release function of a binding that was not made.
func noRun() {}

// cancelRuns stops every run registered on the guard.
func (g *ResourceGuard) cancelRuns() {
	g.cancelling.Store(true)
	g.cancelGen.Add(1)
	g.cancellations.Add(uint64(max(g.active.Load(), 0)))
}

func (g *ResourceGuard) charge(w int64) {
	g.allocated.Add(w)
	used := g.used.Add(w)
	for {
		peak := g.peak.Load()
		if used <= peak || g.peak.CompareAndSwap(peak, used) {
			break
		}
	}
	if lim := g.MemoryLimit(); lim > 0 && used > lim && !g.cancelling.Load() {
		g.overLimit(lim)
	}
}

// overLimit runs when a charge takes the guard over its limit. Much of the
// charge may be garbage the collector has not reached, so it collects first
// and only cancels the interpreter's runs if the estimate is still over.
func (g *ResourceGuard) overLimit(lim int64) {
	if !g.reclaimMu.TryLock() {
		return // another goroutine of this interpreter is already deciding
	}
	defer g.reclaimMu.Unlock()
	if g.reclaim(func() bool { return g.used.Load() <= lim }) {
		return
	}
	g.cancelRuns()
}

// chargeRecord is the argument of a sampled object's cleanup.
type chargeRecord struct {
	g *ResourceGuard
	w int64
}

func releaseCharge(c chargeRecord) {
	c.g.used.Add(-c.w)
}

// reclaim tuning: how long to wait for cleanups after a forced collection,
// and the least time between forced collections for one guard once a
// collection has proved unhelpful.
var (
	reclaimPolls        = 20
	reclaimPollInterval = 500 * time.Microsecond
	forcedGCMu          sync.Mutex
	forcedGCMinInterval = 100 * time.Millisecond
)

// reclaim is reclaimUntil for one guard, forcing at most one unhelpful
// collection per forcedGCMinInterval: a refusal soon after one that a
// collection did not fix is answered without another collection, so a
// script catching MemoryError in a loop cannot make the process collect
// continuously. A refusal within the interval may be spurious when the
// script has released memory since; the next attempt after it is not.
func (g *ResourceGuard) reclaim(ok func() bool) bool {
	if ok() {
		return true
	}
	if last := g.lastForcedGC.Load(); last != 0 && time.Since(time.Unix(0, last)) < forcedGCMinInterval {
		return false
	}
	if reclaimUntil(ok) {
		g.lastForcedGC.Store(0)
		return true
	}
	g.lastForcedGC.Store(time.Now().UnixNano())
	return false
}

// reclaimUntil forces a garbage collection unless ok already holds, then waits
// briefly for the cleanups it queued to credit their charges back. It reports
// whether ok holds afterwards. Collections are serialised so interpreters that
// go over at the same time share one.
func reclaimUntil(ok func() bool) bool {
	forcedGCMu.Lock()
	if !ok() {
		runtime.GC()
	}
	forcedGCMu.Unlock()
	for range reclaimPolls {
		if ok() {
			return true
		}
		time.Sleep(reclaimPollInterval)
	}
	return ok()
}

// ---------------------------------------------------------------------------
// Which interpreter is running on this goroutine.
//
// The interpreter lock binds the acquiring goroutine to its guard and restores
// the previous binding on release, so nested interpreters on one goroutine
// (a builtin running a sandboxed script) charge the innermost one. Lookups are
// only made on slow paths: a memory sample, or a size check above the floor.

const guardShardCount = 64

type guardShard struct {
	mu sync.Mutex
	m  map[int64]*ResourceGuard
	_  [40]byte // keep shards on separate cache lines
}

var guardShards [guardShardCount]guardShard

func shardIndex(id int64) uint {
	return uint((uint64(id) * 0x9E3779B97F4A7C15) >> 58) // top 6 bits: 0..63
}

// bindGuard makes g the guard for goroutine id and returns the previous one.
func bindGuard(id int64, g *ResourceGuard) *ResourceGuard {
	s := &guardShards[shardIndex(id)]
	s.mu.Lock()
	if s.m == nil {
		s.m = make(map[int64]*ResourceGuard)
	}
	prev := s.m[id]
	s.m[id] = g
	s.mu.Unlock()
	return prev
}

// unbindGuard restores the binding bindGuard replaced.
func unbindGuard(id int64, prev *ResourceGuard) {
	s := &guardShards[shardIndex(id)]
	s.mu.Lock()
	if prev == nil {
		delete(s.m, id)
	} else {
		s.m[id] = prev
	}
	s.mu.Unlock()
}

// CurrentGuard returns the guard of the interpreter running on the calling
// goroutine, or nil when none is.
func CurrentGuard() *ResourceGuard {
	id := goid()
	s := &guardShards[shardIndex(id)]
	s.mu.Lock()
	g := s.m[id]
	s.mu.Unlock()
	return g
}

// BindGuard makes the calling goroutine work for the interpreter that owns
// guard until release is called: its allocations are charged to that
// interpreter and its size checks use its limits. Use it in goroutines a
// builtin starts to do work for a script (parallel requests, for example).
// A nil guard binds nothing. release must be called on the same goroutine.
func BindGuard(guard *ResourceGuard) (release func()) {
	if guard == nil {
		return noRun
	}
	id := goid()
	prev := bindGuard(id, guard)
	return func() { unbindGuard(id, prev) }
}

// MaxValueBytes returns the value limit for the interpreter running on the
// calling goroutine (the process default when none is), 0 for no limit.
// Use it to bound reads whose size is not known up front.
func MaxValueBytes() int64 {
	return CurrentGuard().MaxValueBytes()
}

// ---------------------------------------------------------------------------
// Allocation reporting.

type sampleShard struct {
	countdown atomic.Int64
	_         [56]byte
}

var sampleShards [guardShardCount]sampleShard

// nextSampleDistance draws the bytes until the next sample from an
// exponential distribution, so samples land on uniformly random bytes.
func nextSampleDistance() int64 {
	d := int64(rand.ExpFloat64() * float64(sampleInterval.Load()))
	return max(d, 1)
}

// sampleWeight is the unbiased weight of a sampled allocation of size bytes:
// size divided by the probability that a sample point landed inside it.
func sampleWeight(size int64) int64 {
	r := float64(sampleInterval.Load())
	p := -math.Expm1(-float64(size) / r)
	if p <= 0 {
		return int64(r)
	}
	return int64(float64(size) / p)
}

// TrackAlloc reports that p was just allocated, holding size bytes. When
// memory accounting is on it may be sampled and charged to the interpreter on
// the calling goroutine until p is collected. It costs one atomic load while
// accounting is off. p must point into a heap allocation.
func TrackAlloc[T any](p *T, size int64) {
	if accountingOn.Load() {
		trackAlloc(p, size)
	}
}

// TrackSlice reports a newly allocated backing array: s must be the slice
// returned by the allocation (make or a growing append), with every element
// slot counting as allocated. Call it only when the backing array is new, for
// example when an append changed the capacity, and only for Go heap memory:
// a slice over C or mapped memory cannot carry the credit.
func TrackSlice[E any](s []E) {
	if accountingOn.Load() && cap(s) > 0 {
		var zero E
		trackAlloc(&s[:1][0], int64(cap(s))*int64(unsafe.Sizeof(zero)))
	}
}

// TrackMap reports a newly made Go map holding about size bytes, charging it
// to the interpreter on the calling goroutine until the map is collected. A
// map has no pointer of its own to attach the credit to, so the cleanup is
// attached to its header allocation.
func TrackMap(m any, size int64) {
	if !accountingOn.Load() || size <= 0 {
		return
	}
	v := reflect.ValueOf(m)
	if v.Kind() != reflect.Map || v.IsNil() {
		return
	}
	trackAlloc((*byte)(v.UnsafePointer()), size)
}

// TrackSliceWeighted is TrackSlice charging perSlot bytes for every slot, for
// backing arrays that stand in for larger hidden storage (a map's buckets).
func TrackSliceWeighted[E any](s []E, perSlot int64) {
	if accountingOn.Load() && cap(s) > 0 {
		trackAlloc(&s[:1][0], int64(cap(s))*perSlot)
	}
}

// trackString and trackBytes keep the constructors small enough to inline:
// they only call out when accounting is on.
//
//go:noinline
func trackString(str *String) {
	trackAlloc(str, int64(len(str.value))+stringObjectOverhead)
}

//go:noinline
func trackBytes(b *Bytes) {
	trackAlloc(b, int64(len(b.value))+bytesObjectOverhead)
}

func trackAlloc[T any](p *T, size int64) {
	if size <= 0 {
		return
	}
	interval := sampleInterval.Load()
	if size >= interval {
		chargeSample(p, size)
		return
	}
	s := &sampleShards[shardIndex(sampleGoid())]
	if s.countdown.Add(-size) > 0 {
		return
	}
	s.countdown.Store(nextSampleDistance())
	chargeSample(p, sampleWeight(size))
}

func chargeSample[T any](p *T, w int64) {
	g := CurrentGuard()
	if g == nil {
		return
	}
	runtime.AddCleanup(p, releaseCharge, chargeRecord{g: g, w: w})
	g.charge(w)
}

// ---------------------------------------------------------------------------
// Size checks.

// CheckAlloc reports whether the interpreter on the calling goroutine may
// create a value of n bytes: within its value limit and, when it has a
// memory limit, within the room left (after a collection, if needed). It
// returns nil when it may, or a MemoryError to hand back to the script. Call
// it before a large allocation whose size depends on script input.
func CheckAlloc(n int64) Object {
	if n <= fastAllocLimit.Load() {
		return nil
	}
	return checkAllocSlow("value", n)
}

// CheckAllocFor is CheckAlloc naming what is being created ("joined string",
// "http response body") in the error.
func CheckAllocFor(what string, n int64) Object {
	if n <= fastAllocLimit.Load() {
		return nil
	}
	return checkAllocSlow(what, n)
}

// CheckElements is CheckAlloc for a container of n element slots.
func CheckElements(n int64) Object {
	if n <= 0 {
		return nil
	}
	const slot = int64(unsafe.Sizeof(Object(nil)))
	if n > math.MaxInt64/slot {
		return NewMemoryError("container too large: %d elements", n)
	}
	return CheckAllocFor("container", n*slot)
}

// AllocWithin reports whether an allocation of n bytes is within every limit
// without looking the interpreter up: one atomic load, for callers that
// want to do cheaper work before the full check.
func AllocWithin(n int64) bool {
	return n <= fastAllocLimit.Load()
}

// CheckSlots applies only the value limit to a container of n element slots,
// for a container that already exists and is about to grow by one: its
// memory is already counted, so only the growth is checked for room, by the
// caller.
func CheckSlots(n int64) Object {
	const slot = int64(unsafe.Sizeof(Object(nil)))
	if n > math.MaxInt64/slot {
		return NewMemoryError("container too large: %d elements", n)
	}
	if maxValue := CurrentGuard().MaxValueBytes(); maxValue > 0 && n*slot > maxValue {
		return NewMemoryError("container too large: %d bytes is more than the %d byte limit", n*slot, maxValue)
	}
	return nil
}

// CheckRoom checks only the memory headroom for n bytes about to be held at
// once (say, a value and the buffer it is built from), without applying the
// value limit. CheckAllocFor covers both for a single value.
func CheckRoom(what string, n int64) Object {
	if n < sampleInterval.Load() || !accountingOn.Load() {
		return nil
	}
	return checkRoom(CurrentGuard(), what, n)
}

func checkAllocSlow(what string, n int64) Object {
	g := CurrentGuard()
	if maxValue := g.MaxValueBytes(); maxValue > 0 && n > maxValue {
		return NewMemoryError("%s too large: %d bytes is more than the %d byte limit", what, n, maxValue)
	}
	return checkRoom(g, what, n)
}

// checkRoom is the memory headroom part of the size checks.
func checkRoom(g *ResourceGuard, what string, n int64) Object {
	// Smaller allocations are left to the sampled accounting, so whether one
	// is refused never depends on another interpreter's value limit having
	// sent it down this path.
	if g == nil || n < sampleInterval.Load() {
		return nil
	}
	lim := g.MemoryLimit()
	if lim <= 0 {
		return nil
	}
	if n > lim {
		return NewMemoryError("%s: %s of %d bytes is more than the %d byte limit", ErrMemoryLimitExceeded.Error(), what, n, lim)
	}
	if g.fits(n, lim) {
		return nil
	}
	if g.reclaim(func() bool { return g.fits(n, lim) }) {
		return nil
	}
	return NewMemoryError("%s: no room for a %s of %d bytes, %d of the %d byte limit is in use", ErrMemoryLimitExceeded.Error(), what, n, max(g.used.Load(), 0), lim)
}

// NewMemoryError returns an error that `except MemoryError` catches.
func NewMemoryError(format string, args ...any) *Error {
	return &Error{Message: fmt.Sprintf(format, args...), ExceptionType: ExceptionTypeMemoryError}
}

// ReadAllLimited reads r to the end within the limits of the interpreter on
// the calling goroutine: the result may not pass its value limit, and its
// buffer must fit in its memory limit (after a collection, if needed). The
// buffer is charged to the interpreter as it grows. what names the data in
// the error, which is a MemoryError object; err reports read failures.
func ReadAllLimited(r io.Reader, what string) (data []byte, limitErr Object, err error) {
	g := CurrentGuard()
	maxValue := g.MaxValueBytes()
	if maxValue == math.MaxInt64 {
		maxValue = 0 // too large to need checking (and to read one past)
	}
	data = make([]byte, 0, 512)
	for {
		if len(data) == cap(data) {
			next := int64(cap(data)) * 2
			if maxValue > 0 {
				next = min(next, maxValue+1) // one past the limit shows it is over
			}
			if errObj := CheckRoom(what, next); errObj != nil {
				return nil, errObj, nil
			}
			grown := make([]byte, len(data), next)
			copy(grown, data)
			data = grown
			TrackSlice(data)
		}
		n, readErr := r.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if maxValue > 0 && int64(len(data)) > maxValue {
			return nil, NewMemoryError("%s too large: more than the %d byte limit", what, maxValue), nil
		}
		if readErr == io.EOF {
			return data, nil, nil
		}
		if readErr != nil {
			return data, nil, readErr
		}
	}
}

// ReadFileLimited reads the named file, refusing a file larger than the value
// limit of the interpreter on the calling goroutine (see ReadAllLimited).
func ReadFileLimited(path string) (data []byte, limitErr Object, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	if info, statErr := f.Stat(); statErr == nil && info.Mode().IsRegular() {
		if limit := MaxValueBytes(); limit > 0 && info.Size() > limit {
			return nil, NewMemoryError("file too large: %d bytes is more than the %d byte limit", info.Size(), limit), nil
		}
	}
	return ReadAllLimited(f, "file")
}

// ErrValueTooLarge matches (with errors.Is) the errors ReadAllLimitedErr and
// ReadFileLimitedErr return for data over the value limit.
var ErrValueTooLarge = stderrors.New("value too large")

// valueTooLargeError carries the MemoryError message as a Go error.
type valueTooLargeError struct{ msg string }

func (e *valueTooLargeError) Error() string        { return e.msg }
func (e *valueTooLargeError) Is(target error) bool { return target == ErrValueTooLarge }

// ReadAllLimitedErr is ReadAllLimited for code that reports Go errors: data
// over the value limit is an error matching ErrValueTooLarge.
func ReadAllLimitedErr(r io.Reader, what string) ([]byte, error) {
	data, tooLarge, err := ReadAllLimited(r, what)
	if tooLarge != nil {
		return nil, &valueTooLargeError{msg: tooLarge.(*Error).Message}
	}
	return data, err
}

// ReadFileLimitedErr is ReadFileLimited for code that reports Go errors.
func ReadFileLimitedErr(path string) ([]byte, error) {
	data, tooLarge, err := ReadFileLimited(path)
	if tooLarge != nil {
		return nil, &valueTooLargeError{msg: tooLarge.(*Error).Message}
	}
	return data, err
}

// FileEquals reports whether the file at path holds exactly data, reading it
// only when its size matches, so comparing against a huge file costs nothing.
func FileEquals(path string, data []byte) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != int64(len(data)) {
		return false
	}
	existing, err := os.ReadFile(path)
	return err == nil && bytes.Equal(existing, data)
}

// detachWaste is how many bytes a substring may keep alive beyond its own
// before NewSubstring copies it out of its source.
const detachWaste = 256

// NewSubstring returns a String for sub, a substring of src. A Go substring
// shares its source's memory, so a short piece of a long string would keep
// the whole source alive while the memory accounting charged only the piece.
// While accounting is on, a substring that would hold on to at least
// detachWaste bytes more than it uses is copied; otherwise this is NewString.
func NewSubstring(sub, src string) *String {
	if accountingOn.Load() && len(src)-len(sub) >= detachWaste {
		sub = strings.Clone(sub)
	}
	return NewString(sub)
}

// byteStrings holds every one-byte string, so a character taken from a long
// string need not share (and keep alive) the long string's memory.
var byteStrings = func() (t [256]string) {
	for i := range t {
		t[i] = string([]byte{byte(i)})
	}
	return t
}()

// ByteString returns the one-byte string b without allocating.
func ByteString(b byte) string { return byteStrings[b] }

// GrowthRoom estimates the memory a buffer of capacity oldCap holds while it
// grows to hold size bytes: the old backing array stays until the copy is
// done, and Go allocates at least double the old capacity.
func GrowthRoom(oldCap, size int) int64 {
	return int64(oldCap) + int64(max(2*oldCap, size))
}

// CheckValue applies the value limit of the interpreter on the calling
// goroutine to a value an operation has just produced: a string, bytes value,
// list, tuple, dict or set larger than the limit gives a MemoryError,
// anything else nil. Containers count one element slot per entry.
// Operations that know their result size up front should use CheckAlloc
// instead; this is the backstop for those that do not (encoders, library
// functions), which can only overshoot by the factor one call expands by.
func CheckValue(obj Object) Object {
	if !valueLimitsOn.Load() {
		return nil
	}
	return checkValueSlow(obj)
}

// CheckString is CheckValue for a string that is evaluated often, such as a
// literal: inlinable, so with no value limit set it costs one atomic load.
func CheckString(s *String) Object {
	if !valueLimitsOn.Load() {
		return nil
	}
	return checkValueSlow(s)
}

func checkValueSlow(obj Object) Object {
	var n int64
	switch v := obj.(type) {
	case *String:
		n = int64(len(v.value))
	case *Bytes:
		n = int64(len(v.value))
	case *List:
		n = int64(len(v.Elements)) * int64(unsafe.Sizeof(Object(nil)))
	case *Tuple:
		n = int64(len(v.Elements)) * int64(unsafe.Sizeof(Object(nil)))
	case *Dict:
		n = int64(len(v.Pairs)) * int64(unsafe.Sizeof(Object(nil)))
	case *Set:
		n = int64(len(v.Elements)) * int64(unsafe.Sizeof(Object(nil)))
	default:
		return nil
	}
	if n <= valueFloor.Load() {
		return nil
	}
	if maxValue := CurrentGuard().MaxValueBytes(); maxValue > 0 && n > maxValue {
		return NewMemoryError("result too large: %d bytes is more than the %d byte limit", n, maxValue)
	}
	return nil
}
