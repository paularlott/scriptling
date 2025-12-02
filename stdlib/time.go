package stdlib

import (
	"context"
	"strings"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var startTime = time.Now()

var TimeLibrary = object.NewLibrary(map[string]*object.Builtin{
	"time": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			return &object.Float{Value: float64(time.Now().UnixNano()) / 1e9}
		},
		HelpText: `time() - Return current time in seconds

Returns the current time as a floating point number of seconds since the Unix epoch.`,
	},
	"perf_counter": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			return &object.Float{Value: time.Since(startTime).Seconds()}
		},
		HelpText: `perf_counter() - Return performance counter

Returns the value of a performance counter in fractional seconds.`,
	},
	"sleep": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `sleep(seconds) - Sleep for specified seconds

Suspends execution for the given number of seconds.`,
	},
	"localtime": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else if len(args) == 1 {
				switch ts := args[0].(type) {
				case *object.Integer:
					t = time.Unix(int64(ts.Value), 0)
				case *object.Float:
					t = time.Unix(int64(ts.Value), 0)
				case *object.Datetime:
					t = ts.Value
				default:
					return errors.NewTypeError("INTEGER, FLOAT, or DATETIME", args[0].Type().String())
				}
			} else {
				return errors.NewArgumentError(len(args), 0)
			}

			return timeToTuple(t, false)
		},
		HelpText: `localtime([timestamp_or_datetime]) - Convert to local time tuple

Returns a time tuple in local time. If timestamp/datetime is omitted, uses current time.`,
	},
	"gmtime": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else if len(args) == 1 {
				switch ts := args[0].(type) {
				case *object.Integer:
					t = time.Unix(int64(ts.Value), 0)
				case *object.Float:
					t = time.Unix(int64(ts.Value), 0)
				case *object.Datetime:
					t = ts.Value
				default:
					return errors.NewTypeError("INTEGER, FLOAT, or DATETIME", args[0].Type().String())
				}
			} else {
				return errors.NewArgumentError(len(args), 0)
			}

			return timeToTuple(t, true)
		},
		HelpText: `gmtime([timestamp_or_datetime]) - Convert to UTC time tuple

Returns a time tuple in UTC. If timestamp/datetime is omitted, uses current time.`,
	},
	"mktime": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `mktime(tuple) - Convert time tuple to timestamp

Converts a time tuple (9 elements) to a Unix timestamp.`,
	},
	"strftime": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `strftime(format[, tuple]) - Format time as string

Formats a time according to the given format string. If tuple is omitted, uses current time.`,
	},
	"strptime": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `strptime(string, format) - Parse time from string

Parses a time string according to the given format and returns a time tuple.`,
	},
	"asctime": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `asctime([tuple]) - Convert time tuple to string

Converts a time tuple to a string in the format 'Mon Jan 2 15:04:05 2006'. If tuple is omitted, uses current time.`,
	},
	"ctime": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		HelpText: `ctime([timestamp]) - Convert timestamp to string

Converts a Unix timestamp to a string in the format 'Mon Jan 2 15:04:05 2006'. If timestamp is omitted, uses current time.`,
	},
}, nil, "Time-related functions library")

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
	goFormat = strings.ReplaceAll(goFormat, "%Y", "2006")
	goFormat = strings.ReplaceAll(goFormat, "%m", "01")
	goFormat = strings.ReplaceAll(goFormat, "%d", "02")
	goFormat = strings.ReplaceAll(goFormat, "%H", "15")
	goFormat = strings.ReplaceAll(goFormat, "%M", "04")
	goFormat = strings.ReplaceAll(goFormat, "%S", "05")
	goFormat = strings.ReplaceAll(goFormat, "%A", "Monday")
	goFormat = strings.ReplaceAll(goFormat, "%a", "Mon")
	goFormat = strings.ReplaceAll(goFormat, "%B", "January")
	goFormat = strings.ReplaceAll(goFormat, "%b", "Jan")
	goFormat = strings.ReplaceAll(goFormat, "%p", "PM")
	return goFormat
}
