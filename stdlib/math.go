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

// toFloat extracts a float64 from an Integer or Float object.
// Returns (value, ok) where ok is true if extraction succeeded.
func toFloat(obj object.Object) (float64, bool) {
	switch arg := obj.(type) {
	case *object.Integer:
		return float64(arg.Value), true
	case *object.Float:
		return arg.Value, true
	default:
		return 0, false
	}
}

// oneFloatFunc creates a math function that takes one float argument and returns a float
func oneFloatFunc(f func(float64) float64) func(context.Context, object.Kwargs, ...object.Object) object.Object {
	return func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		x, ok := toFloat(args[0])
		if !ok {
			return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
		}
		return &object.Float{Value: f(x)}
	}
}

// twoFloatFunc creates a function that takes two floats and applies f
func twoFloatFunc(f func(float64, float64) float64) func(context.Context, object.Kwargs, ...object.Object) object.Object {
	return func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) != 2 {
			return errors.NewArgumentError(len(args), 2)
		}
		x, ok := toFloat(args[0])
		if !ok {
			return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
		}
		y, ok := toFloat(args[1])
		if !ok {
			return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
		}
		return &object.Float{Value: f(x, y)}
	}
}

var MathLibrary = object.NewLibrary(map[string]*object.Builtin{
	"sqrt": {
		Fn: oneFloatFunc(math.Sqrt),
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
	"fabs": {
		Fn: oneFloatFunc(math.Abs),
		HelpText: `fabs(x) - Return the absolute value of x as a float

x can be an integer or float.
Always returns a float.`,
	},
	"floor": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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

	"sin": {
		Fn: oneFloatFunc(math.Sin),
		HelpText: `sin(x) - Return the sine of x (radians)

x can be an integer or float in radians.
Returns a float.`,
	},
	"cos": {
		Fn: oneFloatFunc(math.Cos),
		HelpText: `cos(x) - Return the cosine of x (radians)

x can be an integer or float in radians.
Returns a float.`,
	},
	"tan": {
		Fn: oneFloatFunc(math.Tan),
		HelpText: `tan(x) - Return the tangent of x (radians)

x can be an integer or float in radians.
Returns a float.`,
	},
	"log": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			x, ok := toFloat(args[0])
			if !ok {
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			if x <= 0 {
				return errors.NewError("log: domain error")
			}
			return &object.Float{Value: math.Log(x)}
		},
		HelpText: `log(x) - Return the natural logarithm of x

x must be positive (integer or float).
Returns a float.`,
	},
	"exp": {
		Fn: oneFloatFunc(math.Exp),
		HelpText: `exp(x) - Return e raised to the power x

x can be an integer or float.
Returns a float.`,
	},
	"degrees": {
		Fn: oneFloatFunc(func(x float64) float64 { return x * 180.0 / math.Pi }),
		HelpText: `degrees(x) - Convert radians to degrees

x can be an integer or float in radians.
Returns a float in degrees.`,
	},
	"radians": {
		Fn: oneFloatFunc(func(x float64) float64 { return x * math.Pi / 180.0 }),
		HelpText: `radians(x) - Convert degrees to radians

x can be an integer or float in degrees.
Returns a float in radians.`,
	},
	"fmod": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			x, ok := toFloat(args[0])
			if !ok {
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			y, ok := toFloat(args[1])
			if !ok {
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
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
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			a, ok := args[0].AsInt()
			if !ok {
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}
			b, ok := args[1].AsInt()
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			return &object.Integer{Value: gcd(a, b)}
		},
		HelpText: `gcd(a, b) - Return the greatest common divisor of a and b

a and b must be integers.
Returns an integer.`,
	},
	"factorial": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			n, ok := args[0].AsInt()
			if !ok {
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}
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
		},
		HelpText: `factorial(n) - Return n!

n must be a non-negative integer <= 20.
Returns an integer.`,
	},
	"isnan": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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
		Fn: oneFloatFunc(math.Log10),
		HelpText: `log10(x) - Return the base-10 logarithm of x


x must be positive. Returns a float.`,
	},
	"log2": {
		Fn: oneFloatFunc(math.Log2),
		HelpText: `log2(x) - Return the base-2 logarithm of x

x must be positive. Returns a float.`,
	},
	"hypot": {
		Fn: twoFloatFunc(math.Hypot),
		HelpText: `hypot(x, y) - Return the Euclidean distance sqrt(x*x + y*y)

Returns a float.`,
	},
	"asin": {
		Fn: oneFloatFunc(math.Asin),
		HelpText: `asin(x) - Return the arc sine of x in radians

x must be in the range [-1, 1]. Returns a float.`,
	},
	"acos": {
		Fn: oneFloatFunc(math.Acos),
		HelpText: `acos(x) - Return the arc cosine of x in radians

x must be in the range [-1, 1]. Returns a float.`,
	},
	"atan": {
		Fn: oneFloatFunc(math.Atan),
		HelpText: `atan(x) - Return the arc tangent of x in radians

Returns a float in the range [-pi/2, pi/2].`,
	},
	"atan2": {
		Fn: twoFloatFunc(math.Atan2),
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
