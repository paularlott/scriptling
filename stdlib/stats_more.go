package stdlib

import (
	"context"
	"math"
	"math/big"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// statistics.multimode, quantiles, covariance, correlation and
// linear_regression; math.hypot with any number of coordinates, math.ulp and
// math.sumprod; and string.capwords.

// exactSum is the correctly rounded sum of xs (math.fsum).
func exactSum(xs []float64) float64 {
	acc := new(big.Float).SetPrec(2300)
	for _, x := range xs {
		if math.IsInf(x, 0) || math.IsNaN(x) {
			s := 0.0
			for _, y := range xs {
				s += y
			}
			return s
		}
		acc.Add(acc, big.NewFloat(x))
	}
	out, _ := acc.Float64()
	return out
}

// pairedData reads the two equal-length data sets of covariance,
// correlation and linear_regression.
func pairedData(ctx context.Context, name string, args []object.Object) ([]float64, []float64, object.Object) {
	if len(args) != 2 {
		return nil, nil, errors.NewTypeErrorTagged("%s() takes exactly 2 positional arguments (%d given)", name, len(args))
	}
	x, errObj := extractNumbers(ctx, args[0])
	if errObj != nil {
		return nil, nil, errObj
	}
	y, errObj := extractNumbers(ctx, args[1])
	if errObj != nil {
		return nil, nil, errObj
	}
	if len(x) != len(y) {
		return nil, nil, statisticsError("%s requires that both inputs have same number of data points", name)
	}
	if len(x) < 2 {
		return nil, nil, statisticsError("%s requires at least two data points", name)
	}
	return x, y, nil
}

// deviationSums is (sxy, sxx, syy, xbar, ybar) over paired data.
func deviationSums(x, y []float64) (sxy, sxx, syy, xbar, ybar float64) {
	n := float64(len(x))
	xbar, ybar = exactSum(x)/n, exactSum(y)/n
	pxy := make([]float64, len(x))
	pxx := make([]float64, len(x))
	pyy := make([]float64, len(x))
	for i := range x {
		dx, dy := x[i]-xbar, y[i]-ybar
		pxy[i], pxx[i], pyy[i] = dx*dy, dx*dx, dy*dy
	}
	return exactSum(pxy), exactSum(pxx), exactSum(pyy), xbar, ybar
}

// linearRegressionClass is the class of linear_regression's result.
var linearRegressionClass = &object.Class{Name: "LinearRegression", Methods: map[string]object.Object{
	"__repr__": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			inst := args[0].(*object.Instance)
			return object.NewString("LinearRegression(slope=" + inst.Field("slope").Inspect() + ", intercept=" + inst.Field("intercept").Inspect() + ")")
		},
		HelpText: "__repr__() - LinearRegression(slope=..., intercept=...)",
	},
	"__getitem__": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			inst := args[0].(*object.Instance)
			i, err := args[1].AsInt()
			if err != nil {
				return err
			}
			switch i {
			case 0, -2:
				return inst.Field("slope")
			case 1, -1:
				return inst.Field("intercept")
			}
			return errors.NewIndexError("tuple index out of range")
		},
		HelpText: "__getitem__(i) - slope (0) or intercept (1)",
	},
	"__len__": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return object.NewInteger(2)
		},
		HelpText: "__len__() - 2",
	},
}}

