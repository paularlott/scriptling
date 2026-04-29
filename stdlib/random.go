package stdlib

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func sampleGamma(alpha float64) float64 {
	if alpha < 1 {
		return sampleGamma(alpha+1) * math.Pow(rng.Float64(), 1.0/alpha)
	}
	// Marsaglia and Tsang's method for alpha >= 1
	d := alpha - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)
	for {
		var x float64
		var v float64
		for {
			x = rng.NormFloat64()
			v = 1.0 + c*x
			if v > 0 {
				break
			}
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1.0-0.0331*(x*x)*(x*x) {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1.0-v+math.Log(v)) {
			return d * v
		}
	}
}

// gaussianRandom returns a random number from Gaussian distribution
func gaussianRandom(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 2); err != nil { return err }
	var mu, sigma float64
	switch arg := args[0].(type) {
	case *object.Integer:
		mu = float64(arg.Value)
	case *object.Float:
		mu = arg.Value
	default:
		return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
	}
	switch arg := args[1].(type) {
	case *object.Integer:
		sigma = float64(arg.Value)
	case *object.Float:
		sigma = arg.Value
	default:
		return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
	}
	// Box-Muller transform
	val := rng.NormFloat64()*sigma + mu
	return &object.Float{Value: val}
}

var RandomLibrary = object.NewLibrary(RandomLibraryName, map[string]*object.Builtin{
	"seed": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MaxArgs(args, 1); err != nil { return err }
			var seedVal int64
			if len(args) == 0 {
				seedVal = time.Now().UnixNano()
			} else {
				switch arg := args[0].(type) {
				case *object.Integer:
					seedVal = arg.Value
				case *object.Float:
					seedVal = int64(arg.Value)
				default:
					return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
				}
			}
			rng = rand.New(rand.NewSource(seedVal))
			return &object.Null{}
		},
		HelpText: `seed([a]) - Initialize the random number generator

If a is omitted, current time is used. Otherwise, a is used as the seed.`,
	},
	"randint": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil { return err }
			var min, max int64
			switch arg := args[0].(type) {
			case *object.Integer:
				min = arg.Value
			default:
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}
			switch arg := args[1].(type) {
			case *object.Integer:
				max = arg.Value
			default:
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			if min > max {
				return errors.NewError("randint() min must be <= max")
			}
			val := min + rng.Int63n(max-min+1)
			return &object.Integer{Value: val}
		},
		HelpText: `randint(min, max) - Return random integer

Returns a random integer N such that min <= N <= max.`,
	},
	"random": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 0); err != nil { return err }
			return &object.Float{Value: rng.Float64()}
		},
		HelpText: `random() - Return random float

Returns a random float in the range [0.0, 1.0).`,
	},
	"choice": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil { return err }
			if str, ok := args[0].(*object.String); ok {
				if len(str.Value) == 0 {
					return errors.NewError("choice() string cannot be empty")
				}
				idx := rng.Intn(len(str.Value))
				return &object.String{Value: string(str.Value[idx])}
			}
			if list, ok := args[0].(*object.List); ok {
				if len(list.Elements) == 0 {
					return errors.NewError("choice() list cannot be empty")
				}
				idx := rng.Intn(len(list.Elements))
				return list.Elements[idx]
			}
			return errors.NewTypeError("LIST or STRING", args[0].Type().String())
		},
		HelpText: `choice(seq) - Return random element from sequence

Returns a randomly selected element from the given list or string.`,
	},
	"shuffle": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil { return err }
			if list, ok := args[0].(*object.List); ok {
				n := len(list.Elements)
				for i := n - 1; i > 0; i-- {
					j := rng.Intn(i + 1)
					list.Elements[i], list.Elements[j] = list.Elements[j], list.Elements[i]
				}
				return &object.Null{}
			}
			return errors.NewTypeError("LIST", args[0].Type().String())
		},
		HelpText: `shuffle(list) - Shuffle list in place

Randomly shuffles the elements of the list in place using the Fisher-Yates algorithm. Returns None.`,
	},
	"uniform": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil { return err }
			var a, b float64
			switch arg := args[0].(type) {
			case *object.Integer:
				a = float64(arg.Value)
			case *object.Float:
				a = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			switch arg := args[1].(type) {
			case *object.Integer:
				b = float64(arg.Value)
			case *object.Float:
				b = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}
			// Generate random float in range [a, b]
			val := a + rng.Float64()*(b-a)
			return &object.Float{Value: val}
		},
		HelpText: `uniform(a, b) - Return random float N such that a <= N <= b

Returns a random floating-point number N such that a <= N <= b.`,
	},
	"sample": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil { return err }
			list, err := args[0].AsList()
			if err != nil {
				return err
			}
			k, ok := args[1].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			n := len(list)
			if k.Value < 0 || k.Value > int64(n) {
				return errors.NewError("sample larger than population or is negative")
			}
			// Fisher-Yates shuffle for sampling
			// Create a copy of indices
			indices := make([]int, n)
			for i := range indices {
				indices[i] = i
			}
			// Shuffle first k elements
			for i := 0; i < int(k.Value); i++ {
				j := i + rng.Intn(n-i)
				indices[i], indices[j] = indices[j], indices[i]
			}
			// Build result
			result := make([]object.Object, k.Value)
			for i := 0; i < int(k.Value); i++ {
				result[i] = list[indices[i]]
			}
			return &object.List{Elements: result}
		},
		HelpText: `sample(population, k) - Return k unique random elements from population

Returns a k length list of unique elements chosen from the population sequence.
Used for random sampling without replacement.`,
	},
	"randrange": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 3 {
				return errors.NewError("randrange() takes 1 to 3 arguments (%d given)", len(args))
			}
			var start, stop, step int64
			step = 1
			switch len(args) {
			case 1:
				// randrange(stop) - from 0 to stop-1
				start = 0
				if i, ok := args[0].(*object.Integer); ok {
					stop = i.Value
				} else {
					return errors.NewTypeError("INTEGER", args[0].Type().String())
				}
			case 2:
				// randrange(start, stop) - from start to stop-1
				if i, ok := args[0].(*object.Integer); ok {
					start = i.Value
				} else {
					return errors.NewTypeError("INTEGER", args[0].Type().String())
				}
				if i, ok := args[1].(*object.Integer); ok {
					stop = i.Value
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			case 3:
				// randrange(start, stop, step)
				if i, ok := args[0].(*object.Integer); ok {
					start = i.Value
				} else {
					return errors.NewTypeError("INTEGER", args[0].Type().String())
				}
				if i, ok := args[1].(*object.Integer); ok {
					stop = i.Value
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
				if i, ok := args[2].(*object.Integer); ok {
					step = i.Value
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
				}
			}
			if step == 0 {
				return errors.NewError("randrange() step argument must not be zero")
			}
			var n int64
			if step > 0 {
				n = (stop - start + step - 1) / step
			} else {
				n = (start - stop - step - 1) / (-step)
			}
			if n <= 0 {
				return errors.NewError("randrange() empty range")
			}
			return &object.Integer{Value: start + step*rng.Int63n(n)}
		},
		HelpText: `randrange(stop) or randrange(start, stop[, step]) - Return random integer from range

Returns a randomly selected element from range(start, stop, step).
Like randint, but doesn't include the endpoint.`,
	},
	"gauss": {
		Fn: gaussianRandom,
		HelpText: `gauss(mu, sigma) - Return random number from Gaussian distribution

mu is the mean, sigma is the standard deviation.`,
	},
	"normalvariate": {
		Fn: gaussianRandom,
		HelpText: `normalvariate(mu, sigma) - Return random number from normal distribution

mu is the mean, sigma is the standard deviation.
Same as gauss() but provided for compatibility.`,
	},
	"expovariate": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil { return err }
			var lambd float64
			switch arg := args[0].(type) {
			case *object.Integer:
				lambd = float64(arg.Value)
			case *object.Float:
				lambd = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			if lambd == 0 {
				return errors.NewError("expovariate() lambda must not be zero")
			}
			// Exponential distribution: -ln(U) / lambda
			val := -math.Log(1.0-rng.Float64()) / lambd
			return &object.Float{Value: val}
		},
		HelpText: `expovariate(lambd) - Return random number from exponential distribution

lambd is 1.0 divided by the desired mean.`,
	},
	"choices": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}
			population, err := args[0].AsList()
			if err != nil {
				return err
			}
			n := len(population)
			if n == 0 {
				return errors.NewError("choices: population cannot be empty")
			}

			var weights []float64
			if wObj := kwargs.Get("weights"); wObj != nil {
				wList, ok := wObj.(*object.List)
				if !ok {
					return errors.NewTypeError("LIST", wObj.Type().String())
				}
				if len(wList.Elements) != n {
					return errors.NewError("choices: weights length (%d) must match population length (%d)", len(wList.Elements), n)
				}
				weights = make([]float64, n)
				for i, w := range wList.Elements {
					f, err := w.AsFloat()
					if err != nil {
						return errors.NewTypeError("INTEGER or FLOAT", w.Type().String())
					}
					weights[i] = f
				}
			} else {
				weights = make([]float64, n)
				for i := range weights {
					weights[i] = 1.0
				}
			}

			cumWeights := make([]float64, n)
			cumWeights[0] = weights[0]
			for i := 1; i < n; i++ {
				cumWeights[i] = cumWeights[i-1] + weights[i]
			}
			total := cumWeights[n-1]
			if total <= 0 {
				return errors.NewError("choices: total of weights must be positive")
			}

			k := 1
			if kObj := kwargs.Get("k"); kObj != nil {
				kVal, err := kObj.AsInt()
				if err != nil {
					return err
				}
				if kVal < 0 {
					return errors.NewError("choices: k must be non-negative")
				}
				k = int(kVal)
			}

			result := make([]object.Object, k)
			for i := 0; i < k; i++ {
				r := rng.Float64() * total
				idx := 0
				for idx < n-1 && cumWeights[idx] < r {
					idx++
				}
				result[i] = population[idx]
			}
			return &object.List{Elements: result}
		},
		HelpText: `choices(population, weights=None, k=1) - Weighted random sampling with replacement

Select k items from population with the given weights.
Returns a list of k selected items.`,
	},
	"betavariate": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			alpha, err := args[0].AsFloat()
			if err != nil {
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			beta, err := args[1].AsFloat()
			if err != nil {
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}
			if alpha <= 0 || beta <= 0 {
				return errors.NewError("betavariate: alpha and beta must be positive")
			}
			// Use the relation: if X ~ Gamma(alpha,1) and Y ~ Gamma(beta,1), then X/(X+Y) ~ Beta(alpha,beta)
			x := sampleGamma(alpha)
			y := sampleGamma(beta)
			if x+y == 0 {
				return &object.Float{Value: 0}
			}
			return &object.Float{Value: x / (x + y)}
		},
		HelpText: `betavariate(alpha, beta) - Return random number from beta distribution

alpha and beta must be positive. Returns a float in [0, 1].`,
	},
	"gammavariate": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			alpha, err := args[0].AsFloat()
			if err != nil {
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			beta, err := args[1].AsFloat()
			if err != nil {
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}
			if alpha <= 0 || beta <= 0 {
				return errors.NewError("gammavariate: alpha and beta must be positive")
			}
			return &object.Float{Value: sampleGamma(alpha) * beta}
		},
		HelpText: `gammavariate(alpha, beta) - Return random number from gamma distribution

alpha (shape) and beta (scale) must be positive.`,
	},
	"triangular": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 2, 3); err != nil {
				return err
			}
			low, err := args[0].AsFloat()
			if err != nil {
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			high, err := args[1].AsFloat()
			if err != nil {
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}
			mode := (low + high) / 2.0
			if len(args) == 3 {
				mode, err = args[2].AsFloat()
				if err != nil {
					return errors.NewTypeError("INTEGER or FLOAT", args[2].Type().String())
				}
			}
			if low == high {
				return &object.Float{Value: low}
			}
			u := rng.Float64()
			c := (mode - low) / (high - low)
			var val float64
			if u <= c {
				val = low + math.Sqrt(u*(high-low)*(mode-low))
			} else {
				val = high - math.Sqrt((1-u)*(high-low)*(high-mode))
			}
			return &object.Float{Value: val}
		},
		HelpText: `triangular(low, high[, mode]) - Return random number from triangular distribution

low is the minimum, high is the maximum, mode is the peak (defaults to midpoint).`,
	},
	"paretovariate": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			alpha, err := args[0].AsFloat()
			if err != nil {
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			if alpha <= 0 {
				return errors.NewError("paretovariate: alpha must be positive")
			}
			// Pareto: 1 / U^(1/alpha)
			u := rng.Float64()
			if u == 0 {
				u = math.SmallestNonzeroFloat64
			}
			return &object.Float{Value: 1.0 / math.Pow(u, 1.0/alpha)}
		},
		HelpText: `paretovariate(alpha) - Return random number from Pareto distribution

alpha is the shape parameter, must be positive.`,
	},
	"weibullvariate": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			alpha, err := args[0].AsFloat()
			if err != nil {
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			beta, err := args[1].AsFloat()
			if err != nil {
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}
			if alpha <= 0 || beta <= 0 {
				return errors.NewError("weibullvariate: alpha and beta must be positive")
			}
			// Weibull: alpha * (-ln(U))^(1/beta)
			u := rng.Float64()
			if u == 0 {
				u = math.SmallestNonzeroFloat64
			}
			return &object.Float{Value: alpha * math.Pow(-math.Log(u), 1.0/beta)}
		},
		HelpText: `weibullvariate(alpha, beta) - Return random number from Weibull distribution

alpha is the scale parameter, beta is the shape parameter. Both must be positive.`,
	},
}, nil, "Random number generation library")
