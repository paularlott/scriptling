package stdlib

import (
	"context"
	"strings"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// PythonToGoReplacer converts Python datetime format codes to Go format in a single pass
var PythonToGoReplacer = strings.NewReplacer(
	"%Y", "2006",
	"%m", "01",
	"%d", "02",
	"%H", "15",
	"%I", "03", // 12-hour format
	"%M", "04",
	"%S", "05",
	"%A", "Monday",
	"%a", "Mon",
	"%B", "January",
	"%b", "Jan",
	"%p", "PM",
	"%Z", "MST",
	"%z", "-0700",
)

// PythonToGoDateFormat converts Python datetime format codes to Go format
func PythonToGoDateFormat(pyFormat string) string {
	return PythonToGoReplacer.Replace(pyFormat)
}

// Shared builtin functions used by both module-level and datetime.datetime class
var strptimeBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) != 2 {
			return errors.NewArgumentError(len(args), 2)
		}

		dateStr, ok := args[0].AsString()
		if !ok {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}

		format, ok := args[1].AsString()
		if !ok {
			return errors.NewTypeError("STRING", args[1].Type().String())
		}

		goFormat := PythonToGoDateFormat(format)

		t, err := time.Parse(goFormat, dateStr)
		if err != nil {
			return errors.NewError("strptime() parse error: %s", err.Error())
		}

		return &object.Datetime{Value: t}
	},
	HelpText: `strptime(date_string, format) - Parse datetime from string

Parses a date string according to the given format and returns a datetime object.`,
}

var strftimeBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) != 2 {
			return errors.NewArgumentError(len(args), 2)
		}

		format, ok := args[0].AsString()
		if !ok {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}

		var t time.Time
		switch dt := args[1].(type) {
		case *object.Integer:
			t = time.Unix(int64(dt.Value), 0)
		case *object.Float:
			t = time.Unix(int64(dt.Value), 0)
		case *object.Datetime:
			t = dt.Value
		default:
			return errors.NewTypeError("INTEGER, FLOAT, or DATETIME", args[1].Type().String())
		}

		goFormat := PythonToGoDateFormat(format)

		return &object.String{Value: t.Format(goFormat)}
	},
	HelpText: `strftime(format, timestamp_or_datetime) - Format timestamp or datetime as string

Formats a Unix timestamp or datetime object according to the given format string.`,
}

var nowDatetimeBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) > 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		return &object.Datetime{Value: time.Now()}
	},
	HelpText: `now() - Return current local datetime as a datetime object`,
}

var utcnowDatetimeBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) > 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		return &object.Datetime{Value: time.Now().UTC()}
	},
	HelpText: `utcnow() - Return current UTC datetime as a datetime object`,
}

var fromtimestampDatetimeBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}

		var timestamp float64
		switch t := args[0].(type) {
		case *object.Integer:
			timestamp = float64(t.Value)
		case *object.Float:
			timestamp = t.Value
		default:
			return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
		}

		return &object.Datetime{Value: time.Unix(int64(timestamp), 0)}
	},
	HelpText: `fromtimestamp(timestamp) - Create datetime from Unix timestamp`,
}

