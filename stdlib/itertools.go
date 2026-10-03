package stdlib

import (
	"context"
	"fmt"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

// isCallable reports whether obj can be called as a function/predicate.
func isCallable(obj object.Object) bool {
	switch o := obj.(type) {
	case *object.Builtin, *object.Function, *object.LambdaFunction, *object.BoundMethod, *object.Class:
		return true
	case *object.Instance:
		_, ok := o.Class.Methods["__call__"]
		return ok
	default:
		return false
	}
}

// callCallable invokes any callable (builtin, def, lambda, bound method) with
// the given positional args and returns its result. Errors and raised
// exceptions are returned as-is so callers can propagate them. Used by the
// itertools higher-order functions so they accept script-defined predicates,
// not only builtins.
func callCallable(ctx context.Context, fn object.Object, args ...object.Object) object.Object {
	if builtin, ok := fn.(*object.Builtin); ok {
		return builtin.Fn(ctx, object.NewKwargs(nil), args...)
	}
	eval := evaliface.FromContext(ctx)
	if eval == nil {
		return errors.NewError("evaluator not available in context")
	}
	return eval.CallObjectFunction(ctx, fn, args, nil, nil)
}

// newArgTypeError builds an error that `except TypeError:` matches, for
// Python's argument errors (unexpected or duplicated keyword arguments).
func newArgTypeError(format string, args ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, args...), ExceptionType: object.ExceptionTypeTypeError}
}

// optionalArg returns the optional parameter at position idx, or the keyword
// argument name when not given positionally (as Python accepts either). A
// value given both ways is a TypeError. None is returned as nil.
func optionalArg(fname string, args []object.Object, kwargs object.Kwargs, idx int, name string) (object.Object, object.Object) {
	var v object.Object
	if len(args) > idx {
		if kwargs.Has(name) {
			return nil, newArgTypeError("%s() got multiple values for argument '%s'", fname, name)
		}
		v = args[idx]
	} else {
		v = kwargs.Get(name)
	}
	if _, isNull := v.(*object.Null); isNull {
		return nil, nil
	}
	return v, nil
}

// kwargsToPositional merges keyword arguments into the positional list for
// functions whose Python parameters may be passed either way. names lists the
// parameters in positional order. A keyword naming an already-filled
// position, an unknown keyword, or a gap before a keyword is a TypeError.
func kwargsToPositional(fname string, args []object.Object, kwargs object.Kwargs, names ...string) ([]object.Object, object.Object) {
	if kwargs.Len() == 0 {
		return args, nil
	}
	out := append([]object.Object(nil), args...)
	for i, name := range names {
		v := kwargs.Get(name)
		if v == nil {
			continue
		}
		if i < len(args) {
			return nil, newArgTypeError("%s() got multiple values for argument '%s'", fname, name)
		}
		if i != len(out) {
			return nil, newArgTypeError("%s() missing required argument: '%s'", fname, names[len(out)])
		}
		out = append(out, v)
	}
	if err := checkKwargs(fname, kwargs, names...); err != nil {
		return nil, err
	}
	return out, nil
}

// checkKwargs rejects keyword arguments the function does not accept, as
// Python raises TypeError rather than silently ignoring them.
func checkKwargs(fname string, kwargs object.Kwargs, allowed ...string) object.Object {
	for _, k := range kwargs.Keys() {
		ok := false
		for _, a := range allowed {
			if k == a {
				ok = true
				break
			}
		}
		if !ok {
			return newArgTypeError("%s() got an unexpected keyword argument '%s'", fname, k)
		}
	}
	return nil
}

