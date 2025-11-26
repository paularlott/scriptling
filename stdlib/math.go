package stdlib

import (
	"context"
	"math"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// gcd calculates the greatest common divisor using Euclidean algorithm
func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

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
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
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
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
			switch arg := args[1].(type) {
			case *object.Integer:
				exp = float64(arg.Value)
			case *object.Float:
				exp = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
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
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
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
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
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
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
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
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
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
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
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
					return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
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
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
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
					return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
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
	"sin": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Float{Value: math.Sin(float64(arg.Value))}
			case *object.Float:
				return &object.Float{Value: math.Sin(arg.Value)}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
		},
	},
	"cos": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Float{Value: math.Cos(float64(arg.Value))}
			case *object.Float:
				return &object.Float{Value: math.Cos(arg.Value)}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
		},
	},
	"tan": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Float{Value: math.Tan(float64(arg.Value))}
			case *object.Float:
				return &object.Float{Value: math.Tan(arg.Value)}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
		},
	},
	"log": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				if arg.Value <= 0 {
					return errors.NewError("log: domain error")
				}
				return &object.Float{Value: math.Log(float64(arg.Value))}
			case *object.Float:
				if arg.Value <= 0 {
					return errors.NewError("log: domain error")
				}
				return &object.Float{Value: math.Log(arg.Value)}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
		},
	},
	"exp": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Float{Value: math.Exp(float64(arg.Value))}
			case *object.Float:
				return &object.Float{Value: math.Exp(arg.Value)}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
		},
	},
	"degrees": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Float{Value: float64(arg.Value) * 180.0 / math.Pi}
			case *object.Float:
				return &object.Float{Value: arg.Value * 180.0 / math.Pi}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
		},
	},
	"radians": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Float{Value: float64(arg.Value) * math.Pi / 180.0}
			case *object.Float:
				return &object.Float{Value: arg.Value * math.Pi / 180.0}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
		},
	},
	"fmod": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			var x, y float64
			switch arg := args[0].(type) {
			case *object.Integer:
				x = float64(arg.Value)
			case *object.Float:
				x = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
			switch arg := args[1].(type) {
			case *object.Integer:
				y = float64(arg.Value)
			case *object.Float:
				y = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
			if y == 0 {
				return errors.NewError("fmod: division by zero")
			}
			return &object.Float{Value: math.Mod(x, y)}
		},
	},
	"gcd": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			var a, b int64
			switch arg := args[0].(type) {
			case *object.Integer:
				a = arg.Value
			default:
				return errors.NewTypeError("INTEGER", arg.Type().String())
			}
			switch arg := args[1].(type) {
			case *object.Integer:
				b = arg.Value
			default:
				return errors.NewTypeError("INTEGER", arg.Type().String())
			}
			return &object.Integer{Value: gcd(a, b)}
		},
	},
	"factorial": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				n := arg.Value
				if n < 0 {
					return errors.NewError("factorial: negative number")
				}
				if n > 20 {
					return errors.NewError("factorial: result too large")
				}
				result := int64(1)
				for i := int64(2); i <= n; i++ {
					result *= i
				}
				return &object.Integer{Value: result}
			default:
				return errors.NewTypeError("INTEGER", arg.Type().String())
			}
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
