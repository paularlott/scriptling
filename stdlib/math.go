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

func toFloatMatrix(obj object.Object, fnName, paramName string) ([][]float64, object.Object) {
	list, ok := obj.(*object.List)
	if !ok {
		return nil, errors.NewTypeError("LIST", obj.Type().String())
	}
	rows := make([][]float64, len(list.Elements))
	for i, rowObj := range list.Elements {
		row, ok := rowObj.(*object.List)
		if !ok {
			return nil, errors.NewError("%s: %s must be a list of lists", fnName, paramName)
		}
		rows[i] = make([]float64, len(row.Elements))
		for j, el := range row.Elements {
			f, err := el.AsFloat()
			if err != nil {
				return nil, errors.NewTypeError("INTEGER or FLOAT", el.Type().String())
			}
			rows[i][j] = f
		}
	}
	return rows, nil
}

func floatMatrixToObject(m [][]float64) object.Object {
	rows := make([]object.Object, len(m))
	for i, r := range m {
		elems := make([]object.Object, len(r))
		for j, v := range r {
			elems[j] = &object.Float{Value: v}
		}
		rows[i] = &object.List{Elements: elems}
	}
	return &object.List{Elements: rows}
}

// oneFloatFunc creates a math function that takes one float argument and returns a float
func oneFloatFunc(f func(float64) float64) func(context.Context, object.Kwargs, ...object.Object) object.Object {
	return func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		x, err := args[0].AsFloat()
		if err != nil {
			return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
		}
		return &object.Float{Value: f(x)}
	}
}

// twoFloatFunc creates a function that takes two floats and applies f
func twoFloatFunc(f func(float64, float64) float64) func(context.Context, object.Kwargs, ...object.Object) object.Object {
	return func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if err := errors.ExactArgs(args, 2); err != nil {
			return err
		}
		x, err := args[0].AsFloat()
		if err != nil {
			return err
		}
		y, err := args[1].AsFloat()
		if err != nil {
			return err
		}
		return &object.Float{Value: f(x, y)}
	}
}

// oneIntOrFloatFunc creates a math function that takes one int-or-float and returns an integer or boolean.
// intFn is called for integers, floatFn for floats; both receive the value and return an object.Object.
func oneIntOrFloatFunc(intFn func(*object.Integer) object.Object, floatFn func(*object.Float) object.Object) func(context.Context, object.Kwargs, ...object.Object) object.Object {
	return func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		switch arg := args[0].(type) {
		case *object.Integer:
			return intFn(arg)
		case *object.Float:
			return floatFn(arg)
		default:
			return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
		}
	}
}