// ItertoolsLibrary provides Python-like itertools functions
var ItertoolsLibrary = object.NewLibrary(ItertoolsLibraryName, map[string]*object.Builtin{
	"chain": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// chain(*iterables) - Chain multiple iterables together
			result := []object.Object{}
			for _, arg := range args {
				switch a := arg.(type) {
				case *object.List:
					result = append(result, a.Elements...)
				case *object.Tuple:
					result = append(result, a.Elements...)
				case *object.String:
					for _, ch := range a.StringValue() {
						result = append(result, object.NewString(string(ch)))
					}
				default:
					elems, errObj := collectIterable(ctx, arg)
					if errObj != nil {
						return errObj
					}
					result = append(result, elems...)
				}
			}
			return &object.List{Elements: result}
		},
		HelpText: `chain(*iterables) - Chain multiple iterables together

Returns a list with elements from all iterables concatenated.

Example:
  itertools.chain([1, 2], [3, 4]) -> [1, 2, 3, 4]
  itertools.chain("ab", "cd") -> ["a", "b", "c", "d"]`,
	},
	"repeat": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// repeat(elem[, times]) - Repeat element times times, or
			// endlessly (a lazy iterator, as in Python) when times is omitted.
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			if err := checkKwargs("repeat", kwargs, "times"); err != nil {
				return err
			}
			elem := args[0]
			timesObj, terr := optionalArg("repeat", args, kwargs, 1, "times")
			if terr != nil {
				return terr
			}
			if timesObj == nil {
				return object.NewInfiniteIterator(func() (object.Object, bool) {
					return elem, true
				})
			}
			n, ok := timesObj.(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", timesObj.Type().String())
			}
			left := n.IntValue()
			return object.NewIterator(func() (object.Object, bool) {
				if left <= 0 {
					return nil, false
				}
				left--
				return elem, true
			})
		},
		HelpText: `repeat(elem[, times]) - Repeat element times times

Returns an iterator giving elem times times, or forever when times is
omitted, like Python's itertools.repeat. Use list() for a list, or consume an
endless one with next(), zip() or itertools.islice.

Example:
  list(itertools.repeat("x", 3)) -> ["x", "x", "x"]
  list(zip(range(3), itertools.repeat(0))) -> [(0, 0), (1, 0), (2, 0)]`,
	},
	"cycle": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// cycle(iterable) matches Python: a lazily infinite cycling
			// iterator (list() of it never terminates, exactly like Python).
			// The legacy finite form cycle(iterable, n) still works and
			// returns the materialized list.
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			// An iterator input is consumed lazily, saving each element for
			// the replay passes (Python semantics), so cycle(count()) or a
			// generator works without materializing it up front.
			if src, isIter := args[0].(*object.Iterator); isIter && len(args) == 1 {
				var saved []object.Object
				exhausted := false
				i := 0
				// Endless unless the source is empty, which is not worth
				// consuming the source up front to find out.
				return object.NewInfiniteIterator(func() (object.Object, bool) {
					if !exhausted {
						if v, ok := src.Next(); ok {
							if !object.IsError(v) && v.Type() != object.EXCEPTION_OBJ {
								saved = append(saved, v)
							}
							return v, true
						}
						exhausted = true
					}
					if len(saved) == 0 {
						return nil, false
					}
					elem := saved[i%len(saved)]
					i++
					return elem, true
				})
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.StringValue() {
					elements = append(elements, object.NewString(string(ch)))
				}
			default:
				elems, errObj := collectIterable(ctx, args[0])
				if errObj != nil {
					return errObj
				}
				elements = elems
			}
			if len(args) == 2 {
				n, ok := args[1].(*object.Integer)
				if !ok {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
				if len(elements) == 0 || n.IntValue() <= 0 {
					return &object.List{Elements: []object.Object{}}
				}
				result := make([]object.Object, 0, len(elements)*int(n.IntValue()))
				for i := int64(0); i < n.IntValue(); i++ {
					result = append(result, elements...)
				}
				return &object.List{Elements: result}
			}
			if len(elements) == 0 {
				// Python: cycle of an empty iterable is an empty iterator.
				return object.NewIterator(func() (object.Object, bool) { return nil, false })
			}
			i := 0
			return object.NewInfiniteIterator(func() (object.Object, bool) {
				elem := elements[i%len(elements)]
				i++
				return elem, true
			})
		},
		HelpText: `cycle(iterable) - Cycle through iterable endlessly

Returns an infinite iterator, like Python's itertools.cycle; consume it with
next() or itertools.islice. The legacy form cycle(iterable, n) returns the
materialized list of n repetitions.`,
	},
	"count": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// count(start=0, step=1) - Python's infinite lazy counter; start
			// and step may be positional or keywords.
			{
				if len(args) > 2 {
					return &object.Exception{
						Message:       fmt.Sprintf("count() takes at most 2 arguments (%d given)", len(args)),
						ExceptionType: object.ExceptionTypeTypeError,
						Raised:        true,
					}
				}
				if err := checkKwargs("count", kwargs, "start", "step"); err != nil {
					return err
				}
				startObj, serr := optionalArg("count", args, kwargs, 0, "start")
				if serr != nil {
					return serr
				}
				stepObj, terr := optionalArg("count", args, kwargs, 1, "step")
				if terr != nil {
					return terr
				}
				if _, isNull := stepObj.(*object.Null); isNull {
					stepObj = nil
				}
				if startObj == nil {
					startObj = object.NewInteger(0)
				}
				if stepObj == nil {
					stepObj = object.NewInteger(1)
				}
				for _, o := range []object.Object{startObj, stepObj} {
					switch o.(type) {
					case *object.Integer, *object.Float:
					default:
						return errors.NewTypeError("a number is required", o.Type().String())
					}
				}
				si, sInt := startObj.(*object.Integer)
				ti, tInt := stepObj.(*object.Integer)
				if sInt && tInt {
					cur, step := si.IntValue(), ti.IntValue()
					return object.NewInfiniteIterator(func() (object.Object, bool) {
						v := object.NewInteger(cur)
						cur += step
						return v, true
					})
				}
				curF, _ := startObj.AsFloat()
				stepF, _ := stepObj.AsFloat()
				return object.NewInfiniteIterator(func() (object.Object, bool) {
					v := object.NewFloat(curF)
					curF += stepF
					return v, true
				})
			}
		},
		HelpText: `count(start=0, step=1) - Count up from start by step, forever

Returns an infinite iterator like Python's itertools.count; consume it with
next(), zip(), enumerate() or itertools.islice.`,
	},
	"islice": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// islice(iterable, stop) or islice(iterable, start, stop[, step])
			if err := errors.RangeArgs(args, 2, 4); err != nil {
				return err
			}
			// Iterator inputs stay lazy: islice over an infinite iterator
			// (itertools.cycle) must not materialize it.
			if iter, isIter := args[0].(*object.Iterator); isIter {
				var start, stop, step int64 = 0, 0, 1
				if err := parseIsliceBounds(args, &start, &stop, &step); err != nil {
					return err
				}
				if step <= 0 {
					return errors.NewError("step must be positive")
				}
				skipped := int64(0)
				taken := int64(0)
				return object.NewIterator(func() (object.Object, bool) {
					for {
						if taken >= stop-start {
							return nil, false
						}
						val, ok := iter.Next()
						if !ok {
							return nil, false
						}
						pos := skipped
						skipped++
						if pos < start || (pos-start)%step != 0 {
							continue
						}
						taken++
						return val, true
					}
				})
			}

			elements, errObj := collectIterable(ctx, args[0])
			if errObj != nil {
				return errObj
			}

			var start, stop, step int64 = 0, 0, 1
			if err := parseIsliceBounds(args, &start, &stop, &step); err != nil {
				return err
			}

			if step <= 0 {
				return errors.NewError("step must be positive")
			}
			if start < 0 {
				start = 0
			}
			if stop > int64(len(elements)) {
				stop = int64(len(elements))
			}

			result := []object.Object{}
			for i := start; i < stop; i += step {
				result = append(result, elements[i])
			}
			return &object.List{Elements: result}
		},
		HelpText: `islice(iterable, stop) or islice(iterable, start, stop[, step]) - Slice an iterable

Returns a list with elements from the iterable sliced by indices.

Example:
  itertools.islice([0, 1, 2, 3, 4], 3) -> [0, 1, 2]
  itertools.islice([0, 1, 2, 3, 4], 1, 4) -> [1, 2, 3]
  itertools.islice([0, 1, 2, 3, 4], 0, 5, 2) -> [0, 2, 4]`,
	},
	"takewhile": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// takewhile(predicate, iterable) - Take elements while predicate is true
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			pred := args[0]
			if !isCallable(pred) {
				return errors.NewTypeError("callable", pred.Type().String())
			}
			// Pull lazily: takewhile over an infinite iterator (count(),
			// cycle()) must stop at the first false predicate, as in Python.
			next, ok := object.IterSource(args[1])
			if !ok {
				return notIterableError(args[1])
			}
			result := []object.Object{}
			for {
				if err := ctx.Err(); err != nil {
					return errors.NewError("%s", err.Error())
				}
				elem, more := next()
				if !more {
					break
				}
				if object.IsPropagating(elem) {
					return elem
				}
				res := callCallable(ctx, pred, elem)
				if object.IsError(res) || res.Type() == object.EXCEPTION_OBJ {
					return res
				}
				if !isTruthy(res) {
					break
				}
				result = append(result, elem)
			}
			return &object.List{Elements: result}
		},
		HelpText: `takewhile(predicate, iterable) - Take elements while predicate is true

Returns a list with elements from the start of iterable as long as predicate is true.

Example:
  itertools.takewhile(lambda x: x < 5, [1, 3, 5, 2, 4]) -> [1, 3]`,
	},
	"dropwhile": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// dropwhile(predicate, iterable) - Drop elements while predicate is true
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			pred := args[0]
			if !isCallable(pred) {
				return errors.NewTypeError("callable", pred.Type().String())
			}
			var elements []object.Object
			switch a := args[1].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			default:
				elems, errObj := collectIterable(ctx, args[1])
				if errObj != nil {
					return errObj
				}
				elements = elems
			}
			result := []object.Object{}
			dropping := true
			for _, elem := range elements {
				if dropping {
					res := callCallable(ctx, pred, elem)
					if object.IsError(res) || res.Type() == object.EXCEPTION_OBJ {
						return res
					}
					if isTruthy(res) {
						continue
					}
					dropping = false
				}
				result = append(result, elem)
			}
			return &object.List{Elements: result}
		},
		HelpText: `dropwhile(predicate, iterable) - Drop elements while predicate is true

Returns a list with elements after the predicate becomes false.

Example:
  itertools.dropwhile(lambda x: x < 5, [1, 3, 5, 2, 4]) -> [5, 2, 4]`,
	},
	"zip_longest": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// zip_longest(*iterables, fillvalue=None)
			if len(args) < 1 {
				return &object.List{Elements: []object.Object{}}
			}

			// Get fillvalue from kwargs
			fillvalue := object.Object(&object.Null{})
			if kwargs.Len() > 0 {
				if kwargs.Has("fillvalue") {
					fillvalue = kwargs.Get("fillvalue")
				}
			}

			// Convert all arguments to slices
			iterables := make([][]object.Object, len(args))
			maxLen := 0
			for i, arg := range args {
				switch a := arg.(type) {
				case *object.List:
					iterables[i] = a.Elements
				case *object.Tuple:
					iterables[i] = a.Elements
				case *object.String:
					chars := []object.Object{}
					for _, ch := range a.StringValue() {
						chars = append(chars, object.NewString(string(ch)))
					}
					iterables[i] = chars
				default:
					elems, errObj := collectIterable(ctx, arg)
					if errObj != nil {
						return errObj
					}
					iterables[i] = elems
				}
				if len(iterables[i]) > maxLen {
					maxLen = len(iterables[i])
				}
			}

			result := []object.Object{}
			for j := 0; j < maxLen; j++ {
				tuple := make([]object.Object, len(iterables))
				for i, iter := range iterables {
					if j < len(iter) {
						tuple[i] = iter[j]
					} else {
						tuple[i] = fillvalue
					}
				}
				result = append(result, &object.Tuple{Elements: tuple})
			}
			return &object.List{Elements: result}
		},
		HelpText: `zip_longest(*iterables, fillvalue=None) - Zip iterables, filling shorter ones

Zips iterables together, using fillvalue for missing values in shorter iterables.

Example:
  itertools.zip_longest([1, 2, 3], ["a", "b"]) -> [(1, "a"), (2, "b"), (3, None)]
  itertools.zip_longest([1, 2], ["a"], fillvalue="-") -> [(1, "a"), (2, "-")]`,
	},
	"product": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// product(*iterables) - Cartesian product
			if len(args) < 1 {
				return &object.List{Elements: []object.Object{&object.Tuple{Elements: []object.Object{}}}}
			}

			// product(*iterables, repeat=N) repeats the iterables N times,
			// like Python.
			repeatN := int64(1)
			if v := kwargs.Get("repeat"); v != nil {
				if n, err := v.AsInt(); err == nil {
					repeatN = n
				} else {
					return errors.ParameterError("repeat", err)
				}
			}
			// Materialize each argument once: iterator arguments are
			// consumed by the first pass, so repeating must reuse the
			// collected elements rather than re-iterate.
			collected := make([][]object.Object, 0, len(args))
			for _, arg := range args {
				switch a := arg.(type) {
				case *object.List:
					collected = append(collected, a.Elements)
				case *object.Tuple:
					collected = append(collected, a.Elements)
				case *object.String:
					chars := []object.Object{}
					for _, ch := range a.StringValue() {
						chars = append(chars, object.NewString(string(ch)))
					}
					collected = append(collected, chars)
				default:
					elems, errObj := collectIterable(ctx, arg)
					if errObj != nil {
						return errObj
					}
					collected = append(collected, elems)
				}
			}
			iterables := make([][]object.Object, 0, int(repeatN)*len(collected))
			for rep := int64(0); rep < repeatN; rep++ {
				iterables = append(iterables, collected...)
			}

			// Check for empty iterables
			for _, iter := range iterables {
				if len(iter) == 0 {
					return &object.List{Elements: []object.Object{}}
				}
			}

			// Calculate cartesian product
			result := []object.Object{}
			indices := make([]int, len(iterables))

			for {
				// Create current tuple
				tuple := make([]object.Object, len(iterables))
				for i, idx := range indices {
					tuple[i] = iterables[i][idx]
				}
				result = append(result, &object.Tuple{Elements: tuple})

				// Increment indices
				carry := true
				for i := len(indices) - 1; i >= 0 && carry; i-- {
					indices[i]++
					if indices[i] >= len(iterables[i]) {
						indices[i] = 0
					} else {
						carry = false
					}
				}
				if carry {
					break
				}
			}
			return &object.List{Elements: result}
		},
		HelpText: `product(*iterables) - Cartesian product of iterables

Returns all possible combinations (tuples) of elements from input iterables.

Example:
  itertools.product([1, 2], ["a", "b"]) -> [(1, "a"), (1, "b"), (2, "a"), (2, "b")]
  itertools.product([1, 2], [3, 4]) -> [(1, 3), (1, 4), (2, 3), (2, 4)]`,
	},
	"permutations": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			args, kerr := kwargsToPositional("permutations", args, kwargs, "iterable", "r")
			if kerr != nil {
				return kerr
			}
			// permutations(iterable[, r])
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.StringValue() {
					elements = append(elements, object.NewString(string(ch)))
				}
			default:
				elems, errObj := collectIterable(ctx, args[0])
				if errObj != nil {
					return errObj
				}
				elements = elems
			}

			r := len(elements)
			if len(args) == 2 && args[1].Type() != object.NULL_OBJ {
				if rArg, ok := args[1].(*object.Integer); ok {
					r = int(rArg.IntValue())
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}

			if r < 0 {
				return errors.NewValueError("r must be non-negative")
			}
			if r > len(elements) {
				return &object.List{Elements: []object.Object{}}
			}

			// Generate permutations
			result := []object.Object{}
			generatePermutations(elements, r, []object.Object{}, make([]bool, len(elements)), &result)
			return &object.List{Elements: result}
		},
		HelpText: `permutations(iterable[, r]) - Generate permutations

Returns all r-length permutations of elements from iterable.
If r is not specified, defaults to length of iterable (full permutations).

Example:
  itertools.permutations([1, 2, 3], 2) -> [(1, 2), (1, 3), (2, 1), (2, 3), (3, 1), (3, 2)]
  itertools.permutations("ab") -> [("a", "b"), ("b", "a")]`,
	},
	"combinations": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			args, kerr := kwargsToPositional("combinations", args, kwargs, "iterable", "r")
			if kerr != nil {
				return kerr
			}
			// combinations(iterable, r)
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.StringValue() {
					elements = append(elements, object.NewString(string(ch)))
				}
			default:
				elems, errObj := collectIterable(ctx, args[0])
				if errObj != nil {
					return errObj
				}
				elements = elems
			}

			rArg, ok := args[1].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			r := int(rArg.IntValue())

			if r < 0 || r > len(elements) {
				return &object.List{Elements: []object.Object{}}
			}

			// Generate combinations
			result := []object.Object{}
			generateCombinations(elements, r, 0, []object.Object{}, &result)
			return &object.List{Elements: result}
		},
		HelpText: `combinations(iterable, r) - Generate combinations

Returns all r-length combinations of elements from iterable (without repetition).

Example:
  itertools.combinations([1, 2, 3], 2) -> [(1, 2), (1, 3), (2, 3)]
  itertools.combinations("abc", 2) -> [("a", "b"), ("a", "c"), ("b", "c")]`,
	},
	"combinations_with_replacement": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			args, kerr := kwargsToPositional("combinations_with_replacement", args, kwargs, "iterable", "r")
			if kerr != nil {
				return kerr
			}
			// combinations_with_replacement(iterable, r)
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.StringValue() {
					elements = append(elements, object.NewString(string(ch)))
				}
			default:
				elems, errObj := collectIterable(ctx, args[0])
				if errObj != nil {
					return errObj
				}
				elements = elems
			}

			rArg, ok := args[1].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			r := int(rArg.IntValue())

			if r < 0 {
				return &object.List{Elements: []object.Object{}}
			}
			if len(elements) == 0 && r > 0 {
				return &object.List{Elements: []object.Object{}}
			}

			// Generate combinations with replacement
			result := []object.Object{}
			generateCombinationsWithReplacement(elements, r, 0, []object.Object{}, &result)
			return &object.List{Elements: result}
		},
		HelpText: `combinations_with_replacement(iterable, r) - Generate combinations with replacement

Returns all r-length combinations of elements from iterable (with repetition allowed).

Example:
  itertools.combinations_with_replacement([1, 2], 2) -> [(1, 1), (1, 2), (2, 2)]
  itertools.combinations_with_replacement("ab", 2) -> [("a", "a"), ("a", "b"), ("b", "b")]`,
	},
	"groupby": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// groupby(iterable[, key]) - Group consecutive elements
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			default:
				elems, errObj := collectIterable(ctx, args[0])
				if errObj != nil {
					return errObj
				}
				elements = elems
			}

			if err := checkKwargs("groupby", kwargs, "key"); err != nil {
				return err
			}
			keyFunc, kerr := optionalArg("groupby", args, kwargs, 1, "key")
			if kerr != nil {
				return kerr
			}
			if keyFunc != nil && !isCallable(keyFunc) {
				return errors.NewTypeError("callable", keyFunc.Type().String())
			}

			if len(elements) == 0 {
				return &object.List{Elements: []object.Object{}}
			}

			result := []object.Object{}
			var currentKey object.Object
			var currentGroup []object.Object

			for i, elem := range elements {
				var key object.Object
				if keyFunc != nil {
					key = callCallable(ctx, keyFunc, elem)
					if object.IsError(key) || key.Type() == object.EXCEPTION_OBJ {
						return key
					}
				} else {
					key = elem
				}

				if i == 0 {
					currentKey = key
					currentGroup = []object.Object{elem}
				} else if objectsEqual(key, currentKey) {
					currentGroup = append(currentGroup, elem)
				} else {
					// Save current group and start new one
					result = append(result, &object.Tuple{Elements: []object.Object{
						currentKey,
						&object.List{Elements: currentGroup},
					}})
					currentKey = key
					currentGroup = []object.Object{elem}
				}
			}

			// Don't forget the last group
			if len(currentGroup) > 0 {
				result = append(result, &object.Tuple{Elements: []object.Object{
					currentKey,
					&object.List{Elements: currentGroup},
				}})
			}

			return &object.List{Elements: result}
		},
		HelpText: `groupby(iterable, key=None) - Group consecutive elements

Groups consecutive elements that have the same key value.
Returns list of (key, group) tuples where group is a list.

Example:
  itertools.groupby([1, 1, 2, 2, 3]) -> [(1, [1, 1]), (2, [2, 2]), (3, [3])]
  itertools.groupby(["aa", "ab", "ba"], lambda x: x[0]) -> [("a", ["aa", "ab"]), ("b", ["ba"])]`,
	},
	"accumulate": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// accumulate(iterable[, func]) - Running totals
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			default:
				elems, errObj := collectIterable(ctx, args[0])
				if errObj != nil {
					return errObj
				}
				elements = elems
			}

			if err := checkKwargs("accumulate", kwargs, "func", "initial"); err != nil {
				return err
			}
			accumFunc, ferr := optionalArg("accumulate", args, kwargs, 1, "func")
			if ferr != nil {
				return ferr
			}
			if accumFunc != nil && !isCallable(accumFunc) {
				return errors.NewTypeError("callable", accumFunc.Type().String())
			}
			// initial is keyword-only; when given it is emitted first and
			// seeds the accumulation (Python 3.8+).
			if initial, _ := optionalArg("accumulate", nil, kwargs, 0, "initial"); initial != nil {
				elements = append([]object.Object{initial}, elements...)
			}

			if len(elements) == 0 {
				return &object.List{Elements: []object.Object{}}
			}

			result := []object.Object{elements[0]}
			accumulator := elements[0]

			for i := 1; i < len(elements); i++ {
				if accumFunc != nil {
					accumulator = callCallable(ctx, accumFunc, accumulator, elements[i])
					if object.IsError(accumulator) || accumulator.Type() == object.EXCEPTION_OBJ {
						return accumulator
					}
				} else {
					// Default: addition
					accumulator = addObjects(accumulator, elements[i])
					if object.IsError(accumulator) {
						return accumulator
					}
				}
				result = append(result, accumulator)
			}
			return &object.List{Elements: result}
		},
		HelpText: `accumulate(iterable[, func, *, initial=None]) - Running totals/accumulation

Returns list of accumulated values. Default is sum, but can provide custom function.
If initial is given, it is emitted first and seeds the accumulation.

Example:
  itertools.accumulate([1, 2, 3, 4]) -> [1, 3, 6, 10]
  itertools.accumulate([1, 2, 3], operator.mul) -> [1, 2, 6]`,
	},
	"filterfalse": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// filterfalse(predicate, iterable) - Filter elements where predicate is false
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			pred := args[0]
			if !isCallable(pred) {
				return errors.NewTypeError("callable", pred.Type().String())
			}
			var elements []object.Object
			switch a := args[1].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			default:
				elems, errObj := collectIterable(ctx, args[1])
				if errObj != nil {
					return errObj
				}
				elements = elems
			}
			result := []object.Object{}
			for _, elem := range elements {
				res := callCallable(ctx, pred, elem)
				if object.IsError(res) || res.Type() == object.EXCEPTION_OBJ {
					return res
				}
				if !isTruthy(res) {
					result = append(result, elem)
				}
			}
			return &object.List{Elements: result}
		},
		HelpText: `filterfalse(predicate, iterable) - Filter elements where predicate is false

Returns elements for which the predicate returns false.

Example:
  itertools.filterfalse(lambda x: x % 2, [1, 2, 3, 4]) -> [2, 4]`,
	},
	"starmap": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// starmap(func, iterable) - Apply function to argument tuples
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			fn := args[0]
			if !isCallable(fn) {
				return errors.NewTypeError("callable", fn.Type().String())
			}
			var elements []object.Object
			switch a := args[1].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			default:
				elems, errObj := collectIterable(ctx, args[1])
				if errObj != nil {
					return errObj
				}
				elements = elems
			}
			result := []object.Object{}
			for _, elem := range elements {
				var fnArgs []object.Object
				switch e := elem.(type) {
				case *object.List:
					fnArgs = e.Elements
				case *object.Tuple:
					fnArgs = e.Elements
				default:
					return errors.NewError("starmap() iterable must contain sequences")
				}
				res := callCallable(ctx, fn, fnArgs...)
				if object.IsError(res) || res.Type() == object.EXCEPTION_OBJ {
					return res
				}
				result = append(result, res)
			}
			return &object.List{Elements: result}
		},
		HelpText: `starmap(func, iterable) - Apply function to argument tuples

Applies function using elements of each tuple as arguments.

Example:
  itertools.starmap(pow, [(2, 3), (3, 2)]) -> [8, 9]`,
	},
	"compress": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// compress(data, selectors) - Filter data based on selectors
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			// Pull lazily so an infinite selector (or data) iterator such as
			// itertools.cycle stops with the shorter input, as in Python.
			data, dataOK := object.IterSource(args[0])
			if !dataOK {
				return notIterableError(args[0])
			}
			selectors, selOK := object.IterSource(args[1])
			if !selOK {
				return notIterableError(args[1])
			}
			if isEndless(args[0]) && isEndless(args[1]) {
				return errors.NewError("%s", object.InfiniteIteratorMessage)
			}

			result := []object.Object{}
			for {
				if err := ctx.Err(); err != nil {
					return errors.NewError("%s", err.Error())
				}
				d, ok := data()
				if !ok {
					break
				}
				if object.IsPropagating(d) {
					return d
				}
				sel, ok := selectors()
				if !ok {
					break
				}
				if object.IsPropagating(sel) {
					return sel
				}
				if isTruthy(sel) {
					result = append(result, d)
				}
			}
			return &object.List{Elements: result}
		},
		HelpText: `compress(data, selectors) - Filter data based on selectors

Returns elements from data where corresponding selector is truthy.

Example:
  itertools.compress([1, 2, 3, 4], [True, False, True, False]) -> [1, 3]
  itertools.compress("abcd", [1, 0, 1, 0]) -> ["a", "c"]`,
	},
	"pairwise": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// pairwise(iterable) - Return successive overlapping pairs
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.StringValue() {
					elements = append(elements, object.NewString(string(ch)))
				}
			default:
				elems, errObj := collectIterable(ctx, args[0])
				if errObj != nil {
					return errObj
				}
				elements = elems
			}

			if len(elements) < 2 {
				return &object.List{Elements: []object.Object{}}
			}

			result := make([]object.Object, len(elements)-1)
			for i := 0; i < len(elements)-1; i++ {
				result[i] = &object.Tuple{Elements: []object.Object{elements[i], elements[i+1]}}
			}
			return &object.List{Elements: result}
		},
		HelpText: `pairwise(iterable) - Return successive overlapping pairs

Returns consecutive pairs from the iterable.

Example:
  itertools.pairwise([1, 2, 3, 4]) -> [(1, 2), (2, 3), (3, 4)]
  itertools.pairwise("abc") -> [("a", "b"), ("b", "c")]`,
	},
	"batched": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			args, kerr := kwargsToPositional("batched", args, kwargs, "iterable", "n")
			if kerr != nil {
				return kerr
			}
			// batched(iterable, n) - Batch elements into tuples of size n
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			var elements []object.Object
			switch a := args[0].(type) {
			case *object.List:
				elements = a.Elements
			case *object.Tuple:
				elements = a.Elements
			case *object.String:
				for _, ch := range a.StringValue() {
					elements = append(elements, object.NewString(string(ch)))
				}
			default:
				elems, errObj := collectIterable(ctx, args[0])
				if errObj != nil {
					return errObj
				}
				elements = elems
			}

			n, ok := args[1].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			if n.IntValue() <= 0 {
				return errors.NewValueError("n must be at least one")
			}

			batchSize := int(n.IntValue())
			result := []object.Object{}
			for i := 0; i < len(elements); i += batchSize {
				end := i + batchSize
				if end > len(elements) {
					end = len(elements)
				}
				batch := make([]object.Object, end-i)
				copy(batch, elements[i:end])
				result = append(result, &object.Tuple{Elements: batch})
			}
			return &object.List{Elements: result}
		},
		HelpText: `batched(iterable, n) - Batch elements into tuples of size n

Groups elements into batches of n elements each.

Example:
  itertools.batched([1, 2, 3, 4, 5], 2) -> [(1, 2), (3, 4), (5,)]
  itertools.batched("abcdef", 3) -> [("a", "b", "c"), ("d", "e", "f")]`,
	},
}, nil, "Python-compatible itertools library for iteration utilities")

