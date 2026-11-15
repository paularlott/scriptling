package libutil

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// workCheckUnits is how many units of work a WorkMeter counts between
// cancellation checks. A unit is roughly one inner-loop step (a comparison,
// a cell of a dynamic programming table), so a check costs well under 0.1% of
// the work between checks, and a cancelled call stops within a millisecond or
// so.
const workCheckUnits = 1 << 16

// WorkMeter lets a long-running library function honour its call's
// cancellation and deadline. A Go builtin runs to completion before the
// evaluator looks at the context again, so a quadratic algorithm over a large
// input would otherwise run for as long as it takes whatever the deadline.
// Count work with Add (or Tick for one unit) inside the loops; every
// workCheckUnits units it checks the context and returns the error that
// should end the call (TimeoutError or CancelledError), nil otherwise.
//
// A WorkMeter is not safe for concurrent use. The zero value and a nil
// *WorkMeter never report cancellation.
type WorkMeter struct {
	ctx      context.Context // nil once known never to stop
	done     <-chan struct{}
	run      *object.Run
	work     int64
	resolved bool
}

// NewWorkMeter returns a meter for the call ctx belongs to. The context is
// looked at only once work reaches workCheckUnits, so a short call costs
// nothing beyond the counting; a context that can never be cancelled, and
// runs under no memory limit, then makes every later check a nil test.
func NewWorkMeter(ctx context.Context) *WorkMeter {
	return &WorkMeter{ctx: ctx}
}

// Add counts n units of work and, every workCheckUnits units, checks the
// call's cancellation.
func (m *WorkMeter) Add(n int) object.Object {
	if m == nil || m.ctx == nil {
		return nil
	}
	m.work += int64(n)
	if m.work < workCheckUnits {
		return nil
	}
	m.work = 0
	return m.Check()
}

// Tick counts one unit of work.
func (m *WorkMeter) Tick() object.Object {
	return m.Add(1)
}

// Check checks the call's cancellation now: the memory-limit stop, then the
// context's deadline or cancellation.
func (m *WorkMeter) Check() object.Object {
	if m == nil || m.ctx == nil {
		return nil
	}
	if !m.resolved {
		m.resolved = true
		m.done = m.ctx.Done()
		m.run = object.RunFromContext(m.ctx)
		if m.done == nil && m.run == nil {
			m.ctx = nil
			return nil
		}
	}
	if m.run.Cancelled() {
		return errors.NewMemoryLimitStopError()
	}
	if m.done == nil {
		return nil
	}
	select {
	case <-m.done:
		return CancelledError(m.ctx)
	default:
		return nil
	}
}
