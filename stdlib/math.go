package stdlib

import (
	"github.com/paularlott/scriptling/object"
	"math"
)

func GetMathLibrary() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"sqrt": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "sqrt() takes 1 argument"}
				}
				var val float64
				switch arg := args[0].(type) {
				case *object.Integer:
					val = float64(arg.Value)
				case *object.Float:
					val = arg.Value
				default:
					return &object.Error{Message: "sqrt() argument must be number"}
				}
				return &object.Float{Value: math.Sqrt(val)}
			},
		},
		"pow": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.Error{Message: "pow() takes 2 arguments"}
				}
				var base, exp float64
				switch arg := args[0].(type) {
				case *object.Integer:
					base = float64(arg.Value)
				case *object.Float:
					base = arg.Value
				default:
					return &object.Error{Message: "pow() arguments must be numbers"}
				}
				switch arg := args[1].(type) {
				case *object.Integer:
					exp = float64(arg.Value)
				case *object.Float:
					exp = arg.Value
				default:
					return &object.Error{Message: "pow() arguments must be numbers"}
				}
				return &object.Float{Value: math.Pow(base, exp)}
			},
		},
		"abs": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "abs() takes 1 argument"}
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
					return &object.Error{Message: "abs() argument must be number"}
				}
			},
		},
		"floor": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "floor() takes 1 argument"}
				}
				var val float64
				switch arg := args[0].(type) {
				case *object.Integer:
					return arg
				case *object.Float:
					val = arg.Value
				default:
					return &object.Error{Message: "floor() argument must be number"}
				}
				return &object.Integer{Value: int64(math.Floor(val))}
			},
		},
		"ceil": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "ceil() takes 1 argument"}
				}
				var val float64
				switch arg := args[0].(type) {
				case *object.Integer:
					return arg
				case *object.Float:
					val = arg.Value
				default:
					return &object.Error{Message: "ceil() argument must be number"}
				}
				return &object.Integer{Value: int64(math.Ceil(val))}
			},
		},
		"round": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "round() takes 1 argument"}
				}
				var val float64
				switch arg := args[0].(type) {
				case *object.Integer:
					return arg
				case *object.Float:
					val = arg.Value
				default:
					return &object.Error{Message: "round() argument must be number"}
				}
				return &object.Integer{Value: int64(math.Round(val))}
			},
		},
		"min": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) == 0 {
					return &object.Error{Message: "min() requires at least 1 argument"}
				}
				min := args[0]
				minVal := toFloat(min)
				for i := 1; i < len(args); i++ {
					val := toFloat(args[i])
					if val < minVal {
						minVal = val
						min = args[i]
					}
				}
				return min
			},
		},
		"max": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) == 0 {
					return &object.Error{Message: "max() requires at least 1 argument"}
				}
				max := args[0]
				maxVal := toFloat(max)
				for i := 1; i < len(args); i++ {
					val := toFloat(args[i])
					if val > maxVal {
						maxVal = val
						max = args[i]
					}
				}
				return max
			},
		},
		"pi": {
			Fn: func(args ...object.Object) object.Object {
				return &object.Float{Value: math.Pi}
			},
		},
		"e": {
			Fn: func(args ...object.Object) object.Object {
				return &object.Float{Value: math.E}
			},
		},
	}
}

func toFloat(obj object.Object) float64 {
	switch v := obj.(type) {
	case *object.Integer:
		return float64(v.Value)
	case *object.Float:
		return v.Value
	default:
		return 0
	}
}
