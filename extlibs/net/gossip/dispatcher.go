package gossip

import (
	"sync"
	"time"

	"github.com/paularlott/scriptling/object"
)

// dispatcher serializes script-handler invocations that originate on the gossip
// library's internal goroutines (message receive, health/gossip routines, etc.)
// onto the single script goroutine.
//
// Handlers are enqueued by background goroutines via post/call and executed by
// the script goroutine when it calls pump (exposed to scripts as cluster.wait).
// This guarantees that script code — and therefore the interpreter environment
// tree — is only ever touched by one goroutine at a time, which the evaluator
// requires for correctness.
type dispatcher struct {
	jobs   chan func()
	closed chan struct{}
	once   sync.Once
}

// dispatcherQueueSize bounds how many pending handler jobs may queue before
// background goroutines block in post/call (back-pressure), ensuring messages
// and events are never silently dropped.
const dispatcherQueueSize = 256

func newDispatcher() *dispatcher {
	return &dispatcher{
		jobs:   make(chan func(), dispatcherQueueSize),
		closed: make(chan struct{}),
	}
}

// post enqueues a fire-and-forget handler invocation from a background
// goroutine. It blocks if the queue is full until the script pumps, or returns
// immediately if the dispatcher has been closed.
func (d *dispatcher) post(fn func()) {
	select {
	case d.jobs <- fn:
	case <-d.closed:
	}
}

// call enqueues a handler invocation and blocks until the script goroutine has
// run it via pump, returning the handler's result. Used by request/reply and
// error-returning handlers. Returns nil if the dispatcher closed first.
func (d *dispatcher) call(fn func() object.Object) object.Object {
	res := make(chan object.Object, 1)
	select {
	case d.jobs <- func() { res <- fn() }:
	case <-d.closed:
		return nil
	}
	select {
	case r := <-res:
		return r
	case <-d.closed:
		return nil
	}
}

// pump runs queued handler jobs on the calling (script) goroutine and returns
// how many ran. The timeout controls blocking when nothing is queued:
//
//	timeout < 0:  block until at least one job runs or the dispatcher closes.
//	timeout == 0: run only already-queued jobs, never block (poll).
//	timeout > 0:  if nothing is queued, block up to timeout for the first job.
func (d *dispatcher) pump(timeout time.Duration) int {
	count := 0

	// Always drain whatever is already queued.
	for {
		select {
		case fn := <-d.jobs:
			fn()
			count++
			continue
		default:
		}
		break
	}
	if count > 0 || timeout == 0 {
		return count
	}

	// Nothing was queued: wait for the first job per the timeout policy.
	if timeout < 0 {
		select {
		case fn := <-d.jobs:
			fn()
			count++
		case <-d.closed:
			return count
		}
	} else {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case fn := <-d.jobs:
			fn()
			count++
		case <-timer.C:
			return count
		case <-d.closed:
			return count
		}
	}

	// Drain any siblings that arrived while we were blocked.
	for {
		select {
		case fn := <-d.jobs:
			fn()
			count++
		default:
			return count
		}
	}
}

// close releases any goroutines blocked in post/call/pump. It is idempotent and
// safe to call from any goroutine.
func (d *dispatcher) close() {
	d.once.Do(func() { close(d.closed) })
}
