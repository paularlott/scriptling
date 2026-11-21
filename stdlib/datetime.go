package stdlib

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

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

// PythonToGoDateFormat converts Python datetime format codes to Go format.
// strftime and strptime no longer use it: they handle every directive
// themselves (see strftime.go).
func PythonToGoDateFormat(pyFormat string) string {
	return pythonToGoReplacer.Replace(pyFormat)
}

// Forward declarations for classes (set in init)
var (
	DatetimeClass *object.Class
	DateClass     *object.Class
)

// Helper to get time.Time from an instance's _time field (stored as Unix
// microseconds, Python's resolution, which spans years 1 to 9999 where
// nanoseconds would wrap outside 1678-2262)
func getTimeFromInstance(instance *object.Instance) (time.Time, object.Object) {
	if val, ok := instance.GetField("_time"); ok {
		if us, ok := val.(*object.Integer); ok {
			t := time.UnixMicro(us.IntValue())
			// time.Unix always yields the local zone, but instances created
			// from UTC times (utcnow, strptime) must render their fields in
			// UTC, so restore the zone recorded at creation.
			if flag, ok := instance.GetField("_utc"); ok {
				if b, ok := flag.(*object.Boolean); ok && b.BoolValue() {
					t = t.UTC()
				}
			}
			// An aware datetime renders in its own zone.
			if k, ok := timezoneKey(instance.Field("tzinfo")); ok {
				t = t.In(tzLocation(k))
			}
			return t, nil
		}
	}
	return time.Time{}, &object.Error{Message: "invalid datetime instance", ExceptionType: object.ExceptionTypeTypeError}
}

// Helper to create a datetime instance (stores time as Unix microseconds)
func createDatetimeInstance(t time.Time) *object.Instance {
	// A naive datetime is its wall clock, held in UTC whatever produced it
	// (the local zone for now() and fromtimestamp()), so naive values built
	// any way compare, hash and do arithmetic on the wall clock as Python's
	// do; timestamp() and astimezone() read the wall clock as local time.
	if t.Location() != time.UTC && isNaive(t) {
		t = wallClock(t)
	}
	// year/month/day/... are attributes in Python, not methods: they are
	// set as fields (which resolve before class methods) so dt.year reads
	// as an integer like Python, and dt.year() is "not callable" like Python.
	return object.NewInstanceWithFields(DatetimeClass, map[string]object.Object{
		"_time":        object.NewInteger(t.UnixMicro()),
		"_utc":         object.NewBoolean(t.Location() == time.UTC),
		"__str_repr__": object.NewString(pyDatetimeString(t, " ")),
		"tzinfo":       tzinfoOf(t),
		"year":         object.NewInteger(int64(t.Year())),
		"month":        object.NewInteger(int64(t.Month())),
		"day":          object.NewInteger(int64(t.Day())),
		"hour":         object.NewInteger(int64(t.Hour())),
		"minute":       object.NewInteger(int64(t.Minute())),
		"second":       object.NewInteger(int64(t.Second())),
		"microsecond":  object.NewInteger(int64(t.Nanosecond() / 1000)),
	})
}

// Helper to create a date instance (stores time as Unix microseconds)
func createDateInstance(t time.Time) *object.Instance {
	// A date is its calendar day, held at midnight UTC.
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return object.NewInstanceWithFields(DateClass, map[string]object.Object{
		"_time":        object.NewInteger(t.UnixMicro()),
		"_utc":         object.NewBoolean(t.Location() == time.UTC),
		"__str_repr__": object.NewString(t.Format("2006-01-02")),
		"year":         object.NewInteger(int64(t.Year())),
		"month":        object.NewInteger(int64(t.Month())),
		"day":          object.NewInteger(int64(t.Day())),
	})
}

// GetTimeFromObject extracts time.Time from a datetime/date instance
func GetTimeFromObject(obj object.Object) (time.Time, object.Object) {
	if inst, ok := obj.(*object.Instance); ok {
		return getTimeFromInstance(inst)
	}
	return time.Time{}, &object.Error{Message: "expected datetime instance", ExceptionType: object.ExceptionTypeTypeError}
}

