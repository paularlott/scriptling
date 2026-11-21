package stdlib

import (
	"context"
	"fmt"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// processStart anchors time.monotonic(): Go monotonic reading since start.
var processStart = time.Now()

var startTime = time.Now()

var TimeLibrary = object.NewLibrary(TimeLibraryName, map[string]*object.Builtin{
	"monotonic": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("monotonic", kwargs); err != nil {
				return err
			}
			// monotonic() - seconds on a clock that never goes backwards
			// (Go's monotonic reading since process start).
			return object.NewFloat(time.Since(processStart).Seconds())
		},
		HelpText: `monotonic() - Monotonic clock in seconds`,
	},
	"now": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("now", kwargs); err != nil {
				return err
			}
			return object.NewString(time.Now().Format("2006-01-02T15:04:05.999999"))
		},
		HelpText: `now() - Return current date and time

Returns the current date and time as an ISO 8601 formatted string (YYYY-MM-DDTHH:MM:SS.ffffff).`,
	},
	"time": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("time", kwargs); err != nil {
				return err
			}
			return object.NewFloat(float64(time.Now().UnixNano()) / 1e9)
		},
		HelpText: `time() - Return current time in seconds

Returns the current time as a floating point number of seconds since the Unix epoch.`,
	},
	"perf_counter": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("perf_counter", kwargs); err != nil {
				return err
			}
			return object.NewFloat(time.Since(startTime).Seconds())
		},
		HelpText: `perf_counter() - Return performance counter

Returns the value of a performance counter in fractional seconds.`,
	},
	"sleep": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("sleep", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			seconds, err := args[0].AsFloat()
			if err != nil {
				return errors.ParameterError("seconds", err)
			}
			if seconds < 0 {
				return errors.NewValueError("sleep length must be non-negative")
			}

			// Create a timer that respects context cancellation
			timer := time.NewTimer(time.Duration(seconds * float64(time.Second)))
			defer timer.Stop()

			// Release the interpreter lock while blocked so other goroutines
			// (e.g. runtime.background shared threads) can run script code.
			var result object.Object = &object.Null{}
			object.RunBlocking(ctx, func() {
				select {
				case <-ctx.Done():
					if ctx.Err() == context.DeadlineExceeded {
						result = errors.NewTimeoutError()
					} else {
						result = errors.NewCancelledError()
					}
				case <-timer.C:
				}
			})
			return result
		},
		HelpText: `sleep(seconds) - Sleep for specified seconds

Suspends execution for the given number of seconds.`,
	},
	"localtime": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("localtime", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("gmtime", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("mktime", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}

			vals, verr := timeTupleValues(args[0])
			if verr != nil {
				return verr
			}

			t := time.Date(int(vals[0]), time.Month(vals[1]), int(vals[2]), int(vals[3]), int(vals[4]), int(vals[5]), 0, time.Local)
			return object.NewFloat(float64(t.Unix()))
		},
		HelpText: `mktime(tuple) - Convert time tuple to timestamp

Converts a time tuple (9 elements) to a Unix timestamp.`,
	},
	"strftime": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("strftime", kwargs); err != nil {
				return err
			}
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
				vals, verr := timeTupleValues(args[1])
				if verr != nil {
					return verr
				}
				year, month, day := vals[0], vals[1], vals[2]
				hour, minute, second := vals[3], vals[4], vals[5]

				t = time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.Local)
			}
			return object.NewString(pyStrftime(t, format, zoneLocal))
		},
		HelpText: `strftime(format[, tuple]) - Format time as string

Formats a time according to the given format string. If tuple is omitted, uses current time.`,
	},
	"strptime": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("strptime", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}

			str, err := args[0].AsString()
			if err != nil {
				return err
			}

			format, err := args[1].AsString()
			if err != nil {
				return err
			}

			r, errObj := pyStrptime(str, format)
			if errObj != nil {
				return errObj
			}
			// struct_time holds the fields as parsed (a leap second
			// included), whatever the zone.
			sec := min(r.second, 59)
			year := r.year
			if r.leapFix {
				year = 1904
			}
			if _, errObj := (&datetimeFields{year, r.month, r.day, r.hour, r.minute, sec, 0}).time(time.UTC); errObj != nil {
				return errObj
			}
			tt := timeToTuple(time.Date(year, time.Month(r.month), r.day, r.hour, r.minute, sec, 0, time.UTC), false)
			setStructTimeField(tt, "tm_year", 0, int64(r.year))
			setStructTimeField(tt, "tm_sec", 5, int64(r.second))
			setStructTimeField(tt, "tm_isdst", 8, -1)
			return tt
		},
		HelpText: `strptime(string, format) - Parse time from string

Parses a time string according to the given format and returns a time tuple.`,
	},
	"asctime": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("asctime", kwargs); err != nil {
				return err
			}
			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else if len(args) == 1 {
				vals, verr := timeTupleValues(args[0])
				if verr != nil {
					return verr
				}

				t = time.Date(int(vals[0]), time.Month(vals[1]), int(vals[2]), int(vals[3]), int(vals[4]), int(vals[5]), 0, time.Local)
			} else {
				if err := errors.MaxArgs(args, 1); err != nil {
					return err
				}
			}

			return object.NewString(t.Format("Mon Jan 2 15:04:05 2006"))
		},
		HelpText: `asctime([tuple]) - Convert time tuple to string

Converts a time tuple to a string in the format 'Mon Jan 2 15:04:05 2006'. If tuple is omitted, uses current time.`,
	},
	"ctime": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("ctime", kwargs); err != nil {
				return err
			}
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

			return object.NewString(t.Format("Mon Jan 2 15:04:05 2006"))
		},
		HelpText: `ctime([timestamp]) - Convert timestamp to string

Converts a Unix timestamp to a string in the format 'Mon Jan 2 15:04:05 2006'. If timestamp is omitted, uses current time.`,
	},
}, nil, "Time-related functions library")

