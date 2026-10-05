package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

// instanceAsIterator converts an instance argument to a LAZY iterator by
// running the iterator protocol on demand (calling __iter__, then __next__
// per pull). This is what keeps itertools functions safe over generators
// and user iterables: nothing is materialized up front, so an endless
// source flows element by element to a bounded consumer instead of being
// collected into memory.
//
// Returns (iterator, nil) on success, (nil, error-object) when the protocol
// raised, and (nil, nil) when obj is not an instance (callers keep their
// list/tuple/string fast paths).
func instanceAsIterator(ctx context.Context, obj object.Object) (*object.Iterator, object.Object) {
	inst, isInst := obj.(*object.Instance)
	if !isInst {
		return nil, nil
	}
	eval := evaliface.FromContext(ctx)
	if eval == nil {
		return nil, errors.NewError("evaluator not available in context")
	}
	if fn, has := inst.Class.LookupMember("__iter__"); has {
		res := eval.CallObjectFunction(ctx, fn, []object.Object{inst}, nil, nil)
		if object.IsError(res) || res.Type() == object.EXCEPTION_OBJ {
			return nil, res
		}
		switch it := res.(type) {
		case *object.Iterator:
			return it, nil
		case *object.Instance:
			return nextableAsIterator(ctx, it)
		}
		return nil, &object.Exception{
			Message:       "iter() returned non-iterator of type '" + typeNameOf(res) + "'",
			ExceptionType: object.ExceptionTypeTypeError,
			Raised:        true,
		}
	}
	// No __iter__: the instance may itself be an iterator (__next__).
	if _, has := inst.Class.LookupMember("__next__"); has {
		return nextableAsIterator(ctx, inst)
	}
	return nil, nil
}

func typeNameOf(obj object.Object) string {
	if s, err := obj.AsString(); err == nil {
		return s
	}
	return obj.Type().String()
}

// nextableAsIterator wraps an instance with a __next__ method into a lazy
// iterator. A raised StopIteration ends iteration; any other raised
// exception or internal error is yielded as a propagating value so the
// consumer surfaces it, matching the iterator conventions elsewhere.
func nextableAsIterator(ctx context.Context, inst *object.Instance) (*object.Iterator, object.Object) {
	eval := evaliface.FromContext(ctx)
	if eval == nil {
		return nil, errors.NewError("evaluator not available in context")
	}
	fn, has := inst.Class.LookupMember("__next__")
	if !has {
		return nil, &object.Exception{
			Message:       "iter() returned a non-iterator instance",
			ExceptionType: object.ExceptionTypeTypeError,
			Raised:        true,
		}
	}
	done := false
	return object.NewIterator(func() (object.Object, bool) {
		if done {
			return nil, false
		}
		res := eval.CallObjectFunction(ctx, fn, []object.Object{inst}, nil, nil)
		if object.IsError(res) || res.Type() == object.EXCEPTION_OBJ {
			if exc, isExc := res.(*object.Exception); isExc && exc.ExceptionType == object.ExceptionTypeStopIteration {
				done = true
				return nil, false
			}
			done = true
			return res, true
		}
		return res, true
	}), nil
}
