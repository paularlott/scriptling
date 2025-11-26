package stdlib

import (
	"context"
	"math/rand"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

var randomLibrary = object.NewLibrary(map[string]*object.Builtin{
	"randint": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
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
	},
	"random": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			return &object.Float{Value: rng.Float64()}
		},
	},
	"choice": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			list, ok := args[0].AsList()
			if !ok {
				return errors.NewTypeError("LIST", args[0].Type().String())
			}
			if len(list) == 0 {
				return errors.NewError("choice() list cannot be empty")
			}
			idx := rng.Intn(len(list))
			return list[idx]
		},
	},
	"shuffle": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			list, ok := args[0].AsList()
			if !ok {
				return errors.NewTypeError("LIST", args[0].Type().String())
			}
			n := len(list)
			for i := n - 1; i > 0; i-- {
				j := rng.Intn(i + 1)
				list[i], list[j] = list[j], list[i]
			}
			return &object.Null{}
		},
	},
})

func GetRandomLibrary() *object.Library {
	return randomLibrary
}