func init() {
	s := StatisticsLibrary.Functions()
	s["multimode"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("multimode", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			elems, errObj := iterableElements(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			counts := map[string]int{}
			var order []object.Object
			var keys []string
			for _, e := range elems {
				k, errObj := object.HashKey(ctx, e)
				if errObj != nil {
					return errObj
				}
				if _, seen := counts[k]; !seen {
					order = append(order, e)
					keys = append(keys, k)
				}
				counts[k]++
			}
			best := 0
			for _, c := range counts {
				best = max(best, c)
			}
			out := []object.Object{}
			for i, k := range keys {
				if counts[k] == best {
					out = append(out, order[i])
				}
			}
			return &object.List{Elements: out}
		},
		HelpText: "multimode(data) - The most common values, in the order first seen",
	}
	s["quantiles"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("quantiles", kwargs, "n", "method"); err != nil {
					return err
				}
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			n := int64(4)
			if v := kwargs.Get("n"); v != nil {
				i, ok := v.(*object.Integer)
				if !ok {
					return errors.NewTypeErrorTagged("n must be an int")
				}
				n = i.IntValue()
			}
			if n < 1 {
				return statisticsError("n must be at least 1")
			}
			method := "exclusive"
			if v := kwargs.Get("method"); v != nil {
				m, ok := v.(*object.String)
				if !ok {
					return errors.NewTypeErrorTagged("method must be a str")
				}
				method = m.StringValue()
			}
			data, errObj := extractNumbers(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			if errObj := object.CheckElements(n); errObj != nil {
				return errObj
			}
			sort.Float64s(data)
			ld := int64(len(data))
			out := make([]object.Object, 0, n-1)
			switch {
			case ld == 1:
				for i := int64(1); i < n; i++ {
					out = append(out, object.NewFloat(data[0]))
				}
				return &object.List{Elements: out}
			case ld < 1:
				return statisticsError("must have at least one data point")
			}
			nf := float64(n)
			switch method {
			case "inclusive":
				m := ld - 1
				for i := int64(1); i < n; i++ {
					j, delta := (i*m)/n, (i*m)%n
					out = append(out, object.NewFloat((data[j]*float64(n-delta)+data[j+1]*float64(delta))/nf))
				}
			case "exclusive":
				m := ld + 1
				for i := int64(1); i < n; i++ {
					j := i * m / n
					j = max(1, min(j, ld-1))
					delta := i*m - j*n
					out = append(out, object.NewFloat((data[j-1]*float64(n-delta)+data[j]*float64(delta))/nf))
				}
			default:
				return errors.NewValueError("Unknown method: %s", object.ReprString(method))
			}
			return &object.List{Elements: out}
		},
		HelpText: "quantiles(data, *, n=4, method='exclusive') - The n-1 cut points dividing data into n equal parts",
	}
	s["covariance"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("covariance", kwargs); err != nil {
				return err
			}
			x, y, errObj := pairedData(ctx, "covariance", args)
			if errObj != nil {
				return errObj
			}
			sxy, _, _, _, _ := deviationSums(x, y)
			return object.NewFloat(sxy / float64(len(x)-1))
		},
		HelpText: "covariance(x, y) - The sample covariance of two data sets",
	}
	s["correlation"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("correlation", kwargs, "method"); err != nil {
					return err
				}
			}
			x, y, errObj := pairedData(ctx, "correlation", args)
			if errObj != nil {
				return errObj
			}
			if m := kwargs.Get("method"); m != nil {
				if ms, ok := m.(*object.String); !ok || (ms.StringValue() != "linear" && ms.StringValue() != "ranked") {
					return errors.NewValueError("Unknown method: %s", m.Inspect())
				} else if ms.StringValue() == "ranked" {
					x, y = ranks(x), ranks(y)
				}
			}
			sxy, sxx, syy, _, _ := deviationSums(x, y)
			if sxx == 0 || syy == 0 {
				return statisticsError("at least one of the inputs is constant")
			}
			return object.NewFloat(sxy / math.Sqrt(sxx*syy))
		},
		HelpText: "correlation(x, y, /, *, method='linear') - Pearson's (or, ranked, Spearman's) correlation coefficient",
	}
	s["linear_regression"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("linear_regression", kwargs, "proportional"); err != nil {
					return err
				}
			}
			x, y, errObj := pairedData(ctx, "linear_regression", args)
			if errObj != nil {
				return errObj
			}
			var slope, intercept float64
			if p := kwargs.Get("proportional"); p != nil && isTruthy(p) {
				pxy, pxx := make([]float64, len(x)), make([]float64, len(x))
				for i := range x {
					pxy[i], pxx[i] = x[i]*y[i], x[i]*x[i]
				}
				sxx := exactSum(pxx)
				if sxx == 0 {
					return statisticsError("x is constant")
				}
				slope = exactSum(pxy) / sxx
			} else {
				sxy, sxx, _, xbar, ybar := deviationSums(x, y)
				if sxx == 0 {
					return statisticsError("x is constant")
				}
				slope = sxy / sxx
				intercept = ybar - slope*xbar
			}
			return object.NewInstanceWithFields(linearRegressionClass, map[string]object.Object{
				"slope":     object.NewFloat(slope),
				"intercept": object.NewFloat(intercept),
			})
		},
		HelpText: "linear_regression(x, y, /, *, proportional=False) - LinearRegression(slope, intercept) of the least-squares line",
	}

	m := MathLibrary.Functions()
	m["hypot"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("hypot", kwargs); err != nil {
				return err
			}
			total := 0.0
			scale := 0.0
			vals := make([]float64, len(args))
			for i, a := range args {
				f, err := a.AsFloat()
				if err != nil {
					return errors.NewTypeErrorTagged("must be real number, not %s", object.PythonTypeName(a))
				}
				if math.IsInf(f, 0) {
					return object.NewFloat(math.Inf(1))
				}
				vals[i] = math.Abs(f)
				scale = max(scale, vals[i])
			}
			if len(args) == 2 {
				return object.NewFloat(math.Hypot(vals[0], vals[1]))
			}
			if scale == 0 || math.IsNaN(scale) {
				for _, v := range vals {
					if math.IsNaN(v) {
						return object.NewFloat(math.NaN())
					}
				}
				return object.NewFloat(0)
			}
			for _, v := range vals {
				r := v / scale
				total += r * r
			}
			return object.NewFloat(scale * math.Sqrt(total))
		},
		HelpText: "hypot(*coordinates) - The Euclidean distance from the origin, sqrt(sum(x*x))",
	}
	m["ulp"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("ulp", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			x, err := args[0].AsFloat()
			if err != nil {
				return err
			}
			x = math.Abs(x)
			if math.IsNaN(x) || math.IsInf(x, 0) {
				return object.NewFloat(x)
			}
			next := math.Nextafter(x, math.Inf(1))
			if math.IsInf(next, 0) {
				return object.NewFloat(x - math.Nextafter(x, 0))
			}
			return object.NewFloat(next - x)
		},
		HelpText: "ulp(x) - The value of the least significant bit of x",
	}
	m["sumprod"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("sumprod", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			p, errObj := iterableElements(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			q, errObj := iterableElements(ctx, args[1])
			if errObj != nil {
				return errObj
			}
			if len(p) != len(q) {
				return errors.NewValueError("Inputs are not the same length")
			}
			allInt := true
			var isum int64
			for i := range p {
				a, aok := p[i].(*object.Integer)
				b, bok := q[i].(*object.Integer)
				if !aok || !bok {
					allInt = false
					break
				}
				prod := a.IntValue() * b.IntValue()
				if a.IntValue() != 0 && prod/a.IntValue() != b.IntValue() {
					return errors.NewOverflowError("integer overflow in sumprod")
				}
				isum += prod
			}
			if allInt {
				return object.NewInteger(isum)
			}
			prods := make([]float64, len(p))
			for i := range p {
				a, err := p[i].AsFloat()
				if err != nil {
					return err
				}
				b, err := q[i].AsFloat()
				if err != nil {
					return err
				}
				prods[i] = a * b
			}
			return object.NewFloat(exactSum(prods))
		},
		HelpText: "sumprod(p, q) - The sum of products of the values of two iterables",
	}

	StringLibrary.Functions()["capwords"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("capwords", kwargs, "sep"); err != nil {
					return err
				}
			}
			if len(args) < 1 || len(args) > 2 {
				return errors.NewTypeErrorTagged("capwords() takes 1 or 2 arguments (%d given)", len(args))
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeErrorTagged("capwords() argument must be str, not %s", object.PythonTypeName(args[0]))
			}
			sepObj := kwargs.Get("sep")
			if len(args) == 2 {
				sepObj = args[1]
			}
			cap := func(w string) string {
				r := []rune(w)
				if len(r) == 0 {
					return w
				}
				return strings.ToUpper(string(r[0])) + strings.ToLower(string(r[1:]))
			}
			var words []string
			sep := " "
			if sepObj == nil || sepObj.Type() == object.NULL_OBJ {
				words = strings.Fields(str.StringValue())
			} else {
				s, ok := sepObj.(*object.String)
				if !ok {
					return errors.NewTypeErrorTagged("sep must be str")
				}
				sep = s.StringValue()
				words = strings.Split(str.StringValue(), sep)
			}
			for i, w := range words {
				words[i] = cap(w)
			}
			return object.NewString(strings.Join(words, sep))
		},
		HelpText: "capwords(s, sep=None) - Capitalize each word: split on sep (whitespace runs), capitalize, join",
	}
}

// ranks are the 1-based ranks of xs, ties sharing their mean rank.
func ranks(xs []float64) []float64 {
	idx := make([]int, len(xs))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return xs[idx[a]] < xs[idx[b]] })
	out := make([]float64, len(xs))
	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && xs[idx[j+1]] == xs[idx[i]] {
			j++
		}
		r := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			out[idx[k]] = r
		}
		i = j + 1
	}
	return out
}
