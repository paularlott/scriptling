package stdlib

import (
	"context"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var startTime = time.Now()

var timeLibrary = object.NewLibrary(map[string]*object.Builtin{
	"time": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			return &object.Float{Value: float64(time.Now().UnixNano()) / 1e9}
		},
	},
	"perf_counter": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			return &object.Float{Value: time.Since(startTime).Seconds()}
		},
	},
	"sleep": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			var seconds float64
			switch arg := args[0].(type) {
			case *object.Integer:
				seconds = float64(arg.Value)
			case *object.Float:
				seconds = arg.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", string(arg.Type()))
			}

			// Create a timer that respects context cancellation
			timer := time.NewTimer(time.Duration(seconds * float64(time.Second)))
			defer timer.Stop()

			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.NewTimeoutError()
				}
				return errors.NewCancelledError()
			case <-timer.C:
				return &object.Null{}
			}
		},
	},
	"strftime": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			format, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			var timestamp float64
			switch t := args[1].(type) {
			case *object.Integer:
				timestamp = float64(t.Value)
			case *object.Float:
				timestamp = t.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", string(args[1].Type()))
			}
			t := time.Unix(int64(timestamp), 0)
			return &object.String{Value: t.Format(pythonToGoFormat(format.Value))}
		},
	},
	"strptime": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			format, ok := args[1].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[1].Type()))
			}
			t, err := time.Parse(pythonToGoFormat(format.Value), str.Value)
			if err != nil {
				return errors.NewError("strptime() parse error: %s", err.Error())
			}
			return &object.Float{Value: float64(t.Unix())}
		},
	},
})

func GetTimeLibrary() *object.Library {
	return timeLibrary
}

func pythonToGoFormat(pyFormat string) string {
	goFormat := pyFormat
	goFormat = replaceAll(goFormat, "%Y", "2006")
	goFormat = replaceAll(goFormat, "%m", "01")
	goFormat = replaceAll(goFormat, "%d", "02")
	goFormat = replaceAll(goFormat, "%H", "15")
	goFormat = replaceAll(goFormat, "%M", "04")
	goFormat = replaceAll(goFormat, "%S", "05")
	return goFormat
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}