// Helper functions for permutations and combinations

func generatePermutations(elements []object.Object, r int, current []object.Object, used []bool, result *[]object.Object) {
	if len(current) == r {
		tuple := make([]object.Object, r)
		copy(tuple, current)
		*result = append(*result, &object.Tuple{Elements: tuple})
		return
	}
	for i := 0; i < len(elements); i++ {
		if !used[i] {
			used[i] = true
			current = append(current, elements[i])
			generatePermutations(elements, r, current, used, result)
			current = current[:len(current)-1]
			used[i] = false
		}
	}
}

func generateCombinations(elements []object.Object, r int, start int, current []object.Object, result *[]object.Object) {
	if len(current) == r {
		tuple := make([]object.Object, r)
		copy(tuple, current)
		*result = append(*result, &object.Tuple{Elements: tuple})
		return
	}
	for i := start; i < len(elements); i++ {
		current = append(current, elements[i])
		generateCombinations(elements, r, i+1, current, result)
		current = current[:len(current)-1]
	}
}

func generateCombinationsWithReplacement(elements []object.Object, r int, start int, current []object.Object, result *[]object.Object) {
	if len(current) == r {
		tuple := make([]object.Object, r)
		copy(tuple, current)
		*result = append(*result, &object.Tuple{Elements: tuple})
		return
	}
	for i := start; i < len(elements); i++ {
		current = append(current, elements[i])
		generateCombinationsWithReplacement(elements, r, i, current, result)
		current = current[:len(current)-1]
	}
}