func init() {
	// Initialize datetime class
	DatetimeClass = &object.Class{
		Name: "datetime",
		Methods: map[string]object.Object{
			// __str__ for string representation
			"__str__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__str__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					return object.NewString(pyDatetimeString(t, " "))
				},
				HelpText: "__str__() - Return string representation of datetime",
			},
			// Instance methods
			"strftime": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("strftime", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					format, err := args[1].AsString()
					if err != nil {
						return err
					}
					return object.NewString(pyStrftime(t, format, strftimeZone(t)))
				},
				HelpText: "strftime(format) - Format datetime as string",
			},
			"timestamp": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("timestamp", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					return object.NewFloat(float64(t.Unix()))
				},
				HelpText: "timestamp() - Return POSIX timestamp",
			},
			"year": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("year", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					return object.NewInteger(int64(t.Year()))
				},
				HelpText: "year() - Return the year",
			},
			"month": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("month", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					return object.NewInteger(int64(t.Month()))
				},
				HelpText: "month() - Return the month (1-12)",
			},
			"day": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("day", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					return object.NewInteger(int64(t.Day()))
				},
				HelpText: "day() - Return the day of month",
			},
			"hour": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("hour", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					return object.NewInteger(int64(t.Hour()))
				},
				HelpText: "hour() - Return the hour (0-23)",
			},
			"minute": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("minute", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					return object.NewInteger(int64(t.Minute()))
				},
				HelpText: "minute() - Return the minute (0-59)",
			},
			"second": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("second", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					return object.NewInteger(int64(t.Second()))
				},
				HelpText: "second() - Return the second (0-59)",
			},
			"weekday": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("weekday", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
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
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("isoweekday", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
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
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if kwargs.Len() > 0 {
						if err := errors.CheckKwargs("isoformat", kwargs, "sep"); err != nil {
							return err
						}
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					sep := "T"
					if len(args) > 1 {
						s, ok := args[1].(*object.String)
						if !ok {
							return errors.NewTypeError("STRING", args[1].Type().String())
						}
						sep = s.StringValue()
					} else if v := kwargs.Get("sep"); v != nil {
						s, ok := v.(*object.String)
						if !ok {
							return errors.NewTypeError("STRING", v.Type().String())
						}
						sep = s.StringValue()
					}
					return object.NewString(pyDatetimeString(t, sep))
				},
				HelpText: "isoformat(sep='T') - Return ISO 8601 formatted string (with the UTC offset when aware)",
			},
			"replace": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					vals, errObj := bindDatetimeArgs("replace", datetimeFieldNames, 0, args[1:], kwargs)
					if errObj != nil {
						return errObj
					}
					f := datetimeFields{t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond() / 1000}
					if errObj := f.set(vals[:7]); errObj != nil {
						return errObj
					}
					loc := t.Location()
					if tz := vals[7]; tz != nil {
						// Same wall time, new zone (None makes it naive).
						tzLoc, err := tzinfoLocation(tz)
						if err != nil {
							return err
						}
						loc = time.UTC
						if tzLoc != nil {
							loc = tzLoc
						}
					}
					newTime, errObj := f.time(loc)
					if errObj != nil {
						return errObj
					}
					return createDatetimeInstance(newTime)
				},
				HelpText: "replace(year, month, day, hour, minute, second, microsecond, tzinfo) - Return datetime with the given fields replaced (by position or keyword; tzinfo=None removes the zone)",
			},
			// Comparison dunder methods
			"__lt__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__lt__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return object.NewBoolean(false)
					}
					lt, err := getTimeFromInstance(left)
					if err != nil {
						return err
					}
					rt, err := getTimeFromInstance(right)
					if err != nil {
						return object.NewBoolean(false)
					}
					return object.NewBoolean(lt.Before(rt))
				},
			},
			"__gt__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__gt__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return object.NewBoolean(false)
					}
					lt, err := getTimeFromInstance(left)
					if err != nil {
						return err
					}
					rt, err := getTimeFromInstance(right)
					if err != nil {
						return object.NewBoolean(false)
					}
					return object.NewBoolean(lt.After(rt))
				},
			},
			"__le__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__le__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return object.NewBoolean(false)
					}
					lt, err := getTimeFromInstance(left)
					if err != nil {
						return err
					}
					rt, err := getTimeFromInstance(right)
					if err != nil {
						return object.NewBoolean(false)
					}
					return object.NewBoolean(!lt.After(rt))
				},
			},
			"__ge__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__ge__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return object.NewBoolean(false)
					}
					lt, err := getTimeFromInstance(left)
					if err != nil {
						return err
					}
					rt, err := getTimeFromInstance(right)
					if err != nil {
						return object.NewBoolean(false)
					}
					return object.NewBoolean(!lt.Before(rt))
				},
			},
			"__eq__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__eq__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return object.NewBoolean(false)
					}
					lt, err := getTimeFromInstance(left)
					if err != nil {
						return err
					}
					rt, err := getTimeFromInstance(right)
					if err != nil {
						return object.NewBoolean(false)
					}
					return object.NewBoolean(lt.Equal(rt))
				},
			},
			"__ne__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__ne__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					right, ok := args[1].(*object.Instance)
					if !ok {
						return object.NewBoolean(true)
					}
					lt, err := getTimeFromInstance(left)
					if err != nil {
						return err
					}
					rt, err := getTimeFromInstance(right)
					if err != nil {
						return object.NewBoolean(true)
					}
					return object.NewBoolean(!lt.Equal(rt))
				},
			},
			"__sub__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__sub__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					lt, err := getTimeFromInstance(left)
					if err != nil {
						return err
					}
					// Subtract seconds (as float or int), or another datetime to get the difference
					var seconds float64
					switch v := args[1].(type) {
					case *object.Float:
						seconds = v.FloatValue()
					case *object.Integer:
						seconds = float64(v.IntValue())
					default:
						right, ok := args[1].(*object.Instance)
						if !ok {
							return errors.NewTypeError("datetime instance or number", args[1].Type().String())
						}
						// timedelta instances carry their _total seconds.
						if td, ok := right.Field("_total").(*object.Float); ok {
							seconds = td.FloatValue()
							break
						}
						rt, err := getTimeFromInstance(right)
						if err != nil {
							return err
						}
						// Python: datetime - datetime is a timedelta
						return createTimedeltaInstance(datetimeDiffSeconds(lt, rt))
					}
					newTime, errObj := datetimeAddSeconds(lt, -seconds)
					if errObj != nil {
						return errObj
					}
					return createDatetimeInstance(newTime)
				},
			},
			"__add__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__add__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("datetime instance", args[0].Type().String())
					}
					lt, err := getTimeFromInstance(left)
					if err != nil {
						return err
					}
					// Add seconds (as float or int)
					var seconds float64
					switch v := args[1].(type) {
					case *object.Float:
						seconds = v.FloatValue()
					case *object.Integer:
						seconds = float64(v.IntValue())
					case *object.Instance:
						if td, ok := v.Field("_total").(*object.Float); ok {
							seconds = td.FloatValue()
						} else {
							return errors.NewTypeError("number or timedelta", args[1].Type().String())
						}
					default:
						return errors.NewTypeError("number or timedelta", args[1].Type().String())
					}
					newTime, errObj := datetimeAddSeconds(lt, seconds)
					if errObj != nil {
						return errObj
					}
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
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__str__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("date instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					return object.NewString(t.Format("2006-01-02"))
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
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("isoformat", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("date instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					return object.NewString(t.Format("2006-01-02"))
				},
				HelpText: "isoformat() - Return ISO 8601 formatted date string",
			},
			"replace": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					instance, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("date instance", args[0].Type().String())
					}
					t, err := getTimeFromInstance(instance)
					if err != nil {
						return err
					}
					vals, errObj := bindDatetimeArgs("replace", datetimeFieldNames[:3], 0, args[1:], kwargs)
					if errObj != nil {
						return errObj
					}
					f := datetimeFields{year: t.Year(), month: int(t.Month()), day: t.Day()}
					if errObj := f.set(vals); errObj != nil {
						return errObj
					}
					newTime, errObj := f.time(t.Location())
					if errObj != nil {
						return errObj
					}
					return createDateInstance(newTime)
				},
				HelpText: "replace(year, month, day) - Return date with the given fields replaced (by position or keyword)",
			},
			// Share comparison dunder methods from datetime class
			"__lt__": DatetimeClass.Methods["__lt__"],
			"__gt__": DatetimeClass.Methods["__gt__"],
			"__le__": DatetimeClass.Methods["__le__"],
			"__ge__": DatetimeClass.Methods["__ge__"],
			"__eq__": DatetimeClass.Methods["__eq__"],
			"__ne__": DatetimeClass.Methods["__ne__"],
			"__sub__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__sub__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("date instance", args[0].Type().String())
					}
					lt, err := getTimeFromInstance(left)
					if err != nil {
						return err
					}
					// Subtract days - accept integer (days) or float (seconds from timedelta),
					// or another date to get the difference
					var days int
					switch v := args[1].(type) {
					case *object.Integer:
						days = int(v.IntValue())
					case *object.Float:
						// timedelta returns seconds, convert to days
						days = int(v.FloatValue() / 86400) // 86400 seconds per day
					case *object.Instance:
						if td, ok := v.Field("_total").(*object.Float); ok {
							days = int(td.FloatValue() / 86400)
							break
						}
						// Not a timedelta: a date-to-date subtraction.
						rt, err := getTimeFromInstance(v)
						if err != nil {
							return err
						}
						// Python: date - date is a timedelta (whole days)
						return createTimedeltaInstance(datetimeDiffSeconds(lt, rt))
					default:
						return errors.NewTypeError("date instance or number", args[1].Type().String())
					}
					newTime := lt.AddDate(0, 0, -days)
					if y := newTime.Year(); y < 1 || y > 9999 {
						return errors.NewOverflowError("date value out of range")
					}
					return createDateInstance(newTime)
				},
			},
			"__add__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__add__", kwargs); err != nil {
						return err
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					left, ok := args[0].(*object.Instance)
					if !ok {
						return errors.NewTypeError("date instance", args[0].Type().String())
					}
					lt, err := getTimeFromInstance(left)
					if err != nil {
						return err
					}
					// Add days - accept integer (days) or float (seconds from timedelta)
					var days int
					switch v := args[1].(type) {
					case *object.Integer:
						days = int(v.IntValue())
					case *object.Float:
						// timedelta returns seconds, convert to days
						days = int(v.FloatValue() / 86400) // 86400 seconds per day
					case *object.Instance:
						if td, ok := v.Field("_total").(*object.Float); ok {
							days = int(td.FloatValue() / 86400)
						} else {
							return errors.NewTypeError("date instance or number", args[1].Type().String())
						}
					default:
						return errors.NewTypeError("integer or float", args[1].Type().String())
					}
					newTime := lt.AddDate(0, 0, days)
					if y := newTime.Year(); y < 1 || y > 9999 {
						return errors.NewOverflowError("date value out of range")
					}
					return createDateInstance(newTime)
				},
			},
		},
	}
}

