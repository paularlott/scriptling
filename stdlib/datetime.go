package stdlib

import (
	"context"
	"strings"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Boolean constants for datetime comparisons
var (
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

// nativeBoolToBooleanObject converts a native bool to object.Boolean
func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

// PythonToGoReplacer converts Python datetime format codes to Go format in a single pass
var pythonToGoReplacer = strings.NewReplacer(
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
	return pythonToGoReplacer.Replace(pyFormat)
}

// Forward declarations for classes (set in init)
var (
	DatetimeClass *object.Class
	DateClass     *object.Class
)

// Helper to get time.Time from an instance's _time field (stored as Unix nanoseconds)
func getTimeFromInstance(instance *object.Instance) (time.Time, bool) {
	if val, ok := instance.Fields["_time"]; ok {
		if ns, ok := val.(*object.Integer); ok {
			return time.Unix(0, ns.Value), true
		}
	}
	return time.Time{}, false
}

// Helper to create a datetime instance (stores time as Unix nanoseconds)
func createDatetimeInstance(t time.Time) *object.Instance {
	return &object.Instance{
		Class: DatetimeClass,
		Fields: map[string]object.Object{
			"_time":        &object.Integer{Value: t.UnixNano()},
			"__str_repr__": &object.String{Value: t.Format("2006-01-02 15:04:05")},
		},
	}
}

// Helper to create a date instance (stores time as Unix nanoseconds)
func createDateInstance(t time.Time) *object.Instance {
	// Normalize to midnight for date
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return &object.Instance{
		Class: DateClass,
		Fields: map[string]object.Object{
			"_time":        &object.Integer{Value: t.UnixNano()},
			"__str_repr__": &object.String{Value: t.Format("2006-01-02")},
		},
	}
}

// isDatetimeInstance checks if an object is a datetime or date instance
func isDatetimeInstance(obj object.Object) bool {
	if inst, ok := obj.(*object.Instance); ok {
		return inst.Class == DatetimeClass || inst.Class == DateClass
	}
	return false
}

// GetTimeFromObject extracts time.Time from a datetime/date instance
func GetTimeFromObject(obj object.Object) (time.Time, bool) {
	if inst, ok := obj.(*object.Instance); ok {
		return getTimeFromInstance(inst)
	}
	return time.Time{}, false
}

func init() {
	// Initialize datetime class
	DatetimeClass = &object.Class{
		Name: "datetime",
		Methods: map[string]object.Object{
			// __str__ for string representation
			"__str__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					return &object.String{Value: t.Format("2006-01-02 15:04:05")}
				},
				HelpText: "__str__() - Return string representation of datetime",
			},
			// Instance methods
			"strftime": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					format, ok := args[1].AsString()
					if !ok {
						return errors.NewTypeError("STRING", args[1].Type().String())
					}
					goFormat := PythonToGoDateFormat(format)
					return &object.String{Value: t.Format(goFormat)}
				},
				HelpText: "strftime(format) - Format datetime as string",
			},
			"timestamp": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					return &object.Float{Value: float64(t.Unix())}
				},
				HelpText: "timestamp() - Return POSIX timestamp",
			},
			"year": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					return object.NewInteger(int64(t.Year()))
				},
				HelpText: "year() - Return the year",
			},
			"month": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					return object.NewInteger(int64(t.Month()))
				},
				HelpText: "month() - Return the month (1-12)",
			},
			"day": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					return object.NewInteger(int64(t.Day()))
				},
				HelpText: "day() - Return the day of month",
			},
			"hour": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					return object.NewInteger(int64(t.Hour()))
				},
				HelpText: "hour() - Return the hour (0-23)",
			},
			"minute": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					return object.NewInteger(int64(t.Minute()))
				},
				HelpText: "minute() - Return the minute (0-59)",
			},
			"second": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					return object.NewInteger(int64(t.Second()))
				},
				HelpText: "second() - Return the second (0-59)",
			},
			"weekday": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					// Python weekday: Monday=0, Sunday=6
					w := int(t.Weekday())
					if w == 0 {
						w = 6
					} else {
						w = w - 1
					}
					return object.NewInteger(int64(w))
				},
				HelpText: "weekday() - Return day of week (Monday=0, Sunday=6)",
			},
			"isoweekday": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					// ISO weekday: Monday=1, Sunday=7
					w := int(t.Weekday())
					if w == 0 {
						w = 7
					}
					return object.NewInteger(int64(w))
				},
				HelpText: "isoweekday() - Return day of week (Monday=1, Sunday=7)",
			},
			"isoformat": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					return &object.String{Value: t.Format("2006-01-02T15:04:05")}
				},
				HelpText: "isoformat() - Return ISO 8601 formatted string",
			},
			"replace": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}

					// Apply replacements from kwargs
					year, month, day := t.Year(), t.Month(), t.Day()
					hour, minute, second, nsec := t.Hour(), t.Minute(), t.Second(), t.Nanosecond()

					for key, val := range kwargs {
						intVal, ok := val.(*object.Integer)
						if !ok {
							return errors.NewTypeError("INTEGER", val.Type().String())
						}
						switch key {
						case "year":
							year = int(intVal.Value)
						case "month":
							month = time.Month(intVal.Value)
						case "day":
							day = int(intVal.Value)
						case "hour":
							hour = int(intVal.Value)
						case "minute":
							minute = int(intVal.Value)
						case "second":
							second = int(intVal.Value)
						case "microsecond":
							nsec = int(intVal.Value) * 1000
						default:
							return errors.NewError("replace() unexpected keyword argument: %s", key)
						}
					}

					newTime := time.Date(year, month, day, hour, minute, second, nsec, t.Location())
					return createDatetimeInstance(newTime)
				},
				HelpText: "replace(**kwargs) - Return datetime with replaced fields",
			},
			// Comparison dunder methods
			"__lt__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return FALSE
					}
					lt, ok := getTimeFromInstance(left)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					rt, ok := getTimeFromInstance(right)
					if !ok {
						return FALSE
					}
					return nativeBoolToBooleanObject(lt.Before(rt))
				},
			},
			"__gt__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return FALSE
					}
					lt, ok := getTimeFromInstance(left)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					rt, ok := getTimeFromInstance(right)
					if !ok {
						return FALSE
					}
					return nativeBoolToBooleanObject(lt.After(rt))
				},
			},
			"__le__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return FALSE
					}
					lt, ok := getTimeFromInstance(left)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					rt, ok := getTimeFromInstance(right)
					if !ok {
						return FALSE
					}
					return nativeBoolToBooleanObject(!lt.After(rt))
				},
			},
			"__ge__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return FALSE
					}
					lt, ok := getTimeFromInstance(left)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					rt, ok := getTimeFromInstance(right)
					if !ok {
						return FALSE
					}
					return nativeBoolToBooleanObject(!lt.Before(rt))
				},
			},
			"__eq__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return FALSE
					}
					lt, ok := getTimeFromInstance(left)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					rt, ok := getTimeFromInstance(right)
					if !ok {
						return FALSE
					}
					return nativeBoolToBooleanObject(lt.Equal(rt))
				},
			},
			"__ne__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return TRUE
					}
					lt, ok := getTimeFromInstance(left)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					rt, ok := getTimeFromInstance(right)
					if !ok {
						return TRUE
					}
					return nativeBoolToBooleanObject(!lt.Equal(rt))
				},
			},
			"__sub__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[1].Type().String())
					}
					lt, ok := getTimeFromInstance(left)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					rt, ok := getTimeFromInstance(right)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					// Return difference in seconds as float
					return &object.Float{Value: lt.Sub(rt).Seconds()}
				},
			},
			"__add__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					lt, ok := getTimeFromInstance(left)
					if !ok {
						return errors.NewError("invalid datetime instance")
					}
					// Add seconds (as float or int)
					var seconds float64
					switch v := args[1].(type) {
					case *object.Float:
						seconds = v.Value
					case *object.Integer:
						seconds = float64(v.Value)
					default:
						return errors.NewTypeError("number", args[1].Type().String())
					}
					newTime := lt.Add(time.Duration(seconds * float64(time.Second)))
					return createDatetimeInstance(newTime)
				},
			},
		},
	}

	// Initialize date class (shares most methods with datetime)
	DateClass = &object.Class{
		Name: "date",
		Methods: map[string]object.Object{
			"__str__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("date instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid date instance")
					}
					return &object.String{Value: t.Format("2006-01-02")}
				},
				HelpText: "__str__() - Return string representation of date",
			},
			"strftime":   DatetimeClass.Methods["strftime"],
			"year":       DatetimeClass.Methods["year"],
			"month":      DatetimeClass.Methods["month"],
			"day":        DatetimeClass.Methods["day"],
			"weekday":    DatetimeClass.Methods["weekday"],
			"isoweekday": DatetimeClass.Methods["isoweekday"],
			"isoformat": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("date instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid date instance")
					}
					return &object.String{Value: t.Format("2006-01-02")}
				},
				HelpText: "isoformat() - Return ISO 8601 formatted date string",
			},
			"replace": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("date instance", args[0].Type().String())
					}
					t, ok := getTimeFromInstance(instance)
					if !ok {
						return errors.NewError("invalid date instance")
					}

					year, month, day := t.Year(), t.Month(), t.Day()

					for key, val := range kwargs {
						intVal, ok := val.(*object.Integer)
						if !ok {
							return errors.NewTypeError("INTEGER", val.Type().String())
						}
						switch key {
						case "year":
							year = int(intVal.Value)
						case "month":
							month = time.Month(intVal.Value)
						case "day":
							day = int(intVal.Value)
						default:
							return errors.NewError("replace() unexpected keyword argument: %s", key)
						}
					}

					newTime := time.Date(year, month, day, 0, 0, 0, 0, t.Location())
					return createDateInstance(newTime)
				},
				HelpText: "replace(**kwargs) - Return date with replaced fields",
			},
			// Share comparison dunder methods from datetime class
			"__lt__": DatetimeClass.Methods["__lt__"],
			"__gt__": DatetimeClass.Methods["__gt__"],
			"__le__": DatetimeClass.Methods["__le__"],
			"__ge__": DatetimeClass.Methods["__ge__"],
			"__eq__": DatetimeClass.Methods["__eq__"],
			"__ne__": DatetimeClass.Methods["__ne__"],
			"__sub__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("date instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return errors.NewTypeError("date instance", args[1].Type().String())
					}
					lt, ok := getTimeFromInstance(left)
					if !ok {
						return errors.NewError("invalid date instance")
					}
					rt, ok := getTimeFromInstance(right)
					if !ok {
						return errors.NewError("invalid date instance")
					}
					// Return difference in days as integer
					days := int64(lt.Sub(rt).Hours() / 24)
					return object.NewInteger(days)
				},
			},
			"__add__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) < 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("date instance", args[0].Type().String())
					}
					lt, ok := getTimeFromInstance(left)
					if !ok {
						return errors.NewError("invalid date instance")
					}
					// Add days - accept integer (days) or float (seconds from timedelta)
					var days int
					switch v := args[1].(type) {
					case *object.Integer:
						days = int(v.Value)
					case *object.Float:
						// timedelta returns seconds, convert to days
						days = int(v.Value / 86400) // 86400 seconds per day
					default:
						return errors.NewTypeError("integer or float", args[1].Type().String())
					}
					newTime := lt.AddDate(0, 0, days)
					return createDateInstance(newTime)
				},
			},
		},
	}
}