// Helper function to compare objects for equality
func objectsEqual(a, b object.Object) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch av := a.(type) {
	case *object.Integer:
		if bv, ok := b.(*object.Integer); ok {
			return av.IntValue() == bv.IntValue()
		}
	case *object.Float:
		if bv, ok := b.(*object.Float); ok {
			return av.FloatValue() == bv.FloatValue()
		}
	case *object.String:
		if bv, ok := b.(*object.String); ok {
			return av.StringValue() == bv.StringValue()
		}
	case *object.Boolean:
		if bv, ok := b.(*object.Boolean); ok {
			return av.BoolValue() == bv.BoolValue()
		}
	case *object.Null:
		_, ok := b.(*object.Null)
		return ok
	}
	return a.Inspect() == b.Inspect()
}

// Helper function to add two objects (for accumulate default)
func addObjects(a, b object.Object) object.Object {
	switch av := a.(type) {
	case *object.Integer:
		switch bv := b.(type) {
		case *object.Integer:
			return object.NewInteger(av.IntValue() + bv.IntValue())
		case *object.Float:
			return object.NewFloat(float64(av.IntValue()) + bv.FloatValue())
		}
	case *object.Float:
		switch bv := b.(type) {
		case *object.Integer:
			return object.NewFloat(av.FloatValue() + float64(bv.IntValue()))
		case *object.Float:
			return object.NewFloat(av.FloatValue() + bv.FloatValue())
		}
	case *object.String:
		if bv, ok := b.(*object.String); ok {
			return object.NewString(av.StringValue() + bv.StringValue())
		}
	case *object.List:
		if bv, ok := b.(*object.List); ok {
			newElements := make([]object.Object, len(av.Elements)+len(bv.Elements))
			copy(newElements, av.Elements)
			copy(newElements[len(av.Elements):], bv.Elements)
			return &object.List{Elements: newElements}
		}
	}
	return errors.NewError("cannot add %s and %s", a.Type(), b.Type())
}