// wallClock returns t's wall-clock fields as a UTC time, for arithmetic on
// naive datetimes, which Python does on the wall clock: a day added across a
// daylight-saving change is still 24 wall-clock hours.
func wallClock(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}

// isNaive reports whether t is a naive datetime: one with no tzinfo.
func isNaive(t time.Time) bool {
	_, aware := awareKey(t.Location())
	return !aware
}

// datetimeDiffSeconds is a - b in seconds: on the wall clock when both are
// naive, as in Python, and computed from Unix seconds so differences beyond
// the 292 years a time.Duration holds stay exact.
func datetimeDiffSeconds(a, b time.Time) float64 {
	if isNaive(a) && isNaive(b) {
		a, b = wallClock(a), wallClock(b)
	}
	return float64(a.Unix()-b.Unix()) + float64(a.Nanosecond()-b.Nanosecond())/1e9
}

// datetimeAddSeconds adds secs to t (on the wall clock when t is naive) with
// Python's OverflowError for a result outside years 1 to 9999.
func datetimeAddSeconds(t time.Time, secs float64) (time.Time, object.Object) {
	if math.IsNaN(secs) || math.Abs(secs) > 4e11 {
		return time.Time{}, errors.NewOverflowError("date value out of range")
	}
	micros := int64(math.Round(secs * 1e6))
	base := t
	if isNaive(t) {
		base = wallClock(t)
	}
	r := time.UnixMicro(base.UnixMicro() + micros).In(base.Location())
	if isNaive(t) {
		r = time.Date(r.Year(), r.Month(), r.Day(), r.Hour(), r.Minute(), r.Second(), r.Nanosecond(), t.Location())
	}
	if y := r.Year(); y < 1 || y > 9999 {
		return time.Time{}, errors.NewOverflowError("date value out of range")
	}
	return r, nil
}

