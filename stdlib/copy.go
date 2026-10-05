package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

const CopyLibraryName = "copy"

// CopyLibrary provides copy.copy (shallow) and copy.deepcopy, like Python's
// copy module. deepcopy is cycle-safe (an identity memo), honours a
// __deepcopy__ method on instances, and shares values Python considers
// atomic: scalars, bytes, frozen sets, functions, classes and modules.
var CopyLibrary = object.NewLibrary(CopyLibraryName, map[string]*object.Builtin{
	"copy": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			return shallowCopy(args[0])
		},
		HelpText: `copy(x) - Return a shallow copy of x

Same as the copy() builtin: a new list/dict/set/instance with the same
top-level contents. Nested objects are shared (use deepcopy).`,
	},
	"deepcopy": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// The optional second argument of Python's deepcopy(x, memo) is
			// accepted and ignored: scripts have no way to build a useful
			// memo themselves.
			if len(args) != 1 && len(args) != 2 {
				return errors.NewError("deepcopy() takes 1 or 2 arguments (%d given)", len(args))
			}
			return deepCopy(ctx, args[0], make(map[object.Object]object.Object))
		},
		HelpText: `deepcopy(x) - Return a deep copy of x

Recursively copies lists, tuples, dicts, sets and instances, preserving
shared references (two references to one list stay shared in the copy)
and handling cycles. A class can define __deepcopy__(self) to control its
own copying. Values Python treats as atomic — scalars, bytes, frozen
sets, functions, classes, library modules — are returned as-is.

Example:
  import copy
  original = {"items": [1, 2], "meta": {"on": True}}
  dup = copy.deepcopy(original)
  dup["items"].append(3)
  original["items"]        # [1, 2]: unchanged`,
	},
}, nil, "Shallow and deep copy operations, like Python's copy module")

// shallowCopy is the copy() builtin's behaviour: one new container with
// shared elements.
func shallowCopy(obj object.Object) object.Object {
	switch o := obj.(type) {
	case *object.List:
		elements := make([]object.Object, len(o.Elements))
		copy(elements, o.Elements)
		return &object.List{Elements: elements}
	case *object.Dict:
		clone := object.NewDict()
		clone.SetFactory(o.Factory())
		clone.StoreFrom(o)
		return clone
	case *object.Set:
		return o.Copy()
	case *object.Tuple:
		return o // immutable, safe to share
	case *object.Instance:
		clone := &object.Instance{Class: o.Class}
		o.RangeFields(func(k string, v object.Object) bool {
			clone.SetField(k, v)
			return true
		})
		return clone
	default:
		return obj // scalars are immutable
	}
}

// maxDeepcopyDepth bounds recursion so a pathologically nested structure
// reports a catchable error instead of exhausting the goroutine stack
// (scriptling containers can be built arbitrarily deep by loops; equality
// and repr share this exposure, but new code gets a bound).
const maxDeepcopyDepth = 100000

// deepCopy recursively copies obj. The memo maps an object to its copy,
// which breaks cycles and keeps shared substructure shared; objects are
// keyed by interface identity (every memoized type is a pointer), avoiding
// a per-node key allocation.
func deepCopy(ctx context.Context, obj object.Object, memo map[object.Object]object.Object) object.Object {
	return deepCopyDepth(ctx, obj, memo, maxDeepcopyDepth)
}

// isCopyFailure reports whether a recursive copy result is an error or a
// raised exception that must propagate instead of being embedded in the
// copy as an element.
func isCopyFailure(o object.Object) bool {
	return object.IsError(o) || o.Type() == object.EXCEPTION_OBJ
}

