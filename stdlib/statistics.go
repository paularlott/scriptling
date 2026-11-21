package stdlib

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"sort"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var StatisticsLibrary = object.NewLibrary(StatisticsLibraryName, map[string]*object.Builtin{
	"mean": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("mean", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			elems, errObj := iterableElements(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			if len(elems) == 0 {
				return statisticsError("mean requires at least one data point")
			}
			// Python keeps integer data an int when the mean is whole
			// (mean([1, 2, 3]) is 2) and gives a float otherwise.
			if num, den, ok := intMoments(elems, 1); ok {
				return exactRatio(num, den)
			}
			values, err := numbersOf(elems)
			if err != nil {
				return err
			}
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			return object.NewFloat(sum / float64(len(values)))
		},
		HelpText: `mean(data) - Return the arithmetic mean of data

Parameters:
  data - List of numbers

Returns: Float

Example:
  import statistics
  statistics.mean([1, 2, 3, 4, 5])  # 3.0`,
	},
	"fmean": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("fmean", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			values, err := extractNumbers(ctx, args[0])
			if err != nil {
				return err
			}
			if len(values) == 0 {
				return statisticsError("fmean requires at least one data point")
			}
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			return object.NewFloat(sum / float64(len(values)))
		},
		HelpText: `fmean(data) - Return the arithmetic mean of data (faster float version)

Parameters:
  data - List of numbers

Returns: Float

Example:
  import statistics
  statistics.fmean([1.0, 2.0, 3.0])  # 2.0`,
	},
	"geometric_mean": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("geometric_mean", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			values, err := extractNumbers(ctx, args[0])
			if err != nil {
				return err
			}
			if len(values) == 0 {
				return statisticsError("geometric_mean requires at least one data point")
			}
			// Use log to avoid overflow: exp(mean(log(values)))
			logSum := 0.0
			for _, v := range values {
				if v < 0 {
					return statisticsError("geometric_mean requires a data set with positive numbers")
				}
				if v == 0 {
					return object.NewFloat(0) // as Python: a zero makes the mean zero
				}
				logSum += math.Log(v)
			}
			return object.NewFloat(math.Exp(logSum / float64(len(values))))
		},
		HelpText: `geometric_mean(data) - Return the geometric mean of data

Parameters:
  data - List of positive numbers

Returns: Float

Example:
  import statistics
  statistics.geometric_mean([1, 2, 4])  # 2.0`,
	},
	"harmonic_mean": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("harmonic_mean", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			values, err := extractNumbers(ctx, args[0])
			if err != nil {
				return err
			}
			if len(values) == 0 {
				return statisticsError("harmonic_mean requires at least one data point")
			}
			reciprocalSum := 0.0
			for _, v := range values {
				if v < 0 {
					return statisticsError("harmonic mean does not support negative values")
				}
				if v == 0 {
					return object.NewInteger(0) // as Python
				}
				reciprocalSum += 1.0 / v
			}
			return object.NewFloat(float64(len(values)) / reciprocalSum)
		},
		HelpText: `harmonic_mean(data) - Return the harmonic mean of data

Parameters:
  data - List of positive numbers

Returns: Float

Example:
  import statistics
  statistics.harmonic_mean([1, 2, 4])  # ~1.714`,
	},
	"median": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("median", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			elems, errObj := iterableElements(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			values, err := numbersOf(elems)
			if err != nil {
				return err
			}
			if len(values) == 0 {
				return statisticsError("no median for empty data")
			}
			sorted := make([]float64, len(values))
			copy(sorted, values)
			sort.Float64s(sorted)
			n := len(sorted)
			if n%2 == 0 {
				return object.NewFloat((sorted[n/2-1] + sorted[n/2]) / 2)
			}
			// Odd count: Python returns the middle element itself, keeping
			// int inputs ints (statistics.median([1, 3, 2]) is 2, not 2.0).
			ordered := make([]object.Object, len(elems))
			copy(ordered, elems)
			sort.SliceStable(ordered, func(i, j int) bool {
				return numericOf(ordered[i]) < numericOf(ordered[j])
			})
			if mid, ok := ordered[n/2].(*object.Integer); ok {
				return mid
			}
			return object.NewFloat(sorted[n/2])
		},
		HelpText: `median(data) - Return the median (middle value) of data

Parameters:
  data - List of numbers

Returns: Float

Example:
  import statistics
  statistics.median([1, 3, 5])  # 3.0
  statistics.median([1, 3, 5, 7])  # 4.0`,
	},
	"median_low": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return medianLowHigh(ctx, "median_low", kwargs, args, false)
		},
		HelpText: `median_low(data) - Return the low median of data

For an even number of values, the smaller of the two middle values.
Always a member of the data.`,
	},
	"median_high": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return medianLowHigh(ctx, "median_high", kwargs, args, true)
		},
		HelpText: `median_high(data) - Return the high median of data

For an even number of values, the larger of the two middle values.
Always a member of the data.`,
	},
	"mode": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("mode", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			elems, errObj := iterableElements(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			list := &object.List{Elements: elems}
			if len(list.Elements) == 0 {
				return statisticsError("no mode for empty data")
			}
			// Count occurrences; of equally common values the first seen
			// wins, as in Python.
			counts := make(map[string]int)
			var order []string
			elements := make(map[string]object.Object)
			for _, elem := range list.Elements {
				key := object.DictKey(elem)
				if _, seen := counts[key]; !seen {
					order = append(order, key)
					elements[key] = elem
				}
				counts[key]++
			}
			maxCount := 0
			var modeKey string
			for _, key := range order {
				if counts[key] > maxCount {
					maxCount = counts[key]
					modeKey = key
				}
			}
			return elements[modeKey]
		},
		HelpText: `mode(data) - Return the most common value in data

Parameters:
  data - List of values

Returns: Most frequent value (same type as input elements)

Example:
  import statistics
  statistics.mode([1, 1, 2, 3])  # 1
  statistics.mode(["a", "b", "a"])  # "a"`,
	},
	"stdev": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("stdev", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			values, err := extractNumbers(ctx, args[0])
			if err != nil {
				return err
			}
			if len(values) < 2 {
				return statisticsError("stdev requires at least two data points")
			}
			variance := sampleVariance(values)
			return object.NewFloat(math.Sqrt(variance))
		},
		HelpText: `stdev(data) - Return the sample standard deviation of data

Parameters:
  data - List of numbers (at least 2)

Returns: Float

Example:
  import statistics
  statistics.stdev([1, 2, 3, 4, 5])  # ~1.58`,
	},
	"pstdev": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("pstdev", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			values, err := extractNumbers(ctx, args[0])
			if err != nil {
				return err
			}
			if len(values) < 1 {
				return statisticsError("pstdev requires at least one data point")
			}
			variance := populationVariance(values)
			return object.NewFloat(math.Sqrt(variance))
		},
		HelpText: `pstdev(data) - Return the population standard deviation of data

Parameters:
  data - List of numbers

Returns: Float

Example:
  import statistics
  statistics.pstdev([1, 2, 3, 4, 5])  # ~1.41`,
	},
	"variance": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("variance", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			elems, errObj := iterableElements(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			if len(elems) < 2 {
				return statisticsError("variance requires at least two data points")
			}
			// Integer data has an exact variance, an int when whole.
			if num, den, ok := intMoments(elems, 2); ok {
				n := int64(len(elems))
				return exactRatio(num, den.Mul(den, big.NewInt(n-1)))
			}
			values, err := numbersOf(elems)
			if err != nil {
				return err
			}
			return object.NewFloat(sampleVariance(values))
		},
		HelpText: `variance(data) - Return the sample variance of data

Parameters:
  data - List of numbers (at least 2)

Returns: Float

Example:
  import statistics
  statistics.variance([1, 2, 3, 4, 5])  # 2.5`,
	},
	"pvariance": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("pvariance", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			elems, errObj := iterableElements(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			if len(elems) < 1 {
				return statisticsError("pvariance requires at least one data point")
			}
			if num, den, ok := intMoments(elems, 2); ok {
				n := int64(len(elems))
				return exactRatio(num, den.Mul(den, big.NewInt(n)))
			}
			values, err := numbersOf(elems)
			if err != nil {
				return err
			}
			return object.NewFloat(populationVariance(values))
		},
		HelpText: `pvariance(data) - Return the population variance of data

Parameters:
  data - List of numbers

Returns: Float

Example:
  import statistics
  statistics.pvariance([1, 2, 3, 4, 5])  # 2.0`,
	},
}, map[string]object.Object{
	// statistics.StatisticsError: a ValueError subclass, as in Python.
	"StatisticsError": object.RegisterExceptionClass(&object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("StatisticsError", kwargs); err != nil {
				return err
			}
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{Message: message, ExceptionType: "StatisticsError"}
		},
		HelpText: `StatisticsError([message]) - The exception raised for data the statistics functions cannot use`,
		Repr:     "<class 'statistics.StatisticsError'>",
	}, "StatisticsError"),
}, "Statistical functions library")

