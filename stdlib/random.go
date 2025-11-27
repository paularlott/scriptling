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

var RandomLibrary = object.NewLibrary(map[string]*object.Builtin{
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
		HelpText: `randint(min, max) - Return random integer

Returns a random integer N such that min <= N <= max.`,
	},
	"random": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			return &object.Float{Value: rng.Float64()}
		},
		HelpText: `random() - Return random float

Returns a random float in the range [0.0, 1.0).`,
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
		HelpText: `choice(list) - Return random element

Returns a randomly selected element from the given list.`,
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
		HelpText: `shuffle(list) - Shuffle list in place

Randomly shuffles the elements of the list in place.`,
	},
}, nil, "Random number generation library")
