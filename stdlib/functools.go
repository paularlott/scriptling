package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

var FunctoolsLibrary = object.NewLibrary(FunctoolsLibraryName, map[string]*object.Builtin{
	"lru_cache": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// Both decorator forms: @lru_cache (bare, the function as the
			// first positional argument) and @lru_cache(maxsize=...) which
			// returns the decorator.
			if len(args) > 0 {
				switch args[0].(type) {
				case *object.Function, *object.LambdaFunction, *object.Builtin, *object.BoundMethod:
					return newLRUCacheWrapper(args[0], 128)
				}
				if len(args) > 1 {
					return errors.NewError("lru_cache() takes at most 1 argument")
				}
			}
			maxsize := int64(128)
			if v, ok := kwargs.Kwargs["maxsize"]; ok {
				switch mv := v.(type) {
				case *object.Integer:
					maxsize = mv.IntValue()
				case *object.Null:
					maxsize = -1 // unbounded
				default:
					return errors.NewError("maxsize should be an integer or None")
				}
			} else if len(args) == 1 {
				if iv, ok := args[0].(*object.Integer); ok {
					maxsize = iv.IntValue()
				} else if _, isNull := args[0].(*object.Null); isNull {
					maxsize = -1
				} else {
					return errors.NewError("maxsize should be an integer or None")
				}
			}
			// Decorator-with-args form: return the decorator.
			return &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 1 {
						return errors.NewError("lru_cache decorator requires 1 argument")
					}
					switch args[0].(type) {
					case *object.Function, *object.LambdaFunction, *object.Builtin, *object.BoundMethod:
						return newLRUCacheWrapper(args[0], maxsize)
					}
					return errors.NewTypeError("callable", args[0].Type().String())
				},
				HelpText: "lru_cache(maxsize) - decorator factory",
			}
		},
		HelpText: `lru_cache(user_function) or lru_cache(maxsize=128) - Memoizing decorator

Calls are cached by argument values; maxsize bounds the cache with
least-recently-used eviction (maxsize=None is unbounded). Unhashable
arguments bypass the cache, as in Python.`,
	},
	"cache": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewError("cache() requires 1 argument")
			}
			switch args[0].(type) {
			case *object.Function, *object.LambdaFunction, *object.Builtin, *object.BoundMethod:
				return newLRUCacheWrapper(args[0], -1)
			}
			return errors.NewTypeError("callable", args[0].Type().String())
		},
		HelpText: `cache(user_function) - Unbounded memoizing decorator (lru_cache(maxsize=None))`,
	},
	"reduce": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewError("reduce() requires 2 or 3 arguments")
			}

			callable := args[0]
			switch callable.(type) {
			case *object.Function, *object.LambdaFunction, *object.Builtin, *object.BoundMethod, *object.Class:
				// Any callable is accepted (functions, lambdas, builtins, etc.).
			default:
				return errors.NewTypeError("callable", callable.Type().String())
			}

			list, ok := args[1].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST", args[1].Type().String())
			}

			if len(list.Elements) == 0 {
				if len(args) == 3 {
					return args[2]
				}
				return errors.NewError("reduce() of empty sequence with no initial value")
			}

			var accumulator object.Object
			startIdx := 0
			if len(args) == 3 {
				accumulator = args[2]
			} else {
				accumulator = list.Elements[0]
				startIdx = 1
			}

			eval := evaliface.FromContext(ctx)
			if eval == nil {
				return errors.NewError("evaluator not available in context")
			}

			for i := startIdx; i < len(list.Elements); i++ {
				result := eval.CallObjectFunction(ctx, callable, []object.Object{accumulator, list.Elements[i]}, nil, nil)
				if result == nil {
					return errors.NewError("reduce function returned nil")
				}
				if object.IsError(result) || result.Type() == object.EXCEPTION_OBJ {
					return result
				}
				accumulator = result
			}

			return accumulator
		},
		HelpText: `reduce(function, iterable[, initializer]) - Apply function cumulatively to items

Parameters:
  function    - Function taking 2 arguments (accumulator, item)
  iterable    - List of items to reduce
  initializer - Optional starting value

Returns: Reduced value

Example:
  import functools

  def add(x, y):
      return x + y

  functools.reduce(add, [1, 2, 3, 4])  # 10
  functools.reduce(add, [1, 2, 3], 10)  # 16`,
	},
	"partial": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 {
				return errors.NewError("partial() requires at least 1 argument")
			}

			var fn *object.Function
			var lambda *object.LambdaFunction
			var builtin *object.Builtin
			if f, ok := args[0].(*object.Function); ok {
				fn = f
			} else if l, ok := args[0].(*object.LambdaFunction); ok {
				lambda = l
			} else if b, ok := args[0].(*object.Builtin); ok {
				builtin = b
			} else {
				return errors.NewTypeError("FUNCTION", args[0].Type().String())
			}

			// Copy args[1:] so the returned Builtin doesn't alias the caller's
			// buffer (the closure retains partialArgs across the call).
			partialArgs := make([]object.Object, len(args)-1)
			copy(partialArgs, args[1:])
			partialKwargs := make(map[string]object.Object)
			for k, v := range kwargs.Kwargs {
				partialKwargs[k] = v
			}

			return &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					allArgs := make([]object.Object, len(partialArgs)+len(args))
					copy(allArgs, partialArgs)
					copy(allArgs[len(partialArgs):], args)
					allKwargs := make(map[string]object.Object)
					for k, v := range partialKwargs {
						allKwargs[k] = v
					}
					for k, v := range kwargs.Kwargs {
						allKwargs[k] = v
					}

					if fn != nil {
						eval := evaliface.FromContext(ctx)
						if eval == nil {
							return errors.NewError("evaluator not available in context")
						}
						return eval.CallFunction(ctx, fn, allArgs, allKwargs)
					}
					if lambda != nil {
						eval := evaliface.FromContext(ctx)
						if eval == nil {
							return errors.NewError("evaluator not available in context")
						}
						return eval.CallObjectFunction(ctx, lambda, allArgs, allKwargs, nil)
					}
					return builtin.Fn(ctx, object.NewKwargs(allKwargs), allArgs...)
				},
				HelpText: "Partial function application",
			}
		},
		HelpText: `partial(func, *args, **kwargs) - Create a partial function application

Parameters:
  func - Function to partially apply
  *args - Arguments to pre-fill
  **kwargs - Keyword arguments to pre-fill

Returns: New function with pre-filled arguments

Example:
  import functools

  def add(x, y):
      return x + y

  add_five = functools.partial(add, 5)
  add_five(3)  # 8`,
	},
}, nil, "Higher-order functions and operations on callable objects")