// statisticsError is a raised statistics.StatisticsError, which
// `except ValueError:` also catches.
func statisticsError(format string, args ...interface{}) object.Object {
	return &object.Exception{Message: fmt.Sprintf(format, args...), ExceptionType: "StatisticsError", Raised: true}
}

// Helper functions

func extractNumbers(ctx context.Context, obj object.Object) ([]float64, object.Object) {
	elems, errObj := iterableElements(ctx, obj)
	if errObj != nil {
		return nil, errObj
	}
	return numbersOf(elems)
}

// numbersOf reads number elements as floats.
func numbersOf(elems []object.Object) ([]float64, object.Object) {
	values := make([]float64, 0, len(elems))
	for _, elem := range elems {
		switch v := elem.(type) {
		case *object.Integer:
			values = append(values, float64(v.IntValue()))
		case *object.Float:
			values = append(values, v.FloatValue())
		case *object.Boolean:
			// bool is an int subclass: mean([True, False]) is 0.5.
			values = append(values, float64(boolInt(v.BoolValue())))
		default:
			return nil, errors.NewTypeError("INTEGER or FLOAT", elem.Type().String())
		}
	}
	return values, nil
}

// intMoments computes, for data that is all integers, the exact mean
// (order 1: sum / n) or the exact sum of squared deviations from the mean
// (order 2: (n*sum(x*x) - sum(x)**2) / n) as a numerator and denominator.
// ok is false when any element is not an int, and the caller falls back to
// floats.
func intMoments(elems []object.Object, order int) (num, den *big.Int, ok bool) {
	sum, sumSq := new(big.Int), new(big.Int)
	var x, sq big.Int
	for _, e := range elems {
		i, isInt := e.(*object.Integer)
		if !isInt {
			return nil, nil, false
		}
		x.SetInt64(i.IntValue())
		sum.Add(sum, &x)
		if order == 2 {
			sumSq.Add(sumSq, sq.Mul(&x, &x))
		}
	}
	n := big.NewInt(int64(len(elems)))
	if order == 1 {
		return sum, n, true
	}
	num = new(big.Int).Mul(n, sumSq)
	num.Sub(num, sq.Mul(sum, sum))
	return num, n, true
}