// datetimeConstructorBuiltin is the callable that creates datetime instances
var datetimeConstructorBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) < 3 {
			return errors.NewError("datetime() requires at least 3 arguments: year, month, day")
		}
		if len(args) > 7 {
			return errors.NewError("datetime() takes at most 7 arguments")
		}

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
				nsec = int(us.Value) * 1000
			} else {
				return errors.NewTypeError("INTEGER", args[6].Type().String())
			}
		}

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
		return createDatetimeInstance(t)
	},
	HelpText: `datetime(year, month, day, hour=0, minute=0, second=0, microsecond=0)

Creates a datetime instance for the specified date and time.`,
	Attributes: map[string]object.Object{
		"now": &object.Builtin{
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				return createDatetimeInstance(time.Now())
			},
			HelpText: "now() - Return current local datetime",
		},
		"utcnow": &object.Builtin{
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				return createDatetimeInstance(time.Now().UTC())
			},
			HelpText: "utcnow() - Return current UTC datetime",
		},
		"strftime": &object.Builtin{
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 2 {
					return errors.NewArgumentError(len(args), 2)
				}
				format, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				var t time.Time
				switch ts := args[1].(type) {
				case *object.Integer:
					t = time.Unix(ts.Value, 0)
				case *object.Float:
					t = time.Unix(int64(ts.Value), 0)
				case *object.Instance:
					if dt, ok := getTimeFromInstance(ts); ok {
						t = dt
					} else {
						return errors.NewTypeError("INTEGER, FLOAT, or datetime instance", args[1].Type().String())
					}
				default:
					return errors.NewTypeError("INTEGER, FLOAT, or datetime instance", args[1].Type().String())
				}
				goFormat := PythonToGoDateFormat(format)
				return &object.String{Value: t.Format(goFormat)}
			},
			HelpText: "strftime(format, timestamp_or_datetime) - Format timestamp or datetime as string",
		},
		"strptime": &object.Builtin{
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
				return createDatetimeInstance(t)
			},
			HelpText: "strptime(date_string, format) - Parse string to datetime",
		},
		"fromtimestamp": &object.Builtin{
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
				return createDatetimeInstance(time.Unix(int64(timestamp), 0))
			},
			HelpText: "fromtimestamp(timestamp) - Create datetime from Unix timestamp",
		},
	},
}

