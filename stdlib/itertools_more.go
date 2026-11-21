package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// itertools.chain.from_iterable and itertools.tee, and functools.cmp_to_key.

// lazySource returns a pull function over an iterable: an iterator or a
// script iterable is pulled as it is consumed, anything else is collected.
func lazySource(ctx context.Context, obj object.Object) (func() (object.Object, bool), object.Object) {
	switch o := obj.(type) {
	case *object.Iterator:
		return o.Next, nil
	case *object.List, *object.Tuple, *object.String:
		next, _ := object.IterSource(o)
		return next, nil
	case *object.Instance:
		it, errObj := instanceAsIterator(ctx, o)
		if errObj != nil {
			return nil, errObj
		}
		if it != nil {
			return it.Next, nil
		}
	}
	elems, errObj := collectIterable(ctx, obj)
	if errObj != nil {
		return nil, errObj
	}
	next, _ := object.IterSource(&object.List{Elements: elems})
	return next, nil
}

func init() {
	fns := ItertoolsLibrary.Functions()
	if fns["chain"].Attributes == nil {
		fns["chain"].Attributes = map[string]object.Object{}
	}
	fns["chain"].Attributes["from_iterable"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("from_iterable", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			outer, errObj := lazySource(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			var inner func() (object.Object, bool)
			it := object.NewIterator(func() (object.Object, bool) {
				for {
					if inner != nil {
						if v, ok := inner(); ok {
							return v, true
						}
						inner = nil
					}
					sub, ok := outer()
					if !ok {
						return nil, false
					}
					if object.IsPropagating(sub) {
						return sub, true
					}
					next, errObj := lazySource(ctx, sub)
					if errObj != nil {
						return errObj, true
					}
					inner = next
				}
			})
			it.SetTypeName(object.NewString("chain"))
			return it
		},
		HelpText: "chain.from_iterable(iterable) - Chain the iterables an iterable yields, lazily",
	}
	fns["tee"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("tee", kwargs, "n"); err != nil {
					return err
				}
			}
			if len(args) < 1 || len(args) > 2 {
				return errors.NewTypeErrorTagged("tee expected 1 or 2 arguments, got %d", len(args))
			}
			n := int64(2)
			nObj := kwargs.Get("n")
			if len(args) == 2 {
				nObj = args[1]
			}
			if nObj != nil {
				v, ok := nObj.(*object.Integer)
				if !ok {
					return errors.NewTypeErrorTagged("'%s' object cannot be interpreted as an integer", object.PythonTypeName(nObj))
				}
				n = v.IntValue()
			}
			if n < 0 {
				return errors.NewValueError("n must be >= 0")
			}
			if errObj := object.CheckElements(n); errObj != nil {
				return errObj
			}
			src, errObj := lazySource(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			// Each copy has a queue of the values it has yet to yield; a
			// value pulled from the source joins every queue.
			queues := make([][]object.Object, n)
			done := false
			copies := make([]object.Object, n)
			for i := range copies {
				i := i
				it := object.NewIterator(func() (object.Object, bool) {
					if len(queues[i]) == 0 {
						if done {
							return nil, false
						}
						v, ok := src()
						if !ok {
							done = true
							return nil, false
						}
						for j := range queues {
							queues[j] = append(queues[j], v)
						}
					}
					v := queues[i][0]
					queues[i][0] = nil
					queues[i] = queues[i][1:]
					return v, true
				})
				it.SetTypeName(object.NewString("_tee"))
				copies[i] = it
			}
			return newTrackedTuple(copies)
		},
		HelpText: "tee(iterable, n=2) - n independent iterators over one iterable",
	}

	FunctoolsLibrary.Functions()["cmp_to_key"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("cmp_to_key", kwargs, "mycmp"); err != nil {
					return err
				}
				if v := kwargs.Get("mycmp"); v != nil && len(args) == 0 {
					args = []object.Object{v}
				}
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			cmp := args[0]
			if !isCallable(cmp) {
				return errors.NewTypeErrorTagged("'%s' object is not callable", object.PythonTypeName(cmp))
			}
			return &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("K", kwargs); err != nil {
						return err
					}
					if err := errors.ExactArgs(args, 1); err != nil {
						return err
					}
					return object.NewInstanceWithFields(cmpKeyClass, map[string]object.Object{"obj": args[0], "_cmp": cmp})
				},
				HelpText: "K(obj) - A sort key comparing through the cmp function",
			}
		},
		HelpText: `cmp_to_key(mycmp) - Turn a cmp(a, b) function (negative, zero or positive) into a key function

  sorted(words, key=functools.cmp_to_key(lambda a, b: len(a) - len(b)))`,
	}
}

// cmpKeyClass is the class of functools.cmp_to_key's keys: each compares by
// calling the cmp function on the wrapped objects.
var cmpKeyClass = &object.Class{Name: "functools.KeyWrapper", Methods: map[string]object.Object{}}

func init() {
	ops := map[string]func(int64) bool{
		"__lt__": func(c int64) bool { return c < 0 },
		"__gt__": func(c int64) bool { return c > 0 },
		"__le__": func(c int64) bool { return c <= 0 },
		"__ge__": func(c int64) bool { return c >= 0 },
		"__eq__": func(c int64) bool { return c == 0 },
		"__ne__": func(c int64) bool { return c != 0 },
	}
	for name, test := range ops {
		name, test := name, test
		cmpKeyClass.Methods[name] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 2 {
					return errors.NewArgumentError(len(args), 2)
				}
				l, lok := args[0].(*object.Instance)
				r, rok := args[1].(*object.Instance)
				if !lok || !rok || r.Class != cmpKeyClass {
					return errors.NewTypeErrorTagged("other argument must be K instance")
				}
				res := callCallable(ctx, l.Field("_cmp"), l.Field("obj"), r.Field("obj"))
				if object.IsPropagating(res) {
					return res
				}
				var c int64
				switch v := res.(type) {
				case *object.Integer:
					c = v.IntValue()
				case *object.Float:
					switch f := v.FloatValue(); {
					case f < 0:
						c = -1
					case f > 0:
						c = 1
					}
				case *object.Boolean:
					if v.BoolValue() {
						c = 1
					}
				default:
					return errors.NewTypeErrorTagged("comparison function must return a number, not '%s'", object.PythonTypeName(res))
				}
				return object.NewBoolean(test(c))
			},
			HelpText: name + "(other) - Compare through the cmp function",
		}
	}
}
