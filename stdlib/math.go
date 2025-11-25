package stdlib

import (
	"context"
	"math"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var mathLibrary = object.NewLibrary(map[string]*object.Builtin{
	"sqrt": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Float{Value: math.Sqrt(float64(arg.Value))}
			case *object.Float:
				return &object.Float{Value: math.Sqrt(arg.Value)}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
			}
		},
	},
	"pow": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			var base, exp float64
			switch arg := args[0].(type) {
			case *object.Integer:
				base = float64(arg.Value)
			case *object.Float:
				base = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
			}
			switch arg := args[1].(type) {
			case *object.Integer:
				exp = float64(arg.Value)
			case *object.Float:
				exp = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
			}
			return &object.Float{Value: math.Pow(base, exp)}
		},
	},
	"abs": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				if arg.Value < 0 {
					return &object.Integer{Value: -arg.Value}
				}
				return arg
			case *object.Float:
				return &object.Float{Value: math.Abs(arg.Value)}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
			}
		},
	},
	"floor": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return arg
			case *object.Float:
				return &object.Integer{Value: int64(math.Floor(arg.Value))}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
			}
		},
	},
	"ceil": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return arg
			case *object.Float:
				return &object.Integer{Value: int64(math.Ceil(arg.Value))}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
			}
		},
	},
	"round": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return arg
			case *object.Float:
				return &object.Integer{Value: int64(math.Round(arg.Value))}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
			}
		},
	},
	"min": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			// Track if all inputs are integers
			allIntegers := true
			// Get first value
			var result float64
			switch arg := args[0].(type) {
			case *object.Integer:
				result = float64(arg.Value)
			case *object.Float:
				result = arg.Value
				allIntegers = false
			default:
				return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
			}
			// Compare with remaining values
			for i := 1; i < len(args); i++ {
				var val float64
				switch arg := args[i].(type) {
				case *object.Integer:
					val = float64(arg.Value)
				case *object.Float:
					val = arg.Value
					allIntegers = false
				default:
					return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
				}
				result = math.Min(result, val)
			}
			// Return integer if all inputs were integers
			if allIntegers {
				return &object.Integer{Value: int64(result)}
			}
			return &object.Float{Value: result}
		},
	},
	"max": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			// Track if all inputs are integers
			allIntegers := true
			// Get first value
			var result float64
			switch arg := args[0].(type) {
			case *object.Integer:
				result = float64(arg.Value)
			case *object.Float:
				result = arg.Value
				allIntegers = false
			default:
				return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
			}
			// Compare with remaining values
			for i := 1; i < len(args); i++ {
				var val float64
				switch arg := args[i].(type) {
				case *object.Integer:
					val = float64(arg.Value)
				case *object.Float:
					val = arg.Value
					allIntegers = false
				default:
					return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
				}
				result = math.Max(result, val)
			}
			// Return integer if all inputs were integers
			if allIntegers {
				return &object.Integer{Value: int64(result)}
			}
			return &object.Float{Value: result}
		},
	},
	"pi": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			return &object.Float{Value: math.Pi}
		},
	},
	"e": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			return &object.Float{Value: math.E}
		},
	},
})

func GetMathLibrary() *object.Library {
	return mathLibrary
}
