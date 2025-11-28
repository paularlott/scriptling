package stdlib

import (
	"context"
	"math"
	"sort"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var StatisticsLibrary = object.NewLibrary(map[string]*object.Builtin{
	"mean": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			values, err := extractNumbers(args[0])
			if err != nil {
				return err
			}
			if len(values) == 0 {
				return errors.NewError("mean requires at least one data point")
			}
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			return &object.Float{Value: sum / float64(len(values))}
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			values, err := extractNumbers(args[0])
			if err != nil {
				return err
			}
			if len(values) == 0 {
				return errors.NewError("fmean requires at least one data point")
			}
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			return &object.Float{Value: sum / float64(len(values))}
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			values, err := extractNumbers(args[0])
			if err != nil {
				return err
			}
			if len(values) == 0 {
				return errors.NewError("geometric_mean requires at least one data point")
			}
			// Use log to avoid overflow: exp(mean(log(values)))
			logSum := 0.0
			for _, v := range values {
				if v <= 0 {
					return errors.NewError("geometric_mean requires positive numbers")
				}
				logSum += math.Log(v)
			}
			return &object.Float{Value: math.Exp(logSum / float64(len(values)))}
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			values, err := extractNumbers(args[0])
			if err != nil {
				return err
			}
			if len(values) == 0 {
				return errors.NewError("harmonic_mean requires at least one data point")
			}
			reciprocalSum := 0.0
			for _, v := range values {
				if v <= 0 {
					return errors.NewError("harmonic_mean requires positive numbers")
				}
				reciprocalSum += 1.0 / v
			}
			return &object.Float{Value: float64(len(values)) / reciprocalSum}
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			values, err := extractNumbers(args[0])
			if err != nil {
				return err
			}
			if len(values) == 0 {
				return errors.NewError("median requires at least one data point")
			}
			sorted := make([]float64, len(values))
			copy(sorted, values)
			sort.Float64s(sorted)
			n := len(sorted)
			if n%2 == 0 {
				return &object.Float{Value: (sorted[n/2-1] + sorted[n/2]) / 2}
			}
			return &object.Float{Value: sorted[n/2]}
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
	"mode": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			list, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST", args[0].Type().String())
			}
			if len(list.Elements) == 0 {
				return errors.NewError("mode requires at least one data point")
			}
			// Count occurrences
			counts := make(map[string]int)
			elements := make(map[string]object.Object)
			for _, elem := range list.Elements {
				key := elem.Inspect()
				counts[key]++
				elements[key] = elem
			}
			// Find max count
			maxCount := 0
			var modeKey string
			for key, count := range counts {
				if count > maxCount {
					maxCount = count
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			values, err := extractNumbers(args[0])
			if err != nil {
				return err
			}
			if len(values) < 2 {
				return errors.NewError("stdev requires at least two data points")
			}
			variance := sampleVariance(values)
			return &object.Float{Value: math.Sqrt(variance)}
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			values, err := extractNumbers(args[0])
			if err != nil {
				return err
			}
			if len(values) < 1 {
				return errors.NewError("pstdev requires at least one data point")
			}
			variance := populationVariance(values)
			return &object.Float{Value: math.Sqrt(variance)}
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			values, err := extractNumbers(args[0])
			if err != nil {
				return err
			}
			if len(values) < 2 {
				return errors.NewError("variance requires at least two data points")
			}
			return &object.Float{Value: sampleVariance(values)}
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			values, err := extractNumbers(args[0])
			if err != nil {
				return err
			}
			if len(values) < 1 {
				return errors.NewError("pvariance requires at least one data point")
			}
			return &object.Float{Value: populationVariance(values)}
		},
		HelpText: `pvariance(data) - Return the population variance of data

Parameters:
  data - List of numbers

Returns: Float

Example:
  import statistics
  statistics.pvariance([1, 2, 3, 4, 5])  # 2.0`,
	},
}, nil, "Statistical functions library")

// Helper functions

func extractNumbers(obj object.Object) ([]float64, object.Object) {
	list, ok := obj.(*object.List)
	if !ok {
		return nil, errors.NewTypeError("LIST", obj.Type().String())
	}
	values := make([]float64, 0, len(list.Elements))
	for _, elem := range list.Elements {
		switch v := elem.(type) {
		case *object.Integer:
			values = append(values, float64(v.Value))
		case *object.Float:
			values = append(values, v.Value)
		default:
			return nil, errors.NewTypeError("INTEGER or FLOAT", elem.Type().String())
		}
	}
	return values, nil
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