// datetimeFieldNames are datetime's constructor and replace() parameters in
// order; date uses the first three.
var datetimeFieldNames = []string{"year", "month", "day", "hour", "minute", "second", "microsecond", "tzinfo"}

// bindDatetimeArgs binds positional and keyword arguments to names as Python
// does for datetime(), date() and their replace(): a value per name (nil when
// not given), with TypeError for too many arguments, an unknown keyword, a
// name given both ways, or one of the first required names missing.
func bindDatetimeArgs(fn string, names []string, required int, args []object.Object, kwargs object.Kwargs) ([]object.Object, object.Object) {
	if len(args) > len(names) {
		return nil, errors.NewTypeErrorTagged("%s() takes at most %d arguments (%d given)", fn, len(names), len(args))
	}
	vals := make([]object.Object, len(names))
	copy(vals, args)
	if kwargs.Len() > 0 {
		for key, val := range kwargs.All() {
			i := slices.Index(names, key)
			if i < 0 {
				return nil, errors.NewTypeErrorTagged("'%s' is an invalid keyword argument for %s()", key, fn)
			}
			if i < len(args) {
				return nil, errors.NewTypeErrorTagged("argument for %s() given by name ('%s') and position (%d)", fn, key, i+1)
			}
			vals[i] = val
		}
	}
	for i := 0; i < required; i++ {
		if vals[i] == nil {
			return nil, errors.NewTypeErrorTagged("function missing required argument '%s' (pos %d)", names[i], i+1)
		}
	}
	return vals, nil
}

// datetimeFields are the fields of a date or datetime being built.
type datetimeFields struct {
	year, month, day, hour, minute, second, microsecond int
}

// set stores the given values (nil keeps a field) in field order, each of
// which must be an int.
func (f *datetimeFields) set(vals []object.Object) object.Object {
	ptrs := [...]*int{&f.year, &f.month, &f.day, &f.hour, &f.minute, &f.second, &f.microsecond}
	for i, v := range vals {
		if v == nil {
			continue
		}
		n, ok := v.(*object.Integer)
		if !ok {
			return errors.NewTypeErrorTagged("'%s' object cannot be interpreted as an integer", object.PythonTypeName(v))
		}
		val := n.IntValue()
		if val > math.MaxInt32 || val < math.MinInt32 {
			return errors.NewOverflowError("Python int too large to convert to C int")
		}
		*ptrs[i] = int(val)
	}
	return nil
}

// time checks the fields are in range, raising Python's ValueError rather
// than letting time.Date roll 30 February over into March, and returns the
// time they name.
func (f *datetimeFields) time(loc *time.Location) (time.Time, object.Object) {
	switch {
	case f.year < 1 || f.year > 9999:
		return time.Time{}, errors.NewValueError("year %d is out of range", f.year)
	case f.month < 1 || f.month > 12:
		return time.Time{}, errors.NewValueError("month must be in 1..12")
	case f.day < 1 || f.day > daysInMonth(f.year, time.Month(f.month)):
		return time.Time{}, errors.NewValueError("day is out of range for month")
	case f.hour < 0 || f.hour > 23:
		return time.Time{}, errors.NewValueError("hour must be in 0..23")
	case f.minute < 0 || f.minute > 59:
		return time.Time{}, errors.NewValueError("minute must be in 0..59")
	case f.second < 0 || f.second > 59:
		return time.Time{}, errors.NewValueError("second must be in 0..59")
	case f.microsecond < 0 || f.microsecond > 999999:
		return time.Time{}, errors.NewValueError("microsecond must be in 0..999999")
	}
	return time.Date(f.year, time.Month(f.month), f.day, f.hour, f.minute, f.second, f.microsecond*1000, loc), nil
}

