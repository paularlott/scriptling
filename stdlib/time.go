package stdlib

import (
	"context"
	"strings"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var startTime = time.Now()

var TimeLibrary = object.NewLibrary(TimeLibraryName, map[string]*object.Builtin{
	"time": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Float{Value: float64(time.Now().UnixNano()) / 1e9}
		},
		HelpText: `time() - Return current time in seconds

Returns the current time as a floating point number of seconds since the Unix epoch.`,
	},
	"perf_counter": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Float{Value: time.Since(startTime).Seconds()}
		},
		HelpText: `perf_counter() - Return performance counter

Returns the value of a performance counter in fractional seconds.`,
	},
	"sleep": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil { return err }
			seconds, err := args[0].AsFloat()
			if err != nil {
				return errors.ParameterError("seconds", err)
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
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else if len(args) == 1 {
				// Try AsFloat first (handles both Integer and Float)
				if ts, err := args[0].AsFloat(); err == nil {
					t = time.Unix(int64(ts), 0)
				} else if instance, ok := args[0].(*object.Instance); ok {
					// Handle datetime/date instances
					if dt, err := GetTimeFromObject(instance); err == nil {
						t = dt
					} else {
						return errors.NewTypeError("INTEGER, FLOAT, or datetime instance", args[0].Type().String())
					}
				} else {
					return errors.NewTypeError("INTEGER, FLOAT, or datetime instance", args[0].Type().String())
				}
			} else {
				if err := errors.MaxArgs(args, 1); err != nil {
					return err
				}
			}

			return timeToTuple(t, false)
		},
		HelpText: `localtime([timestamp_or_datetime]) - Convert to local time tuple

Returns a time tuple in local time. If timestamp/datetime is omitted, uses current time.`,
	},
	"gmtime": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else if len(args) == 1 {
				// Try AsFloat first (handles both Integer and Float)
				if ts, err := args[0].AsFloat(); err == nil {
					t = time.Unix(int64(ts), 0)
				} else if instance, ok := args[0].(*object.Instance); ok {
					// Handle datetime/date instances
					if dt, err := GetTimeFromObject(instance); err == nil {
						t = dt
					} else {
						return errors.NewTypeError("INTEGER, FLOAT, or datetime instance", args[0].Type().String())
					}
				} else {
					return errors.NewTypeError("INTEGER, FLOAT, or datetime instance", args[0].Type().String())
				}
			} else {
				if err := errors.MaxArgs(args, 1); err != nil {
					return err
				}
			}

			return timeToTuple(t, true)
		},
		HelpText: `gmtime([timestamp_or_datetime]) - Convert to UTC time tuple

Returns a time tuple in UTC. If timestamp/datetime is omitted, uses current time.`,
	},
	"mktime": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil { return err }

			tuple, err := args[0].AsList()
			if err != nil {
				return err
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
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}

			format, err := args[0].AsString()
			if err != nil {
				return err
			}

			var t time.Time
			if len(args) == 1 {
				t = time.Now()
			} else {
				tuple, err := args[1].AsList()
				if err != nil {
					return err
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
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil { return err }

			str, err := args[0].AsString()
			if err != nil {
				return err
			}

			format, err := args[1].AsString()
			if err != nil {
				return err
			}

			t, parseErr := time.Parse(pythonToGoFormat(format), str)
			if parseErr != nil {
				return errors.NewError("strptime() parse error: %s", parseErr.Error())
			}

			return timeToTuple(t, false)
		},
		HelpText: `strptime(string, format) - Parse time from string

Parses a time string according to the given format and returns a time tuple.`,
	},
	"asctime": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else if len(args) == 1 {
				tuple, err := args[0].AsList()
				if err != nil {
					return err
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
				if err := errors.MaxArgs(args, 1); err != nil {
					return err
				}
			}

			return &object.String{Value: t.Format("Mon Jan 2 15:04:05 2006")}
		},
		HelpText: `asctime([tuple]) - Convert time tuple to string

Converts a time tuple to a string in the format 'Mon Jan 2 15:04:05 2006'. If tuple is omitted, uses current time.`,
	},
	"ctime": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else if len(args) == 1 {
				timestamp, err := args[0].AsFloat()
				if err != nil {
					return errors.ParameterError("timestamp", err)
				}
				t = time.Unix(int64(timestamp), 0)
			} else {
				if err := errors.MaxArgs(args, 1); err != nil {
					return err
				}
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