// dateConstructorBuiltin is the callable that creates date instances
var dateConstructorBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) != 3 {
			return errors.NewError("date() requires exactly 3 arguments: year, month, day")
		}

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

		t := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		return createDateInstance(t)
	},
	HelpText: `date(year, month, day)

Creates a date instance for the specified date.`,
	Attributes: map[string]object.Object{
		"today": &object.Builtin{
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				now := time.Now()
				return createDateInstance(now)
			},
			HelpText: "today() - Return current local date",
		},
	},
}

// timedeltaBuiltinNew provides Python-compatible datetime.timedelta function
var timedeltaBuiltinNew = &object.Builtin{
	Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) > 0 {
			return errors.NewError("timedelta() takes no positional arguments")
		}

		var days, seconds, microseconds, milliseconds, minutes, hours, weeks float64

		extractNum := func(obj object.Object) (float64, object.Object) {
			switch v := obj.(type) {
			case *object.Integer:
				return float64(v.Value), nil
			case *object.Float:
				return v.Value, nil
			default:
				return 0, errors.NewTypeError("INTEGER or FLOAT", obj.Type().String())
			}
		}

		for key, val := range kwargs {
			num, err := extractNum(val)
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

Creates a timedelta representing a duration. Returns the total duration in seconds.`,
}

// DatetimeLibrary is the main datetime module
var DatetimeLibrary = object.NewLibrary(
	map[string]*object.Builtin{
		"timedelta": timedeltaBuiltinNew,
		"datetime":  datetimeConstructorBuiltin,
		"date":      dateConstructorBuiltin,
	},
	nil,
	"Date and time manipulation library.",
)
