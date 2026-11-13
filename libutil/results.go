// Package libutil holds the small helpers shared by Scriptling's standard
// and extension libraries. It depends only on the object and errors packages,
// so a library can use it without linking any other library.
package libutil

import (
	"context"
	"sync/atomic"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Bounded result building for libraries whose input is not a script value
// (file contents, directory entries, parsed records). Such a library used to
// build its whole result before returning, so neither limit could stop it:
// the value limit never saw the data, and a memory-limit cancellation only
// takes effect when the evaluator next looks at the context. The helpers here
// check as the result grows, so an over-limit call stops building instead of
// finishing first.

// resultCheckEvery is how many items are added between cancellation checks.
const resultCheckEvery = 256

// ResultList builds a list for a script, checking each growth of its backing
// array against the value limit and the memory left, and the call's
// cancellation every resultCheckEvery items.
type ResultList struct {
	ctx   context.Context
	elems []object.Object
	added int
}

// NewResultList starts a list expected to hold about hint items. The hint
// is checked up front; a hint too large to admit is an error before anything
// is built.
func NewResultList(ctx context.Context, hint int) (*ResultList, object.Object) {
	if errObj := object.CheckElements(int64(hint)); errObj != nil {
		return nil, errObj
	}
	elems := make([]object.Object, 0, min(hint, 1<<16))
	object.TrackSlice(elems)
	return &ResultList{ctx: ctx, elems: elems}, nil
}

// Add appends v, or returns the error that stops the build.
func (r *ResultList) Add(v object.Object) object.Object {
	if len(r.elems) == cap(r.elems) {
		if errObj := object.CheckElements(int64(len(r.elems)) + 1); errObj != nil {
			return errObj
		}
		r.elems = append(r.elems, v)
		object.TrackSlice(r.elems)
	} else {
		r.elems = append(r.elems, v)
	}
	if r.added++; r.added%resultCheckEvery == 0 {
		return CancelledError(r.ctx)
	}
	return nil
}

// List returns the built list.
func (r *ResultList) List() *object.List {
	return &object.List{Elements: r.elems}
}

// NativeBudget bounds data a library gathers natively before it becomes
// script values: the bytes gathered so far are checked against the memory
// left every nativeCheckBytes, along with the call's cancellation. One budget
// is shared by every goroutine gathering for the same call, so what they
// hold between them is what is checked.
type NativeBudget struct {
	ctx       context.Context
	what      string
	bytes     atomic.Int64
	nextCheck atomic.Int64
}

const nativeCheckBytes = 64 << 10

// NewNativeBudget starts a budget for the call ctx belongs to. what names the
// data being gathered in the error raised when it does not fit.
func NewNativeBudget(ctx context.Context, what string) *NativeBudget {
	b := &NativeBudget{ctx: ctx, what: what}
	b.nextCheck.Store(nativeCheckBytes)
	return b
}

// Add counts n more bytes, or returns the error that stops the gathering.
// The goroutine calling it must be bound to the call's guard.
func (b *NativeBudget) Add(n int) object.Object {
	total := b.bytes.Add(int64(n))
	next := b.nextCheck.Load()
	if total < next || !b.nextCheck.CompareAndSwap(next, total+nativeCheckBytes) {
		return nil
	}
	if errObj := object.CheckRoom(b.what, total); errObj != nil {
		return errObj
	}
	return CancelledError(b.ctx)
}

// CancelledError reports a cancelled context as the evaluator does, nil when
// the context is live.
func CancelledError(ctx context.Context) object.Object {
	if ctx == nil || ctx.Err() == nil {
		return nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return errors.NewTimeoutError()
	}
	return errors.NewCancelledError()
}
