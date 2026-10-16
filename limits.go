package scriptling

import (
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Resource limits.
//
// Each interpreter has two limits. Each follows a process-wide default
// unless the interpreter sets its own, either when it is created (WithMemoryLimit,
// WithMaxValueBytes) or later (the methods of the same names).
// Nothing is limited until a limit is set: every default is off.
//
//   - Memory limit: the estimated memory held by the strings, bytes values and
//     containers the interpreter has created. When it goes over, the guard
//     collects garbage and, if it is still over, cancels the interpreter's
//     running calls with ErrMemoryLimitExceeded. Off by default.
//   - Value limit: the largest single string, bytes value or container one
//     operation may create, including read bodies and captured output. Going
//     over raises MemoryError in the script. Off by default.
//
// Bound running time with the context passed to EvalWithContext and the
// other *WithContext calls.

// ErrMemoryLimitExceeded is the cancellation cause of the context a call runs
// under when its interpreter goes over its memory limit. The call's error
// names it; Go builtins and libraries running inside the call can test the
// context they are given with
// errors.Is(context.Cause(ctx), scriptling.ErrMemoryLimitExceeded). The
// context the host passed in is not cancelled.
var ErrMemoryLimitExceeded = object.ErrMemoryLimitExceeded

// InheritLimit makes an interpreter's limit follow the process-wide default.
const InheritLimit = object.InheritLimit

// ResourceUsage is a snapshot of one interpreter's memory accounting.
type ResourceUsage = object.ResourceUsage

// SetMemoryLimit sets the default memory limit, in bytes, for every
// interpreter that does not set its own. 0 means no limit.
//
// The limit applies to each interpreter separately; it is not a ceiling on the
// process. Memory is estimated by sampling allocations, so it is approximate.
// Once any memory limit is set, the allocation points that report memory cost
// a few nanoseconds each for the rest of the process's life.
func SetMemoryLimit(limitBytes int64) {
	object.SetDefaultMemoryLimit(limitBytes)
}

// MemoryLimit returns the default memory limit in bytes, 0 when there is none.
func MemoryLimit() int64 {
	return object.DefaultMemoryLimit()
}

// SetMaxValueBytes sets the default value limit, in bytes, for every
// interpreter that does not set its own. 0 means no limit.
func SetMaxValueBytes(limitBytes int64) {
	object.SetDefaultMaxValueBytes(limitBytes)
}

// MaxValueBytes returns the default value limit in bytes, 0 when there is none.
func MaxValueBytes() int64 {
	return object.DefaultMaxValueBytes()
}

// Option configures an interpreter when it is created by New.
type Option func(*Scriptling)

// WithMemoryLimit gives the interpreter its own memory limit in bytes; 0 means
// no limit whatever the default.
func WithMemoryLimit(limitBytes int64) Option {
	return func(p *Scriptling) { p.SetMemoryLimit(limitBytes) }
}

// WithMaxValueBytes gives the interpreter its own value limit in bytes; 0 means
// no limit whatever the default.
func WithMaxValueBytes(limitBytes int64) Option {
	return func(p *Scriptling) { p.SetMaxValueBytes(limitBytes) }
}

// SetMemoryLimit sets this interpreter's memory limit in bytes. 0 means no
// limit; InheritLimit returns it to the process default. Set limits between
// calls: a change is enforced from the next call, and a lower limit is
// checked against memory the interpreter already holds when that call
// starts.
func (p *Scriptling) SetMemoryLimit(limitBytes int64) {
	p.guard.SetMemoryLimit(limitBytes)
}

// MemoryLimit returns the memory limit in force for this interpreter, 0 when
// there is none.
func (p *Scriptling) MemoryLimit() int64 {
	return p.guard.MemoryLimit()
}

// SetMaxValueBytes sets this interpreter's value limit in bytes. 0 means no
// limit; InheritLimit returns it to the process default.
func (p *Scriptling) SetMaxValueBytes(limitBytes int64) {
	p.guard.SetMaxValueBytes(limitBytes)
}

// MaxValueBytes returns the value limit in force for this interpreter, 0 when
// there is none.
func (p *Scriptling) MaxValueBytes() int64 {
	return p.guard.MaxValueBytes()
}

// ResourceGuard returns the interpreter's resource guard: its limits and its
// memory accounting.
func (p *Scriptling) ResourceGuard() *object.ResourceGuard {
	return p.guard
}

// ShareResourceGuard makes this interpreter use guard, so it has the same
// limits as the interpreter that owns guard and its memory counts against the
// same budget. Sandboxes and background tasks share their parent's guard this
// way. Call it before the interpreter runs; nil gives it a fresh guard that
// follows the defaults.
func (p *Scriptling) ShareResourceGuard(guard *object.ResourceGuard) {
	if guard == nil {
		guard = object.NewResourceGuard()
	}
	p.guard = guard
	p.env.SetResourceGuard(guard)
}

// ResourceUsage returns this interpreter's memory accounting. Memory is only
// measured while accounting is on, which it is once any memory limit is set.
func (p *Scriptling) ResourceUsage() ResourceUsage {
	return p.guard.Usage()
}

// endRun ends a run started with the guard's StartRun and turns a
// memory-limit cancellation into an error that names the limit.
func (p *Scriptling) endRun(run *object.Run, result object.Object) object.Object {
	cancelledForMemory := run.Cancelled()
	run.End()
	if cancelledForMemory {
		// The evaluator only saw a cancelled context; name the real reason.
		return errors.NewError("%s: the interpreter's estimated memory exceeded its limit of %d bytes", ErrMemoryLimitExceeded.Error(), p.MemoryLimit())
	}
	return result
}