// datetimeConstructor is the callable datetime.datetime class
// It can be called as datetime(year, month, day, ...) to create a datetime object
// and has class methods like datetime.now(), datetime.strptime() as attributes
var datetimeConstructor = &object.Builtin{
	Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		// datetime(year, month, day, hour=0, minute=0, second=0, microsecond=0)
		if len(args) < 3 {
			return errors.NewError("datetime() requires at least 3 arguments: year, month, day")
		}
		if len(args) > 7 {
			return errors.NewError("datetime() takes at most 7 arguments")
		}

		// Extract required positional arguments
		yearObj, ok := args[0].(*object.Integer)
		if !ok {
			return errors.NewTypeError("INTEGER", args[0].Type().String())
		}
		monthObj, ok := args[1].(*object.Integer)
		if !ok {
			return errors.NewTypeError("INTEGER", args[1].Type().String())
		}
		dayObj, ok := args[2].(*object.Integer)
		if !ok {
			return errors.NewTypeError("INTEGER", args[2].Type().String())
		}

		year := int(yearObj.Value)
		month := time.Month(monthObj.Value)
		day := int(dayObj.Value)
		hour, minute, second, nsec := 0, 0, 0, 0

		// Optional positional arguments
		if len(args) > 3 {
			if h, ok := args[3].(*object.Integer); ok {
				hour = int(h.Value)
			} else {
				return errors.NewTypeError("INTEGER", args[3].Type().String())
			}
		}
		if len(args) > 4 {
			if m, ok := args[4].(*object.Integer); ok {
				minute = int(m.Value)
			} else {
				return errors.NewTypeError("INTEGER", args[4].Type().String())
			}
		}
		if len(args) > 5 {
			if s, ok := args[5].(*object.Integer); ok {
				second = int(s.Value)
			} else {
				return errors.NewTypeError("INTEGER", args[5].Type().String())
			}
		}
		if len(args) > 6 {
			if us, ok := args[6].(*object.Integer); ok {
				nsec = int(us.Value) * 1000 // microseconds to nanoseconds
			} else {
				return errors.NewTypeError("INTEGER", args[6].Type().String())
			}
		}

		// Handle keyword arguments (override positional)
		for key, val := range kwargs {
			intVal, ok := val.(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", val.Type().String())
			}
			switch key {
			case "hour":
				hour = int(intVal.Value)
			case "minute":
				minute = int(intVal.Value)
			case "second":
				second = int(intVal.Value)
			case "microsecond":
				nsec = int(intVal.Value) * 1000
			default:
				return errors.NewError("datetime() unexpected keyword argument: %s", key)
			}
		}

		t := time.Date(year, month, day, hour, minute, second, nsec, time.Local)
		return &object.Datetime{Value: t}
	},
	HelpText: `datetime(year, month, day, hour=0, minute=0, second=0, microsecond=0)

Creates a datetime object for the specified date and time.

Parameters:
  year        - Year (required)
  month       - Month 1-12 (required)
  day         - Day of month (required)
  hour        - Hour 0-23 (default 0)
  minute      - Minute 0-59 (default 0)
  second      - Second 0-59 (default 0)
  microsecond - Microseconds (default 0)

Class methods (via attributes):
  datetime.now()           - Current local datetime
  datetime.utcnow()        - Current UTC datetime
  datetime.strptime(s, f)  - Parse string to datetime
  datetime.strftime(f, dt) - Format datetime to string
  datetime.fromtimestamp(t) - Create from Unix timestamp

Example:
  from datetime import datetime
  dt = datetime(2025, 12, 25, 10, 30)
  now = datetime.now()`,
	Attributes: map[string]object.Object{
		"strptime":      strptimeBuiltin,
		"strftime":      strftimeBuiltin,
		"now":           nowDatetimeBuiltin,
		"utcnow":        utcnowDatetimeBuiltin,
		"fromtimestamp": fromtimestampDatetimeBuiltin,
	},
}

// timedeltaBuiltin provides Python-compatible datetime.timedelta function
var timedeltaBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		// timedelta accepts keyword arguments only, no positional args
		if len(args) > 0 {
			return errors.NewError("timedelta() takes no positional arguments")
		}

		var days, seconds, microseconds, milliseconds, minutes, hours, weeks float64

		// Helper to extract numeric value from object
		extractNum := func(key string, obj object.Object) (float64, object.Object) {
			switch v := obj.(type) {
			case *object.Integer:
				return float64(v.Value), nil
			case *object.Float:
				return v.Value, nil
			default:
				return 0, errors.NewTypeError("INTEGER or FLOAT", obj.Type().String())
			}
		}

		// Process keyword arguments
		for key, val := range kwargs {
			num, err := extractNum(key, val)
			if err != nil {
				return err
			}

			switch key {
			case "days":
				days = num
			case "seconds":
				seconds = num
			case "microseconds":
				microseconds = num
			case "milliseconds":
				milliseconds = num
			case "minutes":
				minutes = num
			case "hours":
				hours = num
			case "weeks":
				weeks = num
			default:
				return errors.NewError("timedelta() unexpected keyword argument: %s", key)
			}
		}

		// Calculate total seconds (matching Python's timedelta.total_seconds())
		totalSeconds := weeks*7*24*3600 +
			days*24*3600 +
			hours*3600 +
			minutes*60 +
			seconds +
			milliseconds/1000 +
			microseconds/1000000

		return &object.Float{Value: totalSeconds}
	},
	HelpText: `timedelta(days=0, seconds=0, microseconds=0, milliseconds=0, minutes=0, hours=0, weeks=0)

Creates a timedelta representing a duration. Returns the total duration in seconds.

Parameters (all optional, keyword-only):
  days         - Number of days
  seconds      - Number of seconds
  microseconds - Number of microseconds
  milliseconds - Number of milliseconds
  minutes      - Number of minutes
  hours        - Number of hours
  weeks        - Number of weeks

Returns: Float (total duration in seconds)

Examples:
  datetime.timedelta(days=1)                 # 86400.0 seconds
  datetime.timedelta(hours=2, minutes=30)    # 9000.0 seconds
  datetime.timedelta(weeks=1)                # 604800.0 seconds`,
}

// DatetimeLibrary is the main datetime module with Python-compatible datetime.datetime class
var DatetimeLibrary = object.NewLibrary(
	map[string]*object.Builtin{
		// Python-compatible: timedelta at module level
		"timedelta": timedeltaBuiltin,
		// datetime.datetime is a callable class with attributes for class methods
		"datetime": datetimeConstructor,
	},
	nil, // no constants
	"Date and time manipulation library. Use datetime.datetime(), datetime.datetime.now(), etc.",
)