var MathLibrary = object.NewLibrary(MathLibraryName, map[string]*object.Builtin{
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
		Fn: oneIntOrFloatFunc(
			func(i *object.Integer) object.Object { return i },
			func(f *object.Float) object.Object { return &object.Integer{Value: int64(math.Floor(f.Value))} },
		),
		HelpText: `floor(x) - Return the floor of x

x can be an integer or float.
Returns the largest integer less than or equal to x.`,
	},
	"ceil": {
		Fn: oneIntOrFloatFunc(
			func(i *object.Integer) object.Object { return i },
			func(f *object.Float) object.Object { return &object.Integer{Value: int64(math.Ceil(f.Value))} },
		),
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
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			x, err := args[0].AsFloat()
			if err != nil {
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
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			x, err := args[0].AsFloat()
			if err != nil {
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			y, err := args[1].AsFloat()
			if err != nil {
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
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			a, err := args[0].AsInt()
			if err != nil {
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}
			b, err := args[1].AsInt()
			if err != nil {
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
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			n, err := args[0].AsInt()
			if err != nil {
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
		Fn: oneIntOrFloatFunc(
			func(i *object.Integer) object.Object { return object.NewBoolean(false) },
			func(f *object.Float) object.Object { return object.NewBoolean(math.IsNaN(f.Value)) },
		),
		HelpText: `isnan(x) - Check if x is NaN (Not a Number)

Returns True if x is NaN, False otherwise.`,
	},
	"isinf": {
		Fn: oneIntOrFloatFunc(
			func(i *object.Integer) object.Object { return object.NewBoolean(false) },
			func(f *object.Float) object.Object { return object.NewBoolean(math.IsInf(f.Value, 0)) },
		),
		HelpText: `isinf(x) - Check if x is infinite

Returns True if x is positive or negative infinity.`,
	},
	"isfinite": {
		Fn: oneIntOrFloatFunc(
			func(i *object.Integer) object.Object { return object.NewBoolean(true) },
			func(f *object.Float) object.Object {
				return object.NewBoolean(!math.IsNaN(f.Value) && !math.IsInf(f.Value, 0))
			},
		),
		HelpText: `isfinite(x) - Check if x is finite

Returns True if x is neither NaN nor infinite.`,
	},
	"copysign": {
		Fn: twoFloatFunc(math.Copysign),
		HelpText: `copysign(x, y) - Return x with the sign of y

Returns a float with magnitude of x and sign of y.`,
	},
	"trunc": {
		Fn: oneIntOrFloatFunc(
			func(i *object.Integer) object.Object { return i },
			func(f *object.Float) object.Object { return &object.Integer{Value: int64(math.Trunc(f.Value))} },
		),
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
	"tanh": {
		Fn: oneFloatFunc(math.Tanh),
		HelpText: `tanh(x) - Return the hyperbolic tangent of x

Returns a float in the range [-1, 1].`,
	},
	"softmax": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			list, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST", args[0].Type().String())
			}
			n := len(list.Elements)
			if n == 0 {
				return errors.NewError("softmax: input list cannot be empty")
			}
			vals := make([]float64, n)
			maxVal := math.Inf(-1)
			for i, el := range list.Elements {
				f, err := el.AsFloat()
				if err != nil {
					return errors.NewTypeError("INTEGER or FLOAT", el.Type().String())
				}
				vals[i] = f
				if f > maxVal {
					maxVal = f
				}
			}
			var sum float64
			exps := make([]float64, n)
			for i, v := range vals {
				exps[i] = math.Exp(v - maxVal)
				sum += exps[i]
			}
			result := make([]object.Object, n)
			for i, e := range exps {
				result[i] = &object.Float{Value: e / sum}
			}
			return &object.List{Elements: result}
		},
		HelpText: `softmax(x) - Return numerically stable softmax of a vector

Returns a probability distribution (list of floats summing to 1.0).`,
	},
	"dot": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			aList, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST", args[0].Type().String())
			}
			bList, ok := args[1].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST", args[1].Type().String())
			}
			if len(aList.Elements) != len(bList.Elements) {
				return errors.NewError("dot: vectors must have the same length")
			}
			n := len(aList.Elements)
			if n == 0 {
				return &object.Float{Value: 0}
			}
			var sum float64
			for i := 0; i < n; i++ {
				a, err := aList.Elements[i].AsFloat()
				if err != nil {
					return errors.NewTypeError("INTEGER or FLOAT", aList.Elements[i].Type().String())
				}
				b, err := bList.Elements[i].AsFloat()
				if err != nil {
					return errors.NewTypeError("INTEGER or FLOAT", bList.Elements[i].Type().String())
				}
				sum += a * b
			}
			return &object.Float{Value: sum}
		},
		HelpText: `dot(a, b) - Return the dot product of two vectors

a and b must be lists of numbers with the same length.`,
	},
	"matmul": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			aRows, errObj := toFloatMatrix(args[0], "matmul", "a")
			if errObj != nil {
				return errObj
			}
			bRows, errObj := toFloatMatrix(args[1], "matmul", "b")
			if errObj != nil {
				return errObj
			}
			if len(aRows) == 0 || len(bRows) == 0 {
				return errors.NewError("matmul: matrices cannot be empty")
			}
			k := len(aRows[0])
			if len(bRows) != k {
				return errors.NewError("matmul: inner dimensions mismatch (%d vs %d)", k, len(bRows))
			}
			n := len(bRows[0])
			result := make([][]float64, len(aRows))
			for i := range result {
				result[i] = make([]float64, n)
			}
			for i, aRow := range aRows {
				for j := 0; j < n; j++ {
					var sum float64
					for l := 0; l < k; l++ {
						sum += aRow[l] * bRows[l][j]
					}
					result[i][j] = sum
				}
			}
			return floatMatrixToObject(result)
		},
		HelpText: `matmul(a, b) - Matrix-matrix multiply

a is (M x K), b is (K x N). Returns (M x N) matrix as list of lists.`,
	},
	"transpose": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			rows, errObj := toFloatMatrix(args[0], "transpose", "matrix")
			if errObj != nil {
				return errObj
			}
			if len(rows) == 0 {
				return &object.List{Elements: []object.Object{}}
			}
			m := len(rows)
			n := len(rows[0])
			result := make([][]float64, n)
			for j := range result {
				result[j] = make([]float64, m)
				for i := 0; i < m; i++ {
					result[j][i] = rows[i][j]
				}
			}
			return floatMatrixToObject(result)
		},
		HelpText: `transpose(m) - Transpose a 2D matrix

Rows become columns. Returns a new matrix.`,
	},
	"mat_add": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			aRows, errObj := toFloatMatrix(args[0], "mat_add", "a")
			if errObj != nil {
				return errObj
			}
			bRows, errObj := toFloatMatrix(args[1], "mat_add", "b")
			if errObj != nil {
				return errObj
			}
			if len(aRows) != len(bRows) {
				return errors.NewError("mat_add: matrices must have the same shape")
			}
			if len(aRows) == 0 {
				return &object.List{Elements: []object.Object{}}
			}
			if len(aRows[0]) != len(bRows[0]) {
				return errors.NewError("mat_add: matrices must have the same shape")
			}
			result := make([][]float64, len(aRows))
			for i := range aRows {
				result[i] = make([]float64, len(aRows[0]))
				for j := range aRows[0] {
					result[i][j] = aRows[i][j] + bRows[i][j]
				}
			}
			return floatMatrixToObject(result)
		},
		HelpText: `mat_add(a, b) - Element-wise addition of two matrices

a and b must have the same shape. Returns a new matrix.`,
	},
	"erf": {
		Fn: oneFloatFunc(math.Erf),
		HelpText: `erf(x) - Return the error function of x

Returns a float in the range [-1, 1].`,
	},
	"erfc": {
		Fn: oneFloatFunc(math.Erfc),
		HelpText: `erfc(x) - Return the complementary error function of x

Returns a float in the range [0, 2].`,
	},
	"gamma": {
		Fn: oneFloatFunc(math.Gamma),
		HelpText: `gamma(x) - Return the gamma function of x

Returns a float.`,
	},
	"lgamma": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			x, err := args[0].AsFloat()
			if err != nil {
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			val, sign := math.Lgamma(x)
			return &object.List{Elements: []object.Object{
				&object.Float{Value: val},
				object.NewInteger(int64(sign)),
			}}
		},
		HelpText: `lgamma(x) - Return the natural log of the absolute value of the gamma function

Returns a list [log_abs_gamma, sign].`,
	},
	"nextafter": {
		Fn: twoFloatFunc(math.Nextafter),
		HelpText: `nextafter(x, y) - Return the next floating-point value after x towards y

Returns a float.`,
	},
	"cbrt": {
		Fn: oneFloatFunc(math.Cbrt),
		HelpText: `cbrt(x) - Return the cube root of x

Returns a float.`,
	},
	"remainder": {
		Fn: twoFloatFunc(math.Remainder),
		HelpText: `remainder(x, y) - Return the IEEE 754-style remainder of x/y

Returns a float.`,
	},
	"log1p": {
		Fn: oneFloatFunc(math.Log1p),
		HelpText: `log1p(x) - Return log(1+x) accurately for small x

Returns a float.`,
	},
	"expm1": {
		Fn: oneFloatFunc(math.Expm1),
		HelpText: `expm1(x) - Return exp(x)-1 accurately for small x

Returns a float.`,
	},
	"comb": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			n, err := args[0].AsInt()
			if err != nil {
				return err
			}
			k, err := args[1].AsInt()
			if err != nil {
				return err
			}
			if n < 0 || k < 0 {
				return errors.NewError("comb: arguments must be non-negative")
			}
			if k > n {
				return object.NewInteger(0)
			}
			if k == 0 || k == n {
				return object.NewInteger(1)
			}
			if k > n-k {
				k = n - k
			}
			var result int64 = 1
			for i := int64(0); i < k; i++ {
				if result > (math.MaxInt64-1)/(n-i)/(i+1) {
					return errors.NewError("comb: result too large")
				}
				result = result * (n - i) / (i + 1)
			}
			return object.NewInteger(result)
		},
		HelpText: `comb(n, k) - Return the number of ways to choose k items from n

Also known as the binomial coefficient. n and k must be non-negative integers.`,
	},
	"perm": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			n, err := args[0].AsInt()
			if err != nil {
				return err
			}
			if n < 0 {
				return errors.NewError("perm: argument must be non-negative")
			}
			k := n
			if len(args) == 2 {
				k, err = args[1].AsInt()
				if err != nil {
					return err
				}
			}
			if k < 0 || k > n {
				return object.NewInteger(0)
			}
			var result int64 = 1
			for i := int64(0); i < k; i++ {
				if result > math.MaxInt64/(n-i) {
					return errors.NewError("perm: result too large")
				}
				result *= n - i
			}
			return object.NewInteger(result)
		},
		HelpText: `perm(n[, k]) - Return the number of ways to choose k items from n with order

n and k must be non-negative integers. If k is omitted, returns n!.`,
	},
	"prod": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			list, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST", args[0].Type().String())
			}
			if len(list.Elements) == 0 {
				return object.NewInteger(1)
			}
			if kwargs.Has("start") && kwargs.Get("start") != nil {
				result, err := kwargs.Get("start").AsFloat()
				if err != nil {
					return err
				}
				for _, el := range list.Elements {
					v, err := el.AsFloat()
					if err != nil {
						return errors.NewTypeError("INTEGER or FLOAT", el.Type().String())
					}
					result *= v
				}
				return &object.Float{Value: result}
			}
			var intResult int64 = 1
			allInt := true
			overflow := false
			for _, el := range list.Elements {
				vInt, ok := el.(*object.Integer)
				if !ok {
					allInt = false
					break
				}
				if overflow || intResult == 0 {
					continue
				}
				if vInt.Value != 0 && intResult > math.MaxInt64/vInt.Value {
					overflow = true
					continue
				}
				intResult *= vInt.Value
			}
			if allInt && !overflow {
				return object.NewInteger(intResult)
			}
			var result float64 = 1
			for _, el := range list.Elements {
				v, err := el.AsFloat()
				if err != nil {
					return errors.NewTypeError("INTEGER or FLOAT", el.Type().String())
				}
				result *= v
			}
			return &object.Float{Value: result}
		},
		HelpText: `prod(iterable, start=1) - Return the product of all elements

With start keyword, begins multiplication from that value.
Returns an integer for all-integer inputs, float otherwise.`,
	},
	"dist": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			pList, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST", args[0].Type().String())
			}
			qList, ok := args[1].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST", args[1].Type().String())
			}
			if len(pList.Elements) != len(qList.Elements) {
				return errors.NewError("dist: points must have the same dimension")
			}
			var sum float64
			for i := 0; i < len(pList.Elements); i++ {
				p, err := pList.Elements[i].AsFloat()
				if err != nil {
					return errors.NewTypeError("INTEGER or FLOAT", pList.Elements[i].Type().String())
				}
				q, err := qList.Elements[i].AsFloat()
				if err != nil {
					return errors.NewTypeError("INTEGER or FLOAT", qList.Elements[i].Type().String())
				}
				d := p - q
				sum += d * d
			}
			return &object.Float{Value: math.Sqrt(sum)}
		},
		HelpText: `dist(p, q) - Return the Euclidean distance between two points

p and q must be lists of numbers with the same length.`,
	},
}, map[string]object.Object{
	"pi":  &object.Float{Value: math.Pi},
	"e":   &object.Float{Value: math.E},
	"inf": &object.Float{Value: math.Inf(1)},
	"nan": &object.Float{Value: math.NaN()},
	"tau": &object.Float{Value: 2 * math.Pi},
}, "Mathematical functions library")