// daysInMonth is the number of days in month of year.
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// datetimeConstructorBuiltin is the callable that creates datetime instances
var datetimeConstructorBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		vals, errObj := bindDatetimeArgs("datetime", datetimeFieldNames, 3, args, kwargs)
		if errObj != nil {
			return errObj
		}
		var f datetimeFields
		if errObj := f.set(vals[:7]); errObj != nil {
			return errObj
		}
		loc := time.UTC
		if tz := vals[7]; tz != nil {
			tzLoc, err := tzinfoLocation(tz)
			if err != nil {
				return err
			}
			if tzLoc != nil {
				loc = tzLoc
			}
		}
		t, errObj := f.time(loc)
		if errObj != nil {
			return errObj
		}
		return createDatetimeInstance(t)
	},
	HelpText: `datetime(year, month, day, hour=0, minute=0, second=0, microsecond=0, tzinfo=None)

Creates a datetime instance for the specified date and time. With tzinfo (a
datetime.timezone) the datetime is aware and isoformat() shows its offset.`,
	Attributes: map[string]object.Object{
		"fromisoformat": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("fromisoformat", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				text, err := args[0].AsString()
				if err != nil {
					return err
				}
				// With a UTC offset (or Z) the result is aware.
				for _, layout := range []string{
					"2006-01-02T15:04:05.999999999Z07:00",
					"2006-01-02T15:04:05Z07:00",
					"2006-01-02 15:04:05.999999999Z07:00",
					"2006-01-02 15:04:05Z07:00",
				} {
					if t, parseErr := time.Parse(layout, text); parseErr == nil {
						_, off := t.Zone()
						return createDatetimeInstance(t.In(tzLocation(tzKey{offset: off})))
					}
				}
				for _, layout := range []string{
					"2006-01-02T15:04:05.999999999",
					"2006-01-02T15:04:05",
					"2006-01-02T15:04",
					"2006-01-02 15:04:05.999999999",
					"2006-01-02 15:04:05",
					"2006-01-02 15:04",
					"2006-01-02",
				} {
					if t, parseErr := time.Parse(layout, text); parseErr == nil {
						return createDatetimeInstance(t)
					}
				}
				return &object.Exception{Message: "Invalid isoformat string: '" + text + "'", ExceptionType: object.ExceptionTypeValueError, Raised: true}
			},
			HelpText: "fromisoformat(date_string) - Parse an ISO 8601 datetime string",
		},
		"now": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if kwargs.Len() > 0 {
					if err := errors.CheckKwargs("now", kwargs, "tz"); err != nil {
						return err
					}
				}
				tz := kwargs.Get("tz")
				if len(args) > 0 {
					tz = args[0]
				}
				loc, err := tzinfoLocation(tz)
				if err != nil {
					return err
				}
				if loc != nil {
					return createDatetimeInstance(time.Now().In(loc))
				}
				return createDatetimeInstance(time.Now())
			},
			HelpText: "now(tz=None) - Return the current local datetime, or the current time in tz (aware)",
		},
		"utcnow": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("utcnow", kwargs); err != nil {
					return err
				}
				return createDatetimeInstance(time.Now().UTC())
			},
			HelpText: "utcnow() - Return current UTC datetime",
		},
		"strftime": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("strftime", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				format, err := args[0].AsString()
				if err != nil {
					return err
				}
				var t time.Time
				switch ts := args[1].(type) {
				case *object.Integer:
					t = time.Unix(ts.IntValue(), 0)
				case *object.Float:
					t = time.Unix(int64(ts.FloatValue()), 0)
				case *object.Instance:
					dt, err := getTimeFromInstance(ts)
					if err != nil {
						return err
					}
					t = dt
				default:
					return errors.NewTypeError("INTEGER, FLOAT, or datetime instance", args[1].Type().String())
				}
				return object.NewString(pyStrftime(t, format, strftimeZone(t)))
			},
			HelpText: "strftime(format, timestamp_or_datetime) - Format timestamp or datetime as string",
		},
		"strptime": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("strptime", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				dateStr, err := args[0].AsString()
				if err != nil {
					return err
				}
				format, err := args[1].AsString()
				if err != nil {
					return err
				}
				r, errObj := pyStrptime(dateStr, format)
				if errObj != nil {
					return errObj
				}
				t, errObj := strptimeDatetime(r)
				if errObj != nil {
					return errObj
				}
				return createDatetimeInstance(t)
			},
			HelpText: "strptime(date_string, format) - Parse string to datetime",
		},
		"fromtimestamp": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if kwargs.Len() > 0 {
					if err := errors.CheckKwargs("fromtimestamp", kwargs, "tz"); err != nil {
						return err
					}
				}
				if err := errors.RangeArgs(args, 1, 2); err != nil {
					return err
				}
				var timestamp float64
				switch t := args[0].(type) {
				case *object.Integer:
					timestamp = float64(t.IntValue())
				case *object.Float:
					timestamp = t.FloatValue()
				default:
					return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
				}
				if math.IsNaN(timestamp) || math.Abs(timestamp) > 2.5e11 {
					return errors.NewValueError("year is out of range")
				}
				t := time.UnixMicro(int64(math.Round(timestamp * 1e6)))
				tz := kwargs.Get("tz")
				if len(args) > 1 {
					tz = args[1]
				}
				loc, err := tzinfoLocation(tz)
				if err != nil {
					return err
				}
				if loc != nil {
					t = t.In(loc)
				}
				return createDatetimeInstance(t)
			},
			HelpText: "fromtimestamp(timestamp, tz=None) - Create datetime from Unix timestamp (aware in tz when given)",
		},
	},
}