func isTruthy(obj object.Object) bool {
	switch v := obj.(type) {
	case *object.Null:
		return false
	case *object.Boolean:
		return v.BoolValue()
	case *object.Integer:
		return v.IntValue() != 0
	case *object.Float:
		return v.FloatValue() != 0
	case *object.String:
		return len(v.StringValue()) > 0
	case *object.List:
		return len(v.Elements) > 0
	case *object.Dict:
		return len(v.Pairs) > 0
	default:
		return true
	}
}

// parseIsliceBounds reads islice(iterable, stop) or islice(iterable,
// start, stop[, step]) bounds.
func parseIsliceBounds(args []object.Object, start, stop, step *int64) object.Object {
	if len(args) == 2 {
		if s, ok := args[1].(*object.Integer); ok {
			*stop = s.IntValue()
		} else {
			return errors.NewTypeError("INTEGER", args[1].Type().String())
		}
		return nil
	}
	if s, ok := args[1].(*object.Integer); ok {
		*start = s.IntValue()
	} else {
		return errors.NewTypeError("INTEGER", args[1].Type().String())
	}
	if s, ok := args[2].(*object.Integer); ok {
		*stop = s.IntValue()
	} else {
		return errors.NewTypeError("INTEGER", args[2].Type().String())
	}
	if len(args) == 4 {
		if s, ok := args[3].(*object.Integer); ok {
			*step = s.IntValue()
		} else {
			return errors.NewTypeError("INTEGER", args[3].Type().String())
		}
	}
	return nil
}

