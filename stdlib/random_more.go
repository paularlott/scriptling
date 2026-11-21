package stdlib

import (
	"context"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// randomSeedValue turns seed()'s argument into the generator seed: None is
// the current time; ints, floats, bools, strings and bytes seed
// deterministically.
func randomSeedValue(a object.Object) (int64, object.Object) {
	switch v := a.(type) {
	case nil, *object.Null:
		return time.Now().UnixNano(), nil
	case *object.Integer:
		return v.IntValue(), nil
	case *object.Boolean:
		if v.BoolValue() {
			return 1, nil
		}
		return 0, nil
	case *object.Float:
		return int64(v.FloatValue()), nil
	case *object.String:
		return fnvSeed(v.StringValue()), nil
	case *object.Bytes:
		return fnvSeed("b:" + string(v.BytesValue())), nil
	}
	return 0, errors.NewTypeErrorTagged("The only supported seed types are:\nNone, int, float, str, bytes, and bytearray.")
}

func fnvSeed(s string) int64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return int64(h)
}

func init() {
	RandomLibrary.Functions()["getrandbits"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("getrandbits", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			var k int64
			switch v := args[0].(type) {
			case *object.Integer:
				k = v.IntValue()
			case *object.Boolean:
				if v.BoolValue() {
					k = 1
				}
			default:
				return errors.NewTypeErrorTagged("'%s' object cannot be interpreted as an integer", object.PythonTypeName(args[0]))
			}
			switch {
			case k < 0:
				return errors.NewValueError("number of bits must be non-negative")
			case k == 0:
				return object.NewInteger(0)
			case k > 63:
				return errors.NewOverflowError("getrandbits(%d) does not fit a 64-bit integer; use at most 63 bits", k)
			}
			return object.NewInteger(rng.Int63() >> (63 - k))
		},
		HelpText: `getrandbits(k) - A random int with k random bits (0 <= k <= 63)`,
	}
}
