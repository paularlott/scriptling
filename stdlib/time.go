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
				return errors.NewTypeError("INTEGER or FLOAT", arg.Type().String())
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
	"localtime": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else if len(args) == 1 {
				var timestamp float64
				switch ts := args[0].(type) {
				case *object.Integer:
					timestamp = float64(ts.Value)
				case *object.Float:
					timestamp = ts.Value
				default:
					return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
				}
				t = time.Unix(int64(timestamp), 0)
			} else {
				return errors.NewArgumentError(len(args), 0)
			}

			return timeToTuple(t, false)
		},
	},
	"gmtime": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else if len(args) == 1 {
				var timestamp float64
				switch ts := args[0].(type) {
				case *object.Integer:
					timestamp = float64(ts.Value)
				case *object.Float:
					timestamp = ts.Value
				default:
					return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
				}
				t = time.Unix(int64(timestamp), 0)
			} else {
				return errors.NewArgumentError(len(args), 0)
			}

			return timeToTuple(t, true)
		},
	},
	"mktime": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			tuple, ok := args[0].AsList()
			if !ok {
				return errors.NewTypeError("LIST", args[0].Type().String())
			}

			if len(tuple) != 9 {
				return errors.NewError("time tuple must have exactly 9 elements")
			}

			// Extract values from tuple
			year, _ := tuple[0].AsInt()
			month, _ := tuple[1].AsInt()
			day, _ := tuple[2].AsInt()
			hour, _ := tuple[3].AsInt()
			minute, _ := tuple[4].AsInt()
			second, _ := tuple[5].AsInt()

			t := time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.Local)
			return &object.Float{Value: float64(t.Unix())}
		},
	},
	"strftime": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}

			format, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			var t time.Time
			if len(args) == 1 {
				t = time.Now()
			} else {
				tuple, ok := args[1].AsList()
				if !ok {
					return errors.NewTypeError("LIST", args[1].Type().String())
				}
				if len(tuple) != 9 {
					return errors.NewError("time tuple must have exactly 9 elements")
				}

				// Extract values from tuple
				year, _ := tuple[0].AsInt()
				month, _ := tuple[1].AsInt()
				day, _ := tuple[2].AsInt()
				hour, _ := tuple[3].AsInt()
				minute, _ := tuple[4].AsInt()
				second, _ := tuple[5].AsInt()

				t = time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.Local)
			}

			return &object.String{Value: t.Format(pythonToGoFormat(format))}
		},
	},
	"strptime": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}

			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			format, ok := args[1].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}

			t, err := time.Parse(pythonToGoFormat(format), str)
			if err != nil {
				return errors.NewError("strptime() parse error: %s", err.Error())
			}

			return timeToTuple(t, false)
		},
	},
	"asctime": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else if len(args) == 1 {
				tuple, ok := args[0].AsList()
				if !ok {
					return errors.NewTypeError("LIST", args[0].Type().String())
				}
				if len(tuple) != 9 {
					return errors.NewError("time tuple must have exactly 9 elements")
				}

				// Extract values from tuple
				year, _ := tuple[0].AsInt()
				month, _ := tuple[1].AsInt()
				day, _ := tuple[2].AsInt()
				hour, _ := tuple[3].AsInt()
				minute, _ := tuple[4].AsInt()
				second, _ := tuple[5].AsInt()

				t = time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.Local)
			} else {
				return errors.NewArgumentError(len(args), 0)
			}

			return &object.String{Value: t.Format("Mon Jan 2 15:04:05 2006")}
		},
	},
	"ctime": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else if len(args) == 1 {
				var timestamp float64
				switch ts := args[0].(type) {
				case *object.Integer:
					timestamp = float64(ts.Value)
				case *object.Float:
					timestamp = ts.Value
				default:
					return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
				}
				t = time.Unix(int64(timestamp), 0)
			} else {
				return errors.NewArgumentError(len(args), 0)
			}

			return &object.String{Value: t.Format("Mon Jan 2 15:04:05 2006")}
		},
	},
})

func GetTimeLibrary() *object.Library {
	return timeLibrary
}

// Convert Go time.Time to Scriptling time tuple (list)
func timeToTuple(t time.Time, utc bool) *object.List {
	var elements []object.Object

	// Get components
	year, month, day := t.Date()
	hour, minute, second := t.Clock()
	weekday := int(t.Weekday())
	yearday := t.YearDay()

	// DST flag (simplified - Go doesn't provide this directly)
	dst := 0

	elements = []object.Object{
		&object.Integer{Value: int64(year)},
		&object.Integer{Value: int64(month)},
		&object.Integer{Value: int64(day)},
		&object.Integer{Value: int64(hour)},
		&object.Integer{Value: int64(minute)},
		&object.Integer{Value: int64(second)},
		&object.Integer{Value: int64(weekday)},
		&object.Integer{Value: int64(yearday)},
		&object.Integer{Value: int64(dst)},
	}

	return &object.List{Elements: elements}
}

func pythonToGoFormat(pyFormat string) string {
	goFormat := pyFormat
	goFormat = replaceAll(goFormat, "%Y", "2006")
	goFormat = replaceAll(goFormat, "%m", "01")
	goFormat = replaceAll(goFormat, "%d", "02")
	goFormat = replaceAll(goFormat, "%H", "15")
	goFormat = replaceAll(goFormat, "%M", "04")
	goFormat = replaceAll(goFormat, "%S", "05")
	goFormat = replaceAll(goFormat, "%A", "Monday")
	goFormat = replaceAll(goFormat, "%a", "Mon")
	goFormat = replaceAll(goFormat, "%B", "January")
	goFormat = replaceAll(goFormat, "%b", "Jan")
	goFormat = replaceAll(goFormat, "%p", "PM")
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
