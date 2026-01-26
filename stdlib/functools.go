package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var FunctoolsLibrary = object.NewLibrary(FunctoolsLibraryName, map[string]*object.Builtin{
	"reduce": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewError("reduce() requires 2 or 3 arguments")
			}

			// First arg must be a function
			fn, ok := args[0].(*object.Function)
			if !ok {
				// Also check for lambda/builtin
				if builtin, ok := args[0].(*object.Builtin); ok {
					return reduceWithBuiltin(ctx, builtin, args[1:])
				}
				return errors.NewTypeError("FUNCTION", args[0].Type().String())
			}

			// Second arg must be an iterable (list)
			list, ok := args[1].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST", args[1].Type().String())
			}

			if len(list.Elements) == 0 {
				if len(args) == 3 {
					return args[2] // Return initializer
				}
				return errors.NewError("reduce() of empty sequence with no initial value")
			}

			// Get initial accumulator
			var accumulator object.Object
			startIdx := 0
			if len(args) == 3 {
				accumulator = args[2]
			} else {
				accumulator = list.Elements[0]
				startIdx = 1
			}

			// Apply function cumulatively
			for i := startIdx; i < len(list.Elements); i++ {
				// Create a new environment for function call
				fnEnv := object.NewEnclosedEnvironment(fn.Env)
				if len(fn.Parameters) != 2 {
					return errors.NewError("reduce function must take exactly 2 arguments")
				}
				fnEnv.Set(fn.Parameters[0].Value, accumulator)
				fnEnv.Set(fn.Parameters[1].Value, list.Elements[i])

				// Evaluate function body - we need the evaluator from context
				// Since we can't directly call the evaluator here, we'll store
				// a callable reference
				result := callFunction(ctx, fn, []object.Object{accumulator, list.Elements[i]}, nil)
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

			// First arg must be a function or builtin
			var fn *object.Function
			var builtin *object.Builtin
			if f, ok := args[0].(*object.Function); ok {
				fn = f
			} else if b, ok := args[0].(*object.Builtin); ok {
				builtin = b
			} else {
				return errors.NewTypeError("FUNCTION", args[0].Type().String())
			}

			// Remaining args are pre-filled arguments
			partialArgs := args[1:]
			partialKwargs := make(map[string]object.Object)
			for k, v := range kwargs.Kwargs {
				partialKwargs[k] = v
			}

			// Create a new builtin that calls the original with pre-filled args
			return &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					// Combine partial args with new args
					allArgs := append(partialArgs, args...)
					allKwargs := make(map[string]object.Object)
					for k, v := range partialKwargs {
						allKwargs[k] = v
					}
					for k, v := range kwargs.Kwargs {
						allKwargs[k] = v
					}

					if fn != nil {
						return callFunction(ctx, fn, allArgs, allKwargs)
					} else {
						return builtin.Fn(ctx, object.NewKwargs(allKwargs), allArgs...)
					}
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
			return args[1] // Return initializer
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

// callFunction is a helper that will be linked to the evaluator
// For now, we use a callback approach
var callFunction func(ctx context.Context, fn *object.Function, args []object.Object, keywords map[string]object.Object) object.Object

func init() {
	// Default implementation - will be overridden by evaluator
	callFunction = func(ctx context.Context, fn *object.Function, args []object.Object, keywords map[string]object.Object) object.Object {
		return errors.NewError("function calling not initialized")
	}
}

// SetFunctionCaller allows the evaluator to register its function caller
func SetFunctionCaller(caller func(ctx context.Context, fn *object.Function, args []object.Object, keywords map[string]object.Object) object.Object) {
	callFunction = caller
}