func deepCopyDepth(ctx context.Context, obj object.Object, memo map[object.Object]object.Object, depth int) object.Object {
	if depth == 0 {
		return errors.NewError("deepcopy exceeded maximum nesting depth (%d)", maxDeepcopyDepth)
	}
	switch v := obj.(type) {
	case *object.Null, *object.Boolean, *object.Integer, *object.Float,
		*object.String, *object.Bytes, *object.Sentinel:
		return obj
	case *object.List:
		if c, ok := memo[obj]; ok {
			return c
		}
		result := &object.List{Elements: make([]object.Object, len(v.Elements))}
		memo[obj] = result
		for i, e := range v.Elements {
			copied := deepCopyDepth(ctx, e, memo, depth-1)
			if isCopyFailure(copied) {
				return copied
			}
			result.Elements[i] = copied
		}
		return result
	case *object.Tuple:
		// Python returns an all-atomic tuple unchanged (same identity).
		if tupleIsAtomic(v) {
			return v
		}
		if c, ok := memo[obj]; ok {
			return c
		}
		result := &object.Tuple{Elements: make([]object.Object, len(v.Elements))}
		memo[obj] = result
		for i, e := range v.Elements {
			copied := deepCopyDepth(ctx, e, memo, depth-1)
			if isCopyFailure(copied) {
				return copied
			}
			result.Elements[i] = copied
		}
		return result
	case *object.Dict:
		// Library modules are singletons; copying one would be wrong.
		if v.Module != "" {
			return v
		}
		if c, ok := memo[obj]; ok {
			return c
		}
		result := object.NewDict()
		result.SetFactory(v.Factory())
		memo[obj] = result
		// Keys are hashable, hence immutable; values may nest arbitrarily.
		// The copy preserves the dict's insertion order.
		for _, k := range v.OrderedKeys() {
			p := v.Pairs[k]
			copied := deepCopyDepth(ctx, p.Value, memo, depth-1)
			if isCopyFailure(copied) {
				return copied
			}
			result.Store(k, p.Key, copied)
		}
		return result
	case *object.Set:
		if v.Frozen {
			return v
		}
		if c, ok := memo[obj]; ok {
			return c
		}
		result := object.NewSet()
		memo[obj] = result
		// Elements are hashable, so effectively immutable: sharing them is
		// safe and keeps their set keys valid.
		for k, e := range v.Elements {
			result.Elements[k] = e
		}
		return result
	case *object.FloatArray:
		data := make([]float64, len(v.Data))
		copy(data, v.Data)
		shape := make([]int, len(v.Shape))
		copy(shape, v.Shape)
		return &object.FloatArray{Data: data, Shape: shape}
	case *object.Instance:
		if c, ok := memo[obj]; ok {
			return c
		}
		// A __deepcopy__ method takes over completely, as in Python. Python's
		// signature is __deepcopy__(self, memo); the one-argument form is
		// accepted too.
		if fn, ok := v.Class.Methods["__deepcopy__"]; ok {
			if ev := evaliface.FromContext(ctx); ev != nil {
				callArgs := []object.Object{v}
				switch f := fn.(type) {
				case *object.Function:
					if len(f.Parameters) == 2 {
						callArgs = append(callArgs, &object.Dict{Pairs: make(map[string]object.DictPair)})
					}
				case *object.LambdaFunction:
					if len(f.Parameters) == 2 {
						callArgs = append(callArgs, &object.Dict{Pairs: make(map[string]object.DictPair)})
					}
				}
				result := ev.CallObjectFunction(ctx, fn, callArgs, nil, nil)
				if result == nil {
					return errors.NewError("__deepcopy__ returned no value")
				}
				if isCopyFailure(result) {
					return result
				}
				memo[obj] = result
				return result
			}
		}
		result := &object.Instance{Class: v.Class}
		memo[obj] = result
		var errCopy object.Object
		v.RangeFields(func(k string, val object.Object) bool {
			copied := deepCopyDepth(ctx, val, memo, depth-1)
			if isCopyFailure(copied) {
				errCopy = copied
				return false
			}
			result.SetField(k, copied)
			return true
		})
		if errCopy != nil {
			return errCopy
		}
		return result
	default:
		// Functions, lambdas, classes, builtins, iterators, errors: Python
		// treats these as atomic and returns them unchanged.
		return obj
	}
}

// tupleIsAtomic reports whether every element is an immutable value, so the
// tuple can be shared instead of copied.
func tupleIsAtomic(t *object.Tuple) bool {
	for _, e := range t.Elements {
		switch e.(type) {
		case *object.Null, *object.Boolean, *object.Integer, *object.Float,
			*object.String, *object.Bytes, *object.Sentinel:
		case *object.Set:
			if !e.(*object.Set).Frozen {
				return false
			}
		default:
			return false
		}
	}
	return true
}
