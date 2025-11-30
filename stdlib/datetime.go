package stdlib

import (
	"context"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var DatetimeLibrary = object.NewLibrary(map[string]*object.Builtin{
	"now": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) > 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			format := "2006-01-02 15:04:05"
			if len(args) == 1 {
				if args[0].Type() != object.STRING_OBJ {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				formatStr, _ := args[0].AsString()
				format = pythonToGoDateFormat(formatStr)
			}

			return &object.String{Value: time.Now().Format(format)}
		},
		HelpText: `now([format]) - Return current local datetime

Returns current date and time in local timezone. Optional format string uses Python datetime format codes.`,
	},
	"utcnow": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) > 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			format := "2006-01-02 15:04:05"
			if len(args) == 1 {
				if args[0].Type() != object.STRING_OBJ {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				formatStr, _ := args[0].AsString()
				format = pythonToGoDateFormat(formatStr)
			}

			return &object.String{Value: time.Now().UTC().Format(format)}
		},
		HelpText: `utcnow([format]) - Return current UTC datetime

Returns current date and time in UTC. Optional format string uses Python datetime format codes.`,
	},
	"today": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) > 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			format := "2006-01-02"
			if len(args) == 1 {
				if args[0].Type() != object.STRING_OBJ {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				formatStr, _ := args[0].AsString()
				format = pythonToGoDateFormat(formatStr)
			}

			now := time.Now()
			return &object.String{Value: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format(format)}
		},
		HelpText: `today([format]) - Return today's date

Returns today's date at midnight. Optional format string uses Python datetime format codes.`,
	},
	"strptime": {
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

			// Convert Python format codes to Go format
			goFormat := pythonToGoDateFormat(format)

			t, err := time.Parse(goFormat, dateStr)
			if err != nil {
				return errors.NewError("datetime.strptime() parse error: %s", err.Error())
			}

			return &object.Datetime{Value: t}
		},
		HelpText: `strptime(date_string, format) - Parse datetime from string

Parses a date string according to the given format and returns a Unix timestamp.`,
	},
	"strftime": {
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

			// Convert Python format codes to Go format
			goFormat := pythonToGoDateFormat(format)

			return &object.String{Value: t.Format(goFormat)}
		},
		HelpText: `strftime(format, timestamp_or_datetime) - Format timestamp or datetime as string

Formats a Unix timestamp or datetime object according to the given format string.`,
	},
	"fromtimestamp": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
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

			format := "2006-01-02 15:04:05"
			if len(args) == 2 {
				if args[1].Type() != object.STRING_OBJ {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
				formatStr, _ := args[1].AsString()
				format = pythonToGoDateFormat(formatStr)
			}

			t := time.Unix(int64(timestamp), 0)
			return &object.String{Value: t.Format(format)}
		},
		HelpText: `fromtimestamp(timestamp[, format]) - Convert timestamp to string

Converts a Unix timestamp to a formatted date string. Optional format uses Python datetime format codes.`,
	},
	"isoformat": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) > 1 {
				return errors.NewArgumentError(len(args), 0)
			}

			var t time.Time
			if len(args) == 0 {
				t = time.Now()
			} else {
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
			}

			return &object.String{Value: t.Format(time.RFC3339)}
		},
		HelpText: `isoformat([timestamp]) - Return ISO 8601 formatted datetime

Returns datetime in ISO 8601 format. If timestamp is omitted, uses current time.`,
	},
	"timestamp": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			return &object.Float{Value: float64(time.Now().Unix())}
		},
		HelpText: `timestamp() - Return current Unix timestamp

Returns the current time as a Unix timestamp (seconds since epoch).`,
	},
	"timedelta": {
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
  datetime.timedelta(weeks=1)                # 604800.0 seconds

Use with timestamps:
  now = datetime.timestamp()
  tomorrow = now + datetime.timedelta(days=1)`,
	},
}, nil, "Date and time manipulation library")

// Helper function to convert Python datetime format codes to Go format
func pythonToGoDateFormat(pyFormat string) string {
	goFormat := pyFormat
	goFormat = replaceAll(goFormat, "%Y", "2006")
	goFormat = replaceAll(goFormat, "%m", "01")
	goFormat = replaceAll(goFormat, "%d", "02")
	goFormat = replaceAll(goFormat, "%H", "15")
	goFormat = replaceAll(goFormat, "%I", "03") // 12-hour format
	goFormat = replaceAll(goFormat, "%M", "04")
	goFormat = replaceAll(goFormat, "%S", "05")
	goFormat = replaceAll(goFormat, "%A", "Monday")
	goFormat = replaceAll(goFormat, "%a", "Mon")
	goFormat = replaceAll(goFormat, "%B", "January")
	goFormat = replaceAll(goFormat, "%b", "Jan")
	goFormat = replaceAll(goFormat, "%p", "PM")
	goFormat = replaceAll(goFormat, "%Z", "MST")
	goFormat = replaceAll(goFormat, "%z", "-0700")
	return goFormat
}