// exactRatio returns num/den as Python's statistics module does for integer
// data: an int when it divides exactly, otherwise the nearest float.
func exactRatio(num, den *big.Int) object.Object {
	var q, r big.Int
	q.QuoRem(num, den, &r)
	if r.Sign() == 0 && q.IsInt64() {
		return object.NewInteger(q.Int64())
	}
	f, _ := new(big.Rat).SetFrac(num, den).Float64()
	return object.NewFloat(f)
}

func sampleVariance(values []float64) float64 {
	n := len(values)
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(n)

	sumSq := 0.0
	for _, v := range values {
		diff := v - mean
		sumSq += diff * diff
	}
	return sumSq / float64(n-1) // Sample variance uses n-1
}

func populationVariance(values []float64) float64 {
	n := len(values)
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(n)

	sumSq := 0.0
	for _, v := range values {
		diff := v - mean
		sumSq += diff * diff
	}
	return sumSq / float64(n) // Population variance uses n
}

// medianLowHigh implements median_low and median_high: the middle element
// of the sorted data, the lower or higher of the two for an even count.
func medianLowHigh(ctx context.Context, name string, kwargs object.Kwargs, args []object.Object, high bool) object.Object {
	if err := errors.NoKwargs(name, kwargs); err != nil {
		return err
	}
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}
	elems, errObj := iterableElements(ctx, args[0])
	if errObj != nil {
		return errObj
	}
	if len(elems) == 0 {
		return statisticsError("no median for empty data")
	}
	if _, err := numbersOf(elems); err != nil {
		return err
	}
	ordered := make([]object.Object, len(elems))
	copy(ordered, elems)
	sort.SliceStable(ordered, func(i, j int) bool {
		return numericOf(ordered[i]) < numericOf(ordered[j])
	})
	n := len(ordered)
	if n%2 == 1 || high {
		return ordered[n/2]
	}
	return ordered[n/2-1]
}

// numericOf reads a number-typed element's float value for ordering.
func numericOf(v object.Object) float64 {
	switch t := v.(type) {
	case *object.Integer:
		return float64(t.IntValue())
	case *object.Float:
		return t.FloatValue()
	}
	return 0
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
