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

// minMaxFunction returns min or max of arguments
func minMaxFunction(isMin bool) func(context.Context, map[string]object.Object, ...object.Object) object.Object {
	return func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
			if isMin {
				result = math.Min(result, val)
			} else {
				result = math.Max(result, val)
			}
		}
		// Return integer if all inputs were integers
		if allIntegers {
			return &object.Integer{Value: int64(result)}
		}
		return &object.Float{Value: result}
	}
}

// twoFloatFunc creates a function that takes two floats and applies f
func twoFloatFunc(f func(float64, float64) float64) func(context.Context, map[string]object.Object, ...object.Object) object.Object {
	return func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		return &object.Float{Value: f(x, y)}
	}
}

var MathLibrary = object.NewLibrary(map[string]*object.Builtin{
	"sqrt": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `sqrt(x) - Return the square root of x

x must be a non-negative number (integer or float).
Returns a float.`,
	},
	"pow": {
		Fn: twoFloatFunc(math.Pow),
		HelpText: `pow(base, exp) - Return base raised to the power exp

base and exp can be integers or floats.
Returns a float.`,
	},
	"abs": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `abs(x) - Return the absolute value of x

x can be an integer or float.
Returns the same type as input.`,
	},
	"floor": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `floor(x) - Return the floor of x

x can be an integer or float.
Returns the largest integer less than or equal to x.`,
	},
	"ceil": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `ceil(x) - Return the ceiling of x

x can be an integer or float.
Returns the smallest integer greater than or equal to x.`,
	},
	"round": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `round(x) - Return the nearest integer to x

x can be an integer or float.
Rounds to the nearest integer, with ties rounding away from zero.`,
	},
	"min": {
		Fn: minMaxFunction(true),
		HelpText: `min(*args) - Return the minimum value

Takes two or more numbers (integers or floats).
Returns the smallest value, preserving type if all integers.`,
	},
	"max": {
		Fn: minMaxFunction(false),
		HelpText: `max(*args) - Return the maximum value

Takes two or more numbers (integers or floats).
Returns the largest value, preserving type if all integers.`,
	},
	"sin": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `sin(x) - Return the sine of x (radians)

x can be an integer or float in radians.
Returns a float.`,
	},
	"cos": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `cos(x) - Return the cosine of x (radians)

x can be an integer or float in radians.
Returns a float.`,
	},
	"tan": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `tan(x) - Return the tangent of x (radians)

x can be an integer or float in radians.
Returns a float.`,
	},
	"log": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `log(x) - Return the natural logarithm of x

x must be positive (integer or float).
Returns a float.`,
	},
	"exp": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `exp(x) - Return e raised to the power x

x can be an integer or float.
Returns a float.`,
	},
	"degrees": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `degrees(x) - Convert radians to degrees

x can be an integer or float in radians.
Returns a float in degrees.`,
	},
	"radians": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `radians(x) - Convert degrees to radians

x can be an integer or float in degrees.
Returns a float in radians.`,
	},
	"fmod": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `fmod(x, y) - Return the floating-point remainder of x/y

x and y can be integers or floats.
y must not be zero. Returns a float.`,
	},
	"gcd": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `gcd(a, b) - Return the greatest common divisor of a and b

a and b must be integers.
Returns an integer.`,
	},
	"factorial": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `factorial(n) - Return n!

n must be a non-negative integer <= 20.
Returns an integer.`,
	},
	"isnan": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Boolean{Value: false}
			case *object.Float:
				return &object.Boolean{Value: math.IsNaN(arg.Value)}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
		},
		HelpText: `isnan(x) - Check if x is NaN (Not a Number)

Returns True if x is NaN, False otherwise.`,
	},
	"isinf": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Boolean{Value: false}
			case *object.Float:
				return &object.Boolean{Value: math.IsInf(arg.Value, 0)}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
		},
		HelpText: `isinf(x) - Check if x is infinite

Returns True if x is positive or negative infinity.`,
	},
	"isfinite": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return &object.Boolean{Value: true}
			case *object.Float:
				return &object.Boolean{Value: !math.IsNaN(arg.Value) && !math.IsInf(arg.Value, 0)}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
		},
		HelpText: `isfinite(x) - Check if x is finite

Returns True if x is neither NaN nor infinite.`,
	},
	"copysign": {
		Fn: twoFloatFunc(math.Copysign),
		HelpText: `copysign(x, y) - Return x with the sign of y

Returns a float with magnitude of x and sign of y.`,
	},
	"trunc": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return arg
			case *object.Float:
				return &object.Integer{Value: int64(math.Trunc(arg.Value))}
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
		},
		HelpText: `trunc(x) - Truncate x to the nearest integer toward zero

Returns an integer.`,
	},
	"log10": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			var x float64
			switch arg := args[0].(type) {
			case *object.Integer:
				x = float64(arg.Value)
			case *object.Float:
				x = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
			return &object.Float{Value: math.Log10(x)}
		},
		HelpText: `log10(x) - Return the base-10 logarithm of x

x must be positive. Returns a float.`,
	},
	"log2": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			var x float64
			switch arg := args[0].(type) {
			case *object.Integer:
				x = float64(arg.Value)
			case *object.Float:
				x = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
			return &object.Float{Value: math.Log2(x)}
		},
		HelpText: `log2(x) - Return the base-2 logarithm of x

x must be positive. Returns a float.`,
	},
	"hypot": {
		Fn: twoFloatFunc(math.Hypot),
		HelpText: `hypot(x, y) - Return the Euclidean distance sqrt(x*x + y*y)

Returns a float.`,
	},
	"asin": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			var x float64
			switch arg := args[0].(type) {
			case *object.Integer:
				x = float64(arg.Value)
			case *object.Float:
				x = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
			return &object.Float{Value: math.Asin(x)}
		},
		HelpText: `asin(x) - Return the arc sine of x in radians

x must be in the range [-1, 1]. Returns a float.`,
	},
	"acos": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			var x float64
			switch arg := args[0].(type) {
			case *object.Integer:
				x = float64(arg.Value)
			case *object.Float:
				x = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
			return &object.Float{Value: math.Acos(x)}
		},
		HelpText: `acos(x) - Return the arc cosine of x in radians

x must be in the range [-1, 1]. Returns a float.`,
	},
	"atan": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			var x float64
			switch arg := args[0].(type) {
			case *object.Integer:
				x = float64(arg.Value)
			case *object.Float:
				x = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
			return &object.Float{Value: math.Atan(x)}
		},
		HelpText: `atan(x) - Return the arc tangent of x in radians

Returns a float in the range [-pi/2, pi/2].`,
	},
	"atan2": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			var y, x float64
			switch arg := args[0].(type) {
			case *object.Integer:
				y = float64(arg.Value)
			case *object.Float:
				y = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
			switch arg := args[1].(type) {
			case *object.Integer:
				x = float64(arg.Value)
			case *object.Float:
				x = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
			}
			return &object.Float{Value: math.Atan2(y, x)}
		},
		HelpText: `atan2(y, x) - Return the arc tangent of y/x in radians

Correctly handles the quadrant of the result.
Returns a float in the range [-pi, pi].`,
	},
}, map[string]object.Object{
	"pi":  &object.Float{Value: math.Pi},
	"e":   &object.Float{Value: math.E},
	"inf": &object.Float{Value: math.Inf(1)},
	"nan": &object.Float{Value: math.NaN()},
}, "Mathematical functions library")
