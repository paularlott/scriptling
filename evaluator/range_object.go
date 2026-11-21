package evaluator

import (
	"context"
	"fmt"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Range objects (the value of range()) answer indexing, slicing, attributes
// and their two sequence methods arithmetically, as Python's range does.

// sliceBounds is Python's slice.indices(length) for an integer slice:
// the normalised start and stop to walk with step.
func sliceBounds(length, start, stop, step int64, hasStart, hasStop bool) (int64, int64) {
	clamp := func(v int64, has bool, dflt int64) int64 {
		if !has {
			return dflt
		}
		if v < 0 {
			v += length
			if v < 0 {
				if step < 0 {
					return -1
				}
				return 0
			}
		} else if v >= length {
			if step < 0 {
				return length - 1
			}
			return length
		}
		return v
	}
	if step < 0 {
		return clamp(start, hasStart, length-1), clamp(stop, hasStop, -1)
	}
	return clamp(start, hasStart, 0), clamp(stop, hasStop, length)
}

// rangeSlice is r[start:stop:step] for a range object r: another range.
func rangeSlice(r *object.Iterator, start, stop, step int64, hasStart, hasStop bool) object.Object {
	rStart, _, rStep, _ := r.RangeObject()
	n, _ := r.Len()
	i, j := sliceBounds(n, start, stop, step, hasStart, hasStop)
	return object.NewRange(rStart+i*rStep, rStart+j*rStep, rStep*step)
}

// evalRangeIndexExpression is r[i], r[a:b:c] and r.attr for a range object
// r; ok is false when r is another kind of iterator.
func evalRangeIndexExpression(r *object.Iterator, index object.Object, isDotAccess bool) (object.Object, bool) {
	start, stop, step, ok := r.RangeObject()
	if !ok {
		return nil, false
	}
	if isDotAccess {
		attr, _ := index.AsString()
		switch attr {
		case "start":
			return object.NewInteger(start), true
		case "stop":
			return object.NewInteger(stop), true
		case "step":
			return object.NewInteger(step), true
		case "index", "count":
			return rangeMethod(r, attr), true
		}
		return attributeError(r, attr), true
	}
	n, _ := r.Len()
	switch idx := index.(type) {
	case *object.Integer:
		i := idx.IntValue()
		if i < 0 {
			i += n
		}
		if i < 0 || i >= n {
			return &object.Exception{Message: "range object index out of range", ExceptionType: object.ExceptionTypeIndexError, Raised: true}, true
		}
		return object.NewInteger(start + i*step), true
	case *object.Boolean:
		return evalRangeIndexExpression(r, object.NewInteger(boolToInt64(idx.BoolValue())), false)
	case *object.Slice:
		var a, b, c int64 = 0, 0, 1
		var hasA, hasB bool
		for _, part := range []struct {
			v   object.Object
			dst *int64
			has *bool
		}{{idx.Start, &a, &hasA}, {idx.End, &b, &hasB}, {idx.Step, &c, nil}} {
			if part.v == nil || part.v == NULL {
				continue
			}
			if _, isNull := part.v.(*object.Null); isNull {
				continue
			}
			v, errObj := part.v.AsInt()
			if errObj != nil {
				return &object.Exception{Message: "slice indices must be integers or None or have an __index__ method", ExceptionType: object.ExceptionTypeTypeError, Raised: true}, true
			}
			*part.dst = v
			if part.has != nil {
				*part.has = true
			}
		}
		if c == 0 {
			return &object.Exception{Message: "slice step cannot be zero", ExceptionType: object.ExceptionTypeValueError, Raised: true}, true
		}
		return rangeSlice(r, a, b, c, hasA, hasB), true
	}
	return &object.Exception{
		Message:       fmt.Sprintf("range indices must be integers or slices, not %s", getTypeName(index)),
		ExceptionType: object.ExceptionTypeTypeError,
		Raised:        true,
	}, true
}

// rangeMethod is the bound index or count method of a range object.
func rangeMethod(r *object.Iterator, name string) object.Object {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("range."+name, kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			start, _, step, _ := r.RangeObject()
			found := false
			var v int64
			switch x := args[0].(type) {
			case *object.Integer:
				v, found = x.IntValue(), r.RangeContains(x.IntValue())
			case *object.Boolean:
				v = boolToInt64(x.BoolValue())
				found = r.RangeContains(v)
			}
			if name == "count" {
				if found {
					return object.NewInteger(1)
				}
				return object.NewInteger(0)
			}
			if !found {
				return &object.Exception{Message: "range.index(x): x not in range", ExceptionType: object.ExceptionTypeValueError, Raised: true}
			}
			return object.NewInteger((v - start) / step)
		},
		HelpText: fmt.Sprintf("range.%s(value) - %s", name, map[string]string{
			"index": "Return the position of value in the range; ValueError if absent",
			"count": "Return 1 if value is in the range, else 0",
		}[name]),
		Repr: fmt.Sprintf("<built-in method %s of range object>", name),
	}
}
