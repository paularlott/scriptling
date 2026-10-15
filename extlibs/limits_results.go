package extlibs

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

// resultList builds a list for a script, checking each growth of its backing
// array against the value limit and the memory left, and the call's
// cancellation every resultCheckEvery items.
type resultList struct {
	ctx   context.Context
	elems []object.Object
	added int
}

// newResultList starts a list expected to hold about hint items. The hint
// is checked up front; a hint too large to admit is an error before anything
// is built.
func newResultList(ctx context.Context, hint int) (*resultList, object.Object) {
	if errObj := object.CheckElements(int64(hint)); errObj != nil {
		return nil, errObj
	}
	elems := make([]object.Object, 0, min(hint, 1<<16))
	object.TrackSlice(elems)
	return &resultList{ctx: ctx, elems: elems}, nil
}

// add appends v, or returns the error that stops the build.
func (r *resultList) add(v object.Object) object.Object {
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
		return cancelledError(r.ctx)
	}
	return nil
}

// list returns the built list.
func (r *resultList) list() *object.List {
	return &object.List{Elements: r.elems}
}

// nativeBudget bounds data a library gathers natively before it becomes
// script values: the bytes gathered so far are checked against the memory
// left every nativeCheckBytes, along with the call's cancellation. One budget
// is shared by every goroutine gathering for the same call, so what they
// hold between them is what is checked.
type nativeBudget struct {
	ctx       context.Context
	what      string
	bytes     atomic.Int64
	nextCheck atomic.Int64
}

const nativeCheckBytes = 64 << 10

func newNativeBudget(ctx context.Context, what string) *nativeBudget {
	b := &nativeBudget{ctx: ctx, what: what}
	b.nextCheck.Store(nativeCheckBytes)
	return b
}

// add counts n more bytes, or returns the error that stops the gathering.
// The goroutine calling it must be bound to the call's guard.
func (b *nativeBudget) add(n int) object.Object {
	total := b.bytes.Add(int64(n))
	next := b.nextCheck.Load()
	if total < next || !b.nextCheck.CompareAndSwap(next, total+nativeCheckBytes) {
		return nil
	}
	if errObj := object.CheckRoom(b.what, total); errObj != nil {
		return errObj
	}
	return cancelledError(b.ctx)
}

// cancelledError reports a cancelled context as the evaluator does, nil when
// the context is live.
func cancelledError(ctx context.Context) object.Object {
	if ctx == nil || ctx.Err() == nil {
		return nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return errors.NewTimeoutError()
	}
	return errors.NewCancelledError()
}