// Convert Go time.Time to Scriptling time tuple (list)
func timeToTuple(t time.Time, utc bool) *object.Instance {
	// gmtime() must report UTC components; time.Now()/time.Unix() carry the
	// local zone, so convert before extracting the fields.
	if utc {
		t = t.UTC()
	}

	year, month, day := t.Date()
	hour, minute, second := t.Clock()
	// Python's tm_wday is Monday=0; Go's Weekday is Sunday=0.
	weekday := (int(t.Weekday()) + 6) % 7
	yearday := t.YearDay()
	var isdst int64
	if t.IsDST() {
		isdst = 1
	}

	fields := map[string]object.Object{
		"tm_year":  object.NewInteger(int64(year)),
		"tm_mon":   object.NewInteger(int64(month)),
		"tm_mday":  object.NewInteger(int64(day)),
		"tm_hour":  object.NewInteger(int64(hour)),
		"tm_min":   object.NewInteger(int64(minute)),
		"tm_sec":   object.NewInteger(int64(second)),
		"tm_wday":  object.NewInteger(int64(weekday)),
		"tm_yday":  object.NewInteger(int64(yearday)),
		"tm_isdst": object.NewInteger(isdst),
	}
	inst := object.NewInstanceWithFields(StructTimeClass, fields)
	inst.SetField("_order", &object.List{Elements: []object.Object{
		fields["tm_year"], fields["tm_mon"], fields["tm_mday"],
		fields["tm_hour"], fields["tm_min"], fields["tm_sec"],
		fields["tm_wday"], fields["tm_yday"], fields["tm_isdst"],
	}})
	return inst
}

// timeTupleValues accepts a time tuple as a List (legacy) or a struct_time
// instance (gmtime/localtime output) and returns its 9 integer components.
func timeTupleValues(obj object.Object) ([]int64, object.Object) {
	var elems []object.Object
	switch l := obj.(type) {
	case *object.List:
		elems = l.Elements
	case *object.Tuple:
		elems = l.Elements
	}
	if elems != nil {
		if len(elems) != 9 {
			return nil, errors.NewTypeErrorTagged("time tuple must have exactly 9 elements")
		}
		out := make([]int64, 9)
		for i, e := range elems {
			v, err := e.AsInt()
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	}
	if inst, ok := obj.(*object.Instance); ok {
		if order, ok := inst.Field("_order").(*object.List); ok && len(order.Elements) == 9 {
			out := make([]int64, 9)
			for i, e := range order.Elements {
				v, err := e.AsInt()
				if err != nil {
					return nil, err
				}
				out[i] = v
			}
			return out, nil
		}
	}
	return nil, errors.NewTypeError("time tuple or struct_time", obj.Type().String())
}

// StructTimeClass is time.struct_time: indexable like a 9-tuple and with
// named tm_* attributes, as in Python.
var StructTimeClass = &object.Class{
	Name: "struct_time",
	Methods: map[string]object.Object{
		"__getitem__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("__getitem__", kwargs); err != nil {
					return err
				}
				if err := errors.MinArgs(args, 2); err != nil {
					return err
				}
				inst := args[0].(*object.Instance)
				order, _ := inst.Field("_order").(*object.List)
				if sl, ok := args[1].(*object.Slice); ok && order != nil {
					return &object.Tuple{Elements: sliceElements(order.Elements, sl)}
				}
				idx, err := args[1].AsInt()
				if err != nil {
					name, nerr := args[1].AsString()
					if nerr != nil {
						return err
					}
					if v, ok := inst.GetField(name); ok {
						return v
					}
					return &object.Error{Message: fmt.Sprintf("'time.struct_time' object has no attribute '%s'", name), ExceptionType: object.ExceptionTypeAttributeError}
				}
				if order != nil && idx < 0 {
					idx += int64(len(order.Elements))
				}
				if order == nil || idx < 0 || int(idx) >= len(order.Elements) {
					return errors.NewIndexError("tuple index out of range")
				}
				return order.Elements[idx]
			},
			HelpText: "__getitem__(i) - Index or field access",
		},
		"__len__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("__len__", kwargs); err != nil {
					return err
				}
				return object.NewInteger(9)
			},
			HelpText: "__len__() - Always 9 fields",
		},
	},
}

// setStructTimeField sets a struct_time field by name and position.
func setStructTimeField(tt *object.Instance, name string, idx int, v int64) {
	val := object.NewInteger(v)
	tt.SetField(name, val)
	if order, ok := tt.Field("_order").(*object.List); ok {
		order.Elements[idx] = val
	}
}