// dateConstructorBuiltin is the callable that creates date instances
var dateConstructorBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		vals, errObj := bindDatetimeArgs("date", datetimeFieldNames[:3], 3, args, kwargs)
		if errObj != nil {
			return errObj
		}
		var f datetimeFields
		if errObj := f.set(vals); errObj != nil {
			return errObj
		}
		t, errObj := f.time(time.UTC)
		if errObj != nil {
			return errObj
		}
		return createDateInstance(t)
	},
	HelpText: `date(year, month, day)

Creates a date instance for the specified date.`,
	Attributes: map[string]object.Object{
		"fromisoformat": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("fromisoformat", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				text, err := args[0].AsString()
				if err != nil {
					return err
				}
				t, parseErr := time.Parse("2006-01-02", text)
				if parseErr != nil {
					return &object.Exception{Message: "Invalid isoformat string: '" + text + "'", ExceptionType: object.ExceptionTypeValueError, Raised: true}
				}
				return createDateInstance(t)
			},
			HelpText: "fromisoformat(date_string) - Parse an ISO 8601 date string",
		},
		"today": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("today", kwargs); err != nil {
					return err
				}
				now := time.Now()
				return createDateInstance(now)
			},
			HelpText: "today() - Return current local date",
		},
	},
}

// timedeltaBuiltinNew provides Python-compatible datetime.timedelta function
var timedeltaBuiltinNew = &object.Builtin{
	Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if kwargs.Len() > 0 {
			if err := errors.CheckKwargs("timedelta", kwargs, "days", "seconds", "microseconds", "milliseconds", "minutes", "hours", "weeks"); err != nil {
				return err
			}
		}
		// Positional arguments in Python's order.
		positional := []string{"days", "seconds", "microseconds", "milliseconds", "minutes", "hours", "weeks"}
		if len(args) > len(positional) {
			return errors.NewTypeErrorTagged("timedelta() takes at most 7 arguments (%d given)", len(args))
		}

		var days, seconds, microseconds, milliseconds, minutes, hours, weeks float64

		extractNum := func(obj object.Object) (float64, object.Object) {
			switch v := obj.(type) {
			case *object.Integer:
				return float64(v.IntValue()), nil
			case *object.Float:
				return v.FloatValue(), nil
			default:
				return 0, errors.NewTypeError("INTEGER or FLOAT", obj.Type().String())
			}
		}

		values := make(map[string]object.Object, len(args)+kwargs.Len())
		for i, v := range args {
			values[positional[i]] = v
		}
		for key, val := range kwargs.All() {
			if _, dup := values[key]; dup {
				return errors.NewTypeErrorTagged("argument for timedelta() given by name ('%s') and position", key)
			}
			values[key] = val
		}
		for key, val := range values {
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
				return errors.NewTypeErrorTagged("'%s' is an invalid keyword argument for timedelta()", key)
			}
		}

		totalSeconds := weeks*7*24*3600 +
			days*24*3600 +
			hours*3600 +
			minutes*60 +
			seconds +
			milliseconds/1000 +
			microseconds/1000000

		return createTimedeltaInstance(totalSeconds)
	},
	HelpText: `timedelta(days=0, seconds=0, microseconds=0, milliseconds=0, minutes=0, hours=0, weeks=0)

Creates a timedelta representing a duration, with str(), total_seconds(),
and days/seconds/microseconds like Python.`,
}

// createTimedeltaInstance wraps a duration in seconds as a Timedelta object
// whose fields mirror Python's normalization (days, seconds in [0, 86400),
// microseconds in [0, 1e6)) and whose str() matches CPython.
func createTimedeltaInstance(totalSeconds float64) *object.Instance {
	// Python normalizes to whole days plus a non-negative remainder:
	// timedelta(seconds=-5) is days=-1, seconds=86395, printing
	// "-1 day, 23:59:55". Hours are not zero-padded.
	micros := int64(math.Round(totalSeconds * 1e6))
	days := micros / (24 * 3600 * 1000000)
	rem := micros % (24 * 3600 * 1000000)
	if rem < 0 {
		rem += 24 * 3600 * 1000000
		days--
	}
	secs := rem / 1000000
	us := rem % 1000000

	sign := ""
	absDays := days
	if micros < 0 {
		sign = "-"
		absDays = -days
	}
	out := ""
	if absDays != 0 {
		if absDays == 1 {
			out += "1 day, "
		} else {
			out += fmt.Sprintf("%d days, ", absDays)
		}
	}
	timePart := fmt.Sprintf("%d:%02d:%02d", secs/3600, (secs%3600)/60, secs%60)
	if us != 0 {
		timePart += fmt.Sprintf(".%06d", us)
	}
	return object.NewInstanceWithFields(TimedeltaClass, map[string]object.Object{
		"_total":       object.NewFloat(totalSeconds),
		"__str_repr__": object.NewString(sign + out + timePart),
		"days":         object.NewInteger(days),
		"seconds":      object.NewInteger(secs),
		"microseconds": object.NewInteger(us),
	})
}

// TimedeltaClass is the class behind datetime.timedelta values.
var TimedeltaClass = &object.Class{
	Name: "timedelta",
	Methods: map[string]object.Object{
		"total_seconds": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("total_seconds", kwargs); err != nil {
					return err
				}
				if err := errors.MinArgs(args, 1); err != nil {
					return err
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeError("timedelta instance", args[0].Type().String())
				}
				total, ok := inst.Field("_total").(*object.Float)
				if !ok {
					return errors.NewTypeError("timedelta instance", args[0].Type().String())
				}
				return object.NewFloat(total.FloatValue())
			},
			HelpText: "total_seconds() - Return the total duration in seconds",
		},
		"__str__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("__str__", kwargs); err != nil {
					return err
				}
				if err := errors.MinArgs(args, 1); err != nil {
					return err
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeError("timedelta instance", args[0].Type().String())
				}
				if repr, ok := inst.Field("__str_repr__").(*object.String); ok {
					return repr
				}
				return object.NewString("")
			},
			HelpText: "__str__() - Return the Python-style duration string",
		},
		"__lt__": timedeltaCompare("__lt__", "<"),
		"__gt__": timedeltaCompare("__gt__", ">"),
		"__le__": timedeltaCompare("__le__", "<="),
		"__ge__": timedeltaCompare("__ge__", ">="),
		"__eq__": timedeltaCompare("__eq__", "=="),
		"__ne__": timedeltaCompare("__ne__", "!="),
	},
}

