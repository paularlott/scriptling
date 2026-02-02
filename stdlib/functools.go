package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

var FunctoolsLibrary = object.NewLibrary(FunctoolsLibraryName, map[string]*object.Builtin{
	"reduce": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewError("reduce() requires 2 or 3 arguments")
			}

			fn, ok := args[0].(*object.Function)
			if !ok {
				if builtin, ok := args[0].(*object.Builtin); ok {
					return reduceWithBuiltin(ctx, builtin, args[1:])
				}
				return errors.NewTypeError("FUNCTION", args[0].Type().String())
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
				result := eval.CallFunction(ctx, fn, []object.Object{accumulator, list.Elements[i]}, nil)
				if result == nil {
					return errors.NewError("reduce function returned nil")
				}
				if object.IsError(result) {
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
			var builtin *object.Builtin
			if f, ok := args[0].(*object.Function); ok {
				fn = f
			} else if b, ok := args[0].(*object.Builtin); ok {
				builtin = b
			} else {
				return errors.NewTypeError("FUNCTION", args[0].Type().String())
			}

			partialArgs := args[1:]
			partialKwargs := make(map[string]object.Object)
			for k, v := range kwargs.Kwargs {
				partialKwargs[k] = v
			}

			return &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					allArgs := append(partialArgs, args...)
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

func reduceWithBuiltin(ctx context.Context, builtin *object.Builtin, args []object.Object) object.Object {
	list, ok := args[0].(*object.List)
	if !ok {
		return errors.NewTypeError("LIST", args[0].Type().String())
	}

	if len(list.Elements) == 0 {
		if len(args) == 2 {
			return args[1]
		}
		return errors.NewError("reduce() of empty sequence with no initial value")
	}

	var accumulator object.Object
	startIdx := 0
	if len(args) == 2 {
		accumulator = args[1]
	} else {
		accumulator = list.Elements[0]
		startIdx = 1
	}

	for i := startIdx; i < len(list.Elements); i++ {
		result := builtin.Fn(ctx, object.NewKwargs(nil), accumulator, list.Elements[i])
		if result == nil {
			return errors.NewError("reduce function returned nil")
		}
		if object.IsError(result) {
			return result
		}
		accumulator = result
	}

	return accumulator
}