func isEndless(obj object.Object) bool {
	it, ok := obj.(*object.Iterator)
	return ok && it.Infinite()
}

// collectIterable gathers an argument's elements. A script object is
// iterated through __iter__ (and __next__), and an iterator is pulled to its
// end; an error or exception raised while pulling it is returned, as is
// an endless iterator, which would never finish.
func collectIterable(ctx context.Context, obj object.Object) ([]object.Object, object.Object) {
	if inst, ok := obj.(*object.Instance); ok {
		return instanceElements(ctx, inst)
	}
	if it, ok := obj.(*object.Iterator); ok {
		if it.Infinite() {
			return nil, errors.NewError("%s", object.InfiniteIteratorMessage)
		}
		var elems []object.Object
		for {
			v, more := it.Next()
			if !more {
				return elems, nil
			}
			if object.IsPropagating(v) {
				return nil, v
			}
			elems = append(elems, v)
		}
	}
	elems, ok := object.IterableToSlice(obj)
	if !ok {
		return nil, notIterableError(obj)
	}
	return elems, nil
}

// notIterableError is the error for an argument that cannot be collected
// into a list: a clear message for an infinite iterator, else a TypeError.
func notIterableError(obj object.Object) object.Object {
	if it, ok := obj.(*object.Iterator); ok && it.Infinite() {
		return errors.NewError("%s", object.InfiniteIteratorMessage)
	}
	return errors.NewTypeError("iterable", obj.Type().String())
}
