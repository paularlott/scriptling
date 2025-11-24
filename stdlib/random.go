package stdlib

import (
	"github.com/paularlott/scriptling/object"
	"math/rand"
	"time"
)

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func GetRandomLibrary() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"randint": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.Error{Message: "randint() takes 2 arguments"}
				}
				var min, max int64
				switch arg := args[0].(type) {
				case *object.Integer:
					min = arg.Value
				default:
					return &object.Error{Message: "randint() arguments must be integers"}
				}
				switch arg := args[1].(type) {
				case *object.Integer:
					max = arg.Value
				default:
					return &object.Error{Message: "randint() arguments must be integers"}
				}
				if min > max {
					return &object.Error{Message: "randint() min must be <= max"}
				}
				val := min + rng.Int63n(max-min+1)
				return &object.Integer{Value: val}
			},
		},
		"random": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 0 {
					return &object.Error{Message: "random() takes no arguments"}
				}
				return &object.Float{Value: rng.Float64()}
			},
		},
		"choice": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "choice() takes 1 argument"}
				}
				list, ok := args[0].(*object.List)
				if !ok {
					return &object.Error{Message: "choice() argument must be list"}
				}
				if len(list.Elements) == 0 {
					return &object.Error{Message: "choice() list cannot be empty"}
				}
				idx := rng.Intn(len(list.Elements))
				return list.Elements[idx]
			},
		},
		"shuffle": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "shuffle() takes 1 argument"}
				}
				list, ok := args[0].(*object.List)
				if !ok {
					return &object.Error{Message: "shuffle() argument must be list"}
				}
				n := len(list.Elements)
				for i := n - 1; i > 0; i-- {
					j := rng.Intn(i + 1)
					list.Elements[i], list.Elements[j] = list.Elements[j], list.Elements[i]
				}
				return &object.Null{}
			},
		},
	}
}
