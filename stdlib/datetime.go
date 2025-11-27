package stdlib

import (
	"context"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var DatetimeLibrary = object.NewLibrary(map[string]*object.Builtin{
	"now": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
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

			return &object.Float{Value: float64(t.Unix())}
		},
		HelpText: `strptime(date_string, format) - Parse datetime from string

Parses a date string according to the given format and returns a Unix timestamp.`,
	},
	"strftime": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}

			format, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			var timestamp float64
			switch t := args[1].(type) {
			case *object.Integer:
				timestamp = float64(t.Value)
			case *object.Float:
				timestamp = t.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}

			// Convert Python format codes to Go format
			goFormat := pythonToGoDateFormat(format)

			t := time.Unix(int64(timestamp), 0)
			return &object.String{Value: t.Format(goFormat)}
		},
		HelpText: `strftime(format, timestamp) - Format timestamp as string

Formats a Unix timestamp according to the given format string.`,
	},
	"fromtimestamp": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
	"add_days": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
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

			var days float64
			switch d := args[1].(type) {
			case *object.Integer:
				days = float64(d.Value)
			case *object.Float:
				days = d.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}

			t := time.Unix(int64(timestamp), 0)
			newTime := t.AddDate(0, 0, int(days))
			return &object.Float{Value: float64(newTime.Unix())}
		},
		HelpText: `add_days(timestamp, days) - Add days to timestamp

Returns a new timestamp with the specified number of days added.`,
	},
	"add_hours": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
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

			var hours float64
			switch h := args[1].(type) {
			case *object.Integer:
				hours = float64(h.Value)
			case *object.Float:
				hours = h.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}

			t := time.Unix(int64(timestamp), 0)
			duration := time.Duration(hours * float64(time.Hour))
			newTime := t.Add(duration)
			return &object.Float{Value: float64(newTime.Unix())}
		},
		HelpText: `add_hours(timestamp, hours) - Add hours to timestamp

Returns a new timestamp with the specified number of hours added.`,
	},
	"add_minutes": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
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

			var minutes float64
			switch m := args[1].(type) {
			case *object.Integer:
				minutes = float64(m.Value)
			case *object.Float:
				minutes = m.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}

			t := time.Unix(int64(timestamp), 0)
			duration := time.Duration(minutes * float64(time.Minute))
			newTime := t.Add(duration)
			return &object.Float{Value: float64(newTime.Unix())}
		},
		HelpText: `add_minutes(timestamp, minutes) - Add minutes to timestamp

Returns a new timestamp with the specified number of minutes added.`,
	},
	"add_seconds": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
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

			var seconds float64
			switch s := args[1].(type) {
			case *object.Integer:
				seconds = float64(s.Value)
			case *object.Float:
				seconds = s.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}

			t := time.Unix(int64(timestamp), 0)
			duration := time.Duration(seconds * float64(time.Second))
			newTime := t.Add(duration)
			return &object.Float{Value: float64(newTime.Unix())}
		},
		HelpText: `add_seconds(timestamp, seconds) - Add seconds to timestamp

Returns a new timestamp with the specified number of seconds added.`,
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