// DatetimeLibrary is the main datetime module
var DatetimeLibrary = object.NewLibrary(DatetimeLibraryName,
	map[string]*object.Builtin{
		"timedelta": timedeltaBuiltinNew,
		"datetime":  datetimeConstructorBuiltin,
		"date":      dateConstructorBuiltin,
		"timezone":  timezoneConstructorBuiltin,
		"time":      timeConstructorBuiltin,
	},
	map[string]object.Object{
		"UTC":     utcTimezone,
		"MINYEAR": object.NewInteger(1),
		"MAXYEAR": object.NewInteger(9999),
	},
	"Date and time manipulation library.",
)

// timedeltaCompare builds a comparison dunder for timedeltas: both operands
// compare by their _total seconds; the right side may be a timedelta or a
// number of seconds, matching how the arithmetic dunders accept both.
func timedeltaCompare(name, op string) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs(name, kwargs); err != nil {
				return err
			}
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}
			left, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewTypeError("timedelta instance", args[0].Type().String())
			}
			lt, ok := left.Field("_total").(*object.Float)
			if !ok {
				return errors.NewTypeError("timedelta instance", args[0].Type().String())
			}
			var rt float64
			switch v := args[1].(type) {
			case *object.Float:
				rt = v.FloatValue()
			case *object.Integer:
				rt = float64(v.IntValue())
			case *object.Instance:
				total, ok := v.Field("_total").(*object.Float)
				if !ok {
					return errors.NewTypeError("timedelta or number", args[1].Type().String())
				}
				rt = total.FloatValue()
			default:
				return errors.NewTypeError("timedelta or number", args[1].Type().String())
			}
			l, r := lt.FloatValue(), rt
			var result bool
			switch op {
			case "<":
				result = l < r
			case ">":
				result = l > r
			case "<=":
				result = l <= r
			case ">=":
				result = l >= r
			case "==":
				result = l == r
			case "!=":
				result = l != r
			}
			return object.NewBoolean(result)
		},
		HelpText: "Compare two timedeltas (or a timedelta and a number of seconds)",
	}
}

// Methods datetime, date and timedelta share with Python beyond the core set
// above: datetime.date(), timetuple(), isocalendar(), toordinal(),
// date.fromordinal(), hashing, and timedelta arithmetic.
func init() {
	// The date of the datetime's fields, at local midnight like date(y, m, d),
	// so dt.date() == date(dt.year, dt.month, dt.day).
	dateOnly := func(t time.Time) object.Object {
		return createDateInstance(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC))
	}
	DatetimeClass.Methods["date"] = timeMethod("date() - Return the date part as a date", dateOnly)
	for _, c := range []*object.Class{DatetimeClass, DateClass} {
		c.Methods["timetuple"] = timeMethod("timetuple() - Return (year, month, day, hour, minute, second, weekday, yearday, isdst)", timetuple)
		c.Methods["isocalendar"] = timeMethod("isocalendar() - Return (ISO year, ISO week, ISO weekday)", isocalendar)
		c.Methods["toordinal"] = timeMethod("toordinal() - Return the proleptic Gregorian ordinal, 1 for 0001-01-01", toordinal)
		c.Methods["__hash__"] = timeMethod("__hash__() - Hash of the instant", func(t time.Time) object.Object {
			return object.NewInteger(t.UnixMicro())
		})
	}
	dateConstructorBuiltin.Attributes["fromordinal"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			n, err := args[0].AsInt()
			if err != nil {
				return err
			}
			if n < 1 {
				return errors.NewValueError("ordinal must be >= 1")
			}
			t := time.Unix((n-unixEpochOrdinal)*86400, 0).UTC()
			return createDateInstance(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC))
		},
		HelpText: "fromordinal(n) - Return the date with proleptic Gregorian ordinal n",
	}
	dateFromOrdinal := dateConstructorBuiltin.Attributes["fromordinal"].(*object.Builtin)
	datetimeConstructorBuiltin.Attributes["fromordinal"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("fromordinal", kwargs); err != nil {
				return err
			}
			r := dateFromOrdinal.Fn(ctx, kwargs, args...)
			if inst, ok := r.(*object.Instance); ok {
				t, err := getTimeFromInstance(inst)
				if err != nil {
					return err
				}
				return createDatetimeInstance(t)
			}
			return r
		},
		HelpText: "fromordinal(n) - Return midnight of the day with proleptic Gregorian ordinal n",
	}

	TimedeltaClass.Methods["__add__"] = timedeltaArith(func(l, r float64) float64 { return l + r })
	TimedeltaClass.Methods["__sub__"] = timedeltaArith(func(l, r float64) float64 { return l - r })
	TimedeltaClass.Methods["__mul__"] = timedeltaScale(false)
	TimedeltaClass.Methods["__rmul__"] = timedeltaScale(false)
	TimedeltaClass.Methods["__truediv__"] = timedeltaScale(true)
	TimedeltaClass.Methods["__neg__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			total, err := timedeltaTotal(args, 0)
			if err != nil {
				return err
			}
			return createTimedeltaInstance(-total)
		},
		HelpText: "__neg__() - Return the negated duration",
	}
	TimedeltaClass.Methods["__abs__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			total, err := timedeltaTotal(args, 0)
			if err != nil {
				return err
			}
			return createTimedeltaInstance(math.Abs(total))
		},
		HelpText: "__abs__() - Return the duration without its sign",
	}
	TimedeltaClass.Methods["__hash__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			total, err := timedeltaTotal(args, 0)
			if err != nil {
				return err
			}
			return object.NewInteger(int64(math.Round(total * 1e6)))
		},
		HelpText: "__hash__() - Hash of the duration",
	}
}

