package stdlib

import (
	"context"
	"unsafe"

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
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("lru_cache", kwargs, "maxsize", "typed"); err != nil {
					return err
				}
			}
			if len(args) > 0 {
				switch args[0].(type) {
				case *object.Function, *object.LambdaFunction, *object.Builtin, *object.BoundMethod:
					return newLRUCacheWrapper(args[0], 128)
				}
				if len(args) > 1 {
					return errors.NewTypeErrorTagged("lru_cache() takes at most 1 argument")
				}
			}
			maxsize := int64(128)
			if v, ok := kwargs.Lookup("maxsize"); ok {
				switch mv := v.(type) {
				case *object.Integer:
					maxsize = mv.IntValue()
				case *object.Null:
					maxsize = -1 // unbounded
				default:
					return errors.NewTypeErrorTagged("Expected first argument to be an integer, a callable, or None")
				}
			} else if len(args) == 1 {
				if iv, ok := args[0].(*object.Integer); ok {
					maxsize = iv.IntValue()
				} else if _, isNull := args[0].(*object.Null); isNull {
					maxsize = -1
				} else {
					return errors.NewTypeErrorTagged("Expected first argument to be an integer, a callable, or None")
				}
			}
			// Decorator-with-args form: return the decorator.
			return &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("lru_cache", kwargs); err != nil {
						return err
					}
					if len(args) != 1 {
						return errors.NewTypeErrorTagged("lru_cache decorator requires 1 argument")
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
	"wraps": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// wraps(wrapped) returns a decorator that copies the wrapped
			// callable's identity (__name__ above all) onto the wrapper.
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("wraps", kwargs, "assigned", "updated"); err != nil {
					return err
				}
			}
			if len(args) != 1 {
				return errors.NewTypeErrorTagged("wraps() missing 1 required positional argument: 'wrapped'")
			}
			wrapped := args[0]
			name := "<unknown>"
			switch w := wrapped.(type) {
			case *object.Function:
				name = w.Name
			case *object.LambdaFunction:
				name = "<lambda>"
			case *object.Builtin:
				if w.Repr != "" {
					name = w.Repr
				}
			}
			return &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("wraps", kwargs); err != nil {
						return err
					}
					if len(args) != 1 {
						return errors.NewTypeErrorTagged("wraps decorator requires the wrapper")
					}
					switch wr := args[0].(type) {
					case *object.Builtin:
						if wr.Attributes == nil {
							wr.Attributes = map[string]object.Object{}
						}
						wr.Attributes["__name__"] = object.NewString(name)
						wr.Attributes["__wrapped__"] = wrapped
					case *object.Instance:
						wr.SetField("__name__", object.NewString(name))
						wr.SetField("__wrapped__", wrapped)
					case *object.Function:
						// Each call of the decorator defines a new wrapper
						// function, so renaming it changes only this one; a
						// method wrapped in a class body is then registered
						// under the method's own name, as in Python. __doc__
						// and __wrapped__ read through Wrapped.
						if name != "<unknown>" {
							wr.Name = name
						}
						wr.SetWrapped(wrapped)
					}
					return args[0]
				},
				HelpText: "wraps decorator - copy identity onto the wrapper",
			}
		},
		HelpText: `wraps(wrapped) - Decorator factory preserving the wrapped function's identity

The wrapper takes the wrapped function's __name__ and __doc__, and
__wrapped__ refers to the wrapped function.

The standard decorator pattern:

  def deco(fn):
      @functools.wraps(fn)
      def wrapper(*a, **kw):
          return fn(*a, **kw)
      return wrapper`,
	},
	"cache": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("cache", kwargs); err != nil {
				return err
			}
			if len(args) != 1 {
				return errors.NewTypeErrorTagged("cache() missing 1 required positional argument: 'user_function'")
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
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("reduce", kwargs, "initial"); err != nil {
					return err
				}
				if len(args) == 2 {
					args = append(args[:2:2], kwargs.Get("initial"))
				} else if len(args) == 3 {
					return errors.NewTypeErrorTagged("reduce() got multiple values for argument 'initial'")
				}
			}
			if len(args) < 2 || len(args) > 3 {
				return errors.NewTypeErrorTagged("reduce() takes from 2 to 3 positional arguments but %d were given", len(args))
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
				// Any other iterable (tuple, range, set, generator, ...). An
				// error raised while pulling a generator or iterator propagates.
				elems, errObj := collectIterable(ctx, args[1])
				if errObj != nil {
					switch args[1].(type) {
					case *object.Instance, *object.Iterator:
						return errObj
					}
					return errors.NewTypeErrorTagged("reduce() arg 2 must support iteration")
				}
				list = &object.List{Elements: elems}
			}

			if len(list.Elements) == 0 {
				if len(args) == 3 {
					return args[2]
				}
				return errors.NewTypeErrorTagged("reduce() of empty iterable with no initial value")
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
				return errors.NewTypeErrorTagged("type 'partial' takes at least one argument")
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
			// The Kwargs a builtin is called with owns its storage and never
			// changes, so the partial keeps it as it is.
			partialKwargs := kwargs

			// A partial holds its arguments, which can be another partial:
			// account for it so a chain of them is within the memory limit.
			partial := &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					allArgs := make([]object.Object, len(partialArgs)+len(args))
					copy(allArgs, partialArgs)
					copy(allArgs[len(partialArgs):], args)
					allKwargs := partialKwargs.Merge(kwargs)

					if fn != nil || lambda != nil {
						eval := evaliface.FromContext(ctx)
						if eval == nil {
							return errors.NewError("evaluator not available in context")
						}
						var callee object.Object = lambda
						var env *object.Environment
						if fn != nil {
							callee, env = fn, fn.Env
						}
						if kc, ok := eval.(evaliface.KwargsCaller); ok {
							return kc.CallObjectFunctionKwargs(ctx, callee, allArgs, allKwargs, env)
						}
						if fn != nil {
							return eval.CallFunction(ctx, fn, allArgs, allKwargs.Map())
						}
						return eval.CallObjectFunction(ctx, lambda, allArgs, allKwargs.Map(), nil)
					}
					return builtin.Fn(ctx, allKwargs, allArgs...)
				},
				HelpText: "Partial function application",
			}
			object.TrackAlloc(partial, partialObjectSize+int64(len(partialArgs)+partialKwargs.Len())*16)
			return partial
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

// partialObjectSize approximates a partial: its Builtin, the closure and the
// keyword map header.
const partialObjectSize = int64(unsafe.Sizeof(object.Builtin{})) + 128