// unixEpochOrdinal is the proleptic Gregorian ordinal of 1970-01-01.
const unixEpochOrdinal = 719163

// timeMethod builds an instance method of datetime or date from a function
// of the instance's time.
func timeMethod(help string, fn func(time.Time) object.Object) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			instance, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewTypeError("datetime instance", args[0].Type().String())
			}
			t, err := getTimeFromInstance(instance)
			if err != nil {
				return err
			}
			return fn(t)
		},
		HelpText: help,
	}
}

func isoWeekday(t time.Time) int64 {
	if w := int64(t.Weekday()); w != 0 {
		return w
	}
	return 7
}

func timetuple(t time.Time) object.Object {
	return &object.Tuple{Elements: []object.Object{
		object.NewInteger(int64(t.Year())), object.NewInteger(int64(t.Month())), object.NewInteger(int64(t.Day())),
		object.NewInteger(int64(t.Hour())), object.NewInteger(int64(t.Minute())), object.NewInteger(int64(t.Second())),
		object.NewInteger(isoWeekday(t) - 1), object.NewInteger(int64(t.YearDay())), object.NewInteger(-1),
	}}
}

var (
	isoCalendarOnce  sync.Once
	isoCalendarClass *object.Class
)

// isocalendar returns Python's IsoCalendarDate: a namedtuple of year, week
// and weekday (a plain tuple should the class be unavailable).
func isocalendar(t time.Time) object.Object {
	year, week := t.ISOWeek()
	vals := []object.Object{object.NewInteger(int64(year)), object.NewInteger(int64(week)), object.NewInteger(isoWeekday(t))}
	isoCalendarOnce.Do(func() {
		names := &object.List{Elements: []object.Object{object.NewString("year"), object.NewString("week"), object.NewString("weekday")}}
		r := CollectionsLibrary.Functions()["namedtuple"].Fn(context.Background(), object.Kwargs{}, object.NewString("IsoCalendarDate"), names)
		isoCalendarClass, _ = r.(*object.Class)
	})
	if isoCalendarClass != nil {
		if init, ok := isoCalendarClass.Methods["__init__"].(*object.Builtin); ok {
			inst := object.NewInstance(isoCalendarClass)
			if r := init.Fn(context.Background(), object.Kwargs{}, append([]object.Object{inst}, vals...)...); !object.IsError(r) {
				return inst
			}
		}
	}
	return &object.Tuple{Elements: vals}
}

func toordinal(t time.Time) object.Object {
	midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return object.NewInteger(midnight.Unix()/86400 + unixEpochOrdinal)
}

// timedeltaTotal reads the duration in seconds of the timedelta args[i].
func timedeltaTotal(args []object.Object, i int) (float64, object.Object) {
	if i >= len(args) {
		return 0, errors.NewArgumentError(len(args), i+1)
	}
	if inst, ok := args[i].(*object.Instance); ok {
		if total, ok := inst.Field("_total").(*object.Float); ok {
			return total.FloatValue(), nil
		}
	}
	return 0, errors.NewTypeError("timedelta instance", args[i].Type().String())
}

// timedeltaArith builds timedelta + timedelta and timedelta - timedelta.
func timedeltaArith(op func(l, r float64) float64) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			l, err := timedeltaTotal(args, 0)
			if err != nil {
				return err
			}
			r, err := timedeltaTotal(args, 1)
			if err != nil {
				return err
			}
			return createTimedeltaInstance(op(l, r))
		},
		HelpText: "Add or subtract two timedeltas",
	}
}

// timedeltaScale builds timedelta * number and timedelta / number; divide
// also accepts a timedelta divisor, giving their ratio as a float.
func timedeltaScale(divide bool) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			l, err := timedeltaTotal(args, 0)
			if err != nil {
				return err
			}
			if len(args) < 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			var n float64
			switch v := args[1].(type) {
			case *object.Integer:
				n = float64(v.IntValue())
			case *object.Float:
				n = v.FloatValue()
			default:
				r, rerr := timedeltaTotal(args, 1)
				if rerr != nil || !divide {
					return errors.NewTypeErrorTagged("unsupported operand type for timedelta: '%s'", args[1].Type().String())
				}
				if r == 0 {
					return errors.NewZeroDivisionError()
				}
				return object.NewFloat(l / r)
			}
			if !divide {
				return createTimedeltaInstance(l * n)
			}
			if n == 0 {
				return errors.NewZeroDivisionError()
			}
			return createTimedeltaInstance(l / n)
		},
		HelpText: "Multiply or divide a timedelta by a number",
	}
}
