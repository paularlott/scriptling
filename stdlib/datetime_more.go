package stdlib

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// More of Python's datetime: repr() and format() of datetime, date and
// timedelta, the datetime.time class, ctime(), dst(), isoformat(timespec=),
// timedelta floor division and modulo, and the min/max constants.

// TimeClass is the class behind datetime.time values: a time of day,
// stored like a datetime on 1900-01-01 (in its tzinfo's zone when aware,
// UTC otherwise) so the datetime comparisons serve it too.
var TimeClass = &object.Class{Name: "time", Methods: map[string]object.Object{}}

func createTimeInstance(t time.Time) *object.Instance {
	fields := map[string]object.Object{
		"_time":       object.NewInteger(t.UnixMicro()),
		"_utc":        object.NewBoolean(t.Location() == time.UTC),
		"tzinfo":      tzinfoOf(t),
		"hour":        object.NewInteger(int64(t.Hour())),
		"minute":      object.NewInteger(int64(t.Minute())),
		"second":      object.NewInteger(int64(t.Second())),
		"microsecond": object.NewInteger(int64(t.Nanosecond() / 1000)),
	}
	fields["__str_repr__"] = object.NewString(pyTimeString(t, "auto"))
	return object.NewInstanceWithFields(TimeClass, fields)
}

// timeOfDay is t's wall-clock time on 1900-01-01, in loc (UTC when nil).
func timeOfDay(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	return time.Date(1900, 1, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
}

// isoTime renders hours to microseconds as isoformat's timespec asks.
func isoTime(t time.Time, timespec string) (string, object.Object) {
	us := t.Nanosecond() / 1000
	switch timespec {
	case "auto":
		if us != 0 {
			return fmt.Sprintf("%02d:%02d:%02d.%06d", t.Hour(), t.Minute(), t.Second(), us), nil
		}
		return fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second()), nil
	case "hours":
		return fmt.Sprintf("%02d", t.Hour()), nil
	case "minutes":
		return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute()), nil
	case "seconds":
		return fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second()), nil
	case "milliseconds":
		return fmt.Sprintf("%02d:%02d:%02d.%03d", t.Hour(), t.Minute(), t.Second(), us/1000), nil
	case "microseconds":
		return fmt.Sprintf("%02d:%02d:%02d.%06d", t.Hour(), t.Minute(), t.Second(), us), nil
	}
	return "", errors.NewValueError("Unknown timespec value")
}

// pyTimeString is a time's str() / isoformat(): the offset follows when aware.
func pyTimeString(t time.Time, timespec string) string {
	s, _ := isoTime(t, timespec)
	if _, aware := awareKey(t.Location()); aware {
		_, off := t.Zone()
		s += formatUTCOffset(off, true)
	}
	return s
}

// tzReprString is repr() of the timezone with key k.
func tzReprString(k tzKey) string {
	if k == (tzKey{}) {
		return "datetime.timezone.utc"
	}
	if k.name != "" {
		return fmt.Sprintf("datetime.timezone(%s, %s)", timedeltaRepr(k.offset), object.ReprString(k.name))
	}
	return fmt.Sprintf("datetime.timezone(%s)", timedeltaRepr(k.offset))
}

// clockRepr is the ", h, m[, s[, us]]" part of a datetime or time repr.
func clockRepr(t time.Time, always bool) string {
	var b strings.Builder
	us := t.Nanosecond() / 1000
	if always || t.Hour() != 0 || t.Minute() != 0 || t.Second() != 0 || us != 0 {
		fmt.Fprintf(&b, "%d, %d", t.Hour(), t.Minute())
		if t.Second() != 0 || us != 0 {
			fmt.Fprintf(&b, ", %d", t.Second())
		}
		if us != 0 {
			fmt.Fprintf(&b, ", %d", us)
		}
	}
	if k, aware := awareKey(t.Location()); aware {
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		b.WriteString("tzinfo=" + tzReprString(k))
	}
	return b.String()
}

// timedeltaReprString is repr() of a duration in seconds, as Python shows
// it: datetime.timedelta(days=1, seconds=3661), datetime.timedelta(0).
func timedeltaReprString(total float64) string {
	micros := int64(math.Round(total * 1e6))
	const day = 24 * 3600 * 1000000
	days := micros / day
	rem := micros % day
	if rem < 0 {
		rem += day
		days--
	}
	var parts []string
	if days != 0 {
		parts = append(parts, fmt.Sprintf("days=%d", days))
	}
	if s := rem / 1000000; s != 0 {
		parts = append(parts, fmt.Sprintf("seconds=%d", s))
	}
	if us := rem % 1000000; us != 0 {
		parts = append(parts, fmt.Sprintf("microseconds=%d", us))
	}
	if len(parts) == 0 {
		return "datetime.timedelta(0)"
	}
	return "datetime.timedelta(" + strings.Join(parts, ", ") + ")"
}

// selfTime reads the receiver of a datetime, date or time method.
func selfTime(name string, args []object.Object, kwargs object.Kwargs) (time.Time, object.Object) {
	if err := errors.NoKwargs(name, kwargs); err != nil {
		return time.Time{}, err
	}
	return datetimeSelf(args)
}

func strMethod(name, help string, fn func(time.Time) string) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			t, err := selfTime(name, args, kwargs)
			if err != nil {
				return err
			}
			return object.NewString(fn(t))
		},
		HelpText: help,
	}
}

// formatMethod is __format__: strftime with a non-empty spec, else str().
func formatMethod(str func(time.Time) string) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			t, err := selfTime("__format__", args, kwargs)
			if err != nil {
				return err
			}
			if len(args) < 2 {
				return errors.NewArgumentError(len(args)-1, 1)
			}
			spec, err := args[1].AsString()
			if err != nil {
				return err
			}
			if spec == "" {
				return object.NewString(str(t))
			}
			return object.NewString(pyStrftime(t, spec, strftimeZone(t)))
		},
		HelpText: "__format__(spec) - strftime(spec), or str() for an empty spec",
	}
}

// isoformatMethod is isoformat(sep='T', timespec='auto') for datetime and
// isoformat(timespec='auto') for time.
func isoformatMethod(withSep bool) *object.Builtin {
	names := []string{"timespec"}
	if withSep {
		names = []string{"sep", "timespec"}
	}
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("isoformat", kwargs, names...); err != nil {
					return err
				}
			}
			t, err := datetimeSelf(args)
			if err != nil {
				return err
			}
			if len(args)-1 > len(names) {
				return errors.NewTypeErrorTagged("isoformat() takes at most %d arguments (%d given)", len(names), len(args)-1)
			}
			vals := map[string]string{"sep": "T", "timespec": "auto"}
			for i, a := range args[1:] {
				s, ok := a.(*object.String)
				if !ok {
					return errors.NewTypeErrorTagged("isoformat() argument %d must be str, not %s", i+1, object.PythonTypeName(a))
				}
				vals[names[i]] = s.StringValue()
			}
			for k, v := range kwargs.All() {
				s, ok := v.(*object.String)
				if !ok {
					return errors.NewTypeErrorTagged("isoformat() argument '%s' must be str, not %s", k, object.PythonTypeName(v))
				}
				vals[k] = s.StringValue()
			}
			clock, errObj := isoTime(t, vals["timespec"])
			if errObj != nil {
				return errObj
			}
			if _, aware := awareKey(t.Location()); aware {
				_, off := t.Zone()
				clock += formatUTCOffset(off, true)
			}
			if !withSep {
				return object.NewString(clock)
			}
			return object.NewString(t.Format("2006-01-02") + vals["sep"] + clock)
		},
		HelpText: "isoformat(sep='T', timespec='auto') - ISO 8601 text; timespec is auto, hours, minutes, seconds, milliseconds or microseconds",
	}
}

func ctimeString(t time.Time) string { return pyStrftime(t, "%a %b %e %H:%M:%S %Y", zoneNone) }

// timeConstructorBuiltin is datetime.time(hour=0, minute=0, second=0,
// microsecond=0, tzinfo=None).
var timeConstructorBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		names := []string{"hour", "minute", "second", "microsecond", "tzinfo"}
		vals, errObj := bindDatetimeArgs("time", names, 0, args, kwargs)
		if errObj != nil {
			return errObj
		}
		f := datetimeFields{year: 1900, month: 1, day: 1}
		if errObj := f.set(append([]object.Object{nil, nil, nil}, vals[:4]...)); errObj != nil {
			return errObj
		}
		loc, errObj := tzinfoLocation(vals[4])
		if errObj != nil {
			return errObj
		}
		if loc == nil {
			loc = time.UTC
		}
		t, errObj := f.time(loc)
		if errObj != nil {
			return errObj
		}
		return createTimeInstance(t)
	},
	HelpText:   `time(hour=0, minute=0, second=0, microsecond=0, tzinfo=None) - A time of day`,
	Attributes: map[string]object.Object{},
}

func init() {
	dm, am, tm, tdm := DatetimeClass.Methods, DateClass.Methods, TimeClass.Methods, TimedeltaClass.Methods

	dm["__repr__"] = strMethod("__repr__", "__repr__() - datetime.datetime(...)", func(t time.Time) string {
		return fmt.Sprintf("datetime.datetime(%d, %d, %d, %s)", t.Year(), int(t.Month()), t.Day(), clockRepr(t, true))
	})
	am["__repr__"] = strMethod("__repr__", "__repr__() - datetime.date(...)", func(t time.Time) string {
		return fmt.Sprintf("datetime.date(%d, %d, %d)", t.Year(), int(t.Month()), t.Day())
	})
	tm["__repr__"] = strMethod("__repr__", "__repr__() - datetime.time(...)", func(t time.Time) string {
		return "datetime.time(" + clockRepr(t, true) + ")"
	})
	tm["__str__"] = strMethod("__str__", "__str__() - HH:MM:SS[.ffffff][+HH:MM]", func(t time.Time) string { return pyTimeString(t, "auto") })
	tdm["__repr__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			total, err := timedeltaTotal(args, 0)
			if err != nil {
				return err
			}
			return object.NewString(timedeltaReprString(total))
		},
		HelpText: "__repr__() - datetime.timedelta(...)",
	}
	tdm["__bool__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			total, err := timedeltaTotal(args, 0)
			if err != nil {
				return err
			}
			return object.NewBoolean(math.Round(total*1e6) != 0)
		},
		HelpText: "__bool__() - False for a zero duration",
	}
	tdm["__floordiv__"] = timedeltaFloorMod(false)
	tdm["__mod__"] = timedeltaFloorMod(true)

	dm["__format__"] = formatMethod(func(t time.Time) string { return pyDatetimeString(t, " ") })
	am["__format__"] = formatMethod(func(t time.Time) string { return t.Format("2006-01-02") })
	tm["__format__"] = formatMethod(func(t time.Time) string { return pyTimeString(t, "auto") })
	dm["ctime"] = strMethod("ctime", "ctime() - e.g. 'Mon May  6 07:08:09 2024'", ctimeString)
	am["ctime"] = strMethod("ctime", "ctime() - e.g. 'Mon May  6 00:00:00 2024'", ctimeString)
	dm["isoformat"] = isoformatMethod(true)
	tm["isoformat"] = isoformatMethod(false)
	tm["strftime"] = DatetimeClass.Methods["strftime"]
	dm["dst"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if _, err := selfTime("dst", args, kwargs); err != nil {
				return err
			}
			// Naive datetimes and fixed-offset zones have no DST: None.
			return &object.Null{}
		},
		HelpText: "dst() - None: naive datetimes and fixed-offset zones have no daylight saving",
	}
	dm["time"] = timeMethod("time() - The time of day, naive", func(t time.Time) object.Object {
		return createTimeInstance(timeOfDay(t, nil))
	})
	dm["timetz"] = timeMethod("timetz() - The time of day with the tzinfo", func(t time.Time) object.Object {
		var loc *time.Location
		if _, aware := awareKey(t.Location()); aware {
			loc = t.Location()
		}
		return createTimeInstance(timeOfDay(t, loc))
	})
	dm["timestamp"] = timeMethod("timestamp() - POSIX timestamp, with microseconds", func(t time.Time) object.Object {
		return object.NewFloat(float64(t.UnixMicro()) / 1e6)
	})
	for _, name := range []string{"__lt__", "__gt__", "__le__", "__ge__", "__eq__", "__ne__", "__hash__"} {
		tm[name] = DatetimeClass.Methods[name]
	}
	tm["replace"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			t, err := datetimeSelf(args)
			if err != nil {
				return err
			}
			names := []string{"hour", "minute", "second", "microsecond", "tzinfo"}
			vals, errObj := bindDatetimeArgs("replace", names, 0, args[1:], kwargs)
			if errObj != nil {
				return errObj
			}
			f := datetimeFields{1900, 1, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond() / 1000}
			if errObj := f.set(append([]object.Object{nil, nil, nil}, vals[:4]...)); errObj != nil {
				return errObj
			}
			loc := t.Location()
			if vals[4] != nil {
				tzLoc, err := tzinfoLocation(vals[4])
				if err != nil {
					return err
				}
				loc = time.UTC
				if tzLoc != nil {
					loc = tzLoc
				}
			}
			nt, errObj := f.time(loc)
			if errObj != nil {
				return errObj
			}
			return createTimeInstance(nt)
		},
		HelpText: "replace(hour, minute, second, microsecond, tzinfo) - Return the time with the given fields replaced",
	}

	// A datetime never equals a date, as in Python.
	baseEq := DatetimeClass.Methods["__eq__"].(*object.Builtin)
	sameKindEq := &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 2 {
				l, lok := args[0].(*object.Instance)
				r, rok := args[1].(*object.Instance)
				if lok && rok && l.Class != r.Class {
					return object.NewBoolean(false)
				}
			}
			return baseEq.Fn(ctx, kwargs, args...)
		},
		HelpText: baseEq.HelpText,
	}
	baseNe := DatetimeClass.Methods["__ne__"].(*object.Builtin)
	sameKindNe := &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 2 {
				l, lok := args[0].(*object.Instance)
				r, rok := args[1].(*object.Instance)
				if lok && rok && l.Class != r.Class {
					return object.NewBoolean(true)
				}
			}
			return baseNe.Fn(ctx, kwargs, args...)
		},
		HelpText: baseNe.HelpText,
	}
	for _, c := range []*object.Class{DatetimeClass, DateClass, TimeClass} {
		c.Methods["__eq__"] = sameKindEq
		c.Methods["__ne__"] = sameKindNe
	}

	datetimeConstructorBuiltin.Attributes["combine"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("combine", kwargs, "date", "time", "tzinfo"); err != nil {
					return err
				}
			}
			vals, errObj := bindDatetimeArgs("combine", []string{"date", "time", "tzinfo"}, 2, args, kwargs)
			if errObj != nil {
				return errObj
			}
			d, err := GetTimeFromObject(vals[0])
			if err != nil {
				return err
			}
			tmInst, ok := vals[1].(*object.Instance)
			if !ok || tmInst.Class != TimeClass {
				return errors.NewTypeErrorTagged("combine() argument 2 must be datetime.time, not %s", object.PythonTypeName(vals[1]))
			}
			clock, err := getTimeFromInstance(tmInst)
			if err != nil {
				return err
			}
			loc := time.Local
			if _, aware := awareKey(clock.Location()); aware {
				loc = clock.Location()
			}
			if vals[2] != nil {
				tzLoc, err := tzinfoLocation(vals[2])
				if err != nil {
					return err
				}
				loc = time.Local
				if tzLoc != nil {
					loc = tzLoc
				}
			}
			return createDatetimeInstance(time.Date(d.Year(), d.Month(), d.Day(), clock.Hour(), clock.Minute(), clock.Second(), clock.Nanosecond(), loc))
		},
		HelpText: "combine(date, time, tzinfo=time.tzinfo) - The datetime of a date and a time",
	}
	datetimeConstructorBuiltin.Attributes["min"] = createDatetimeInstance(time.Date(1, 1, 1, 0, 0, 0, 0, time.Local))
	datetimeConstructorBuiltin.Attributes["max"] = createDatetimeInstance(time.Date(9999, 12, 31, 23, 59, 59, 999999000, time.Local))
	dateConstructorBuiltin.Attributes["min"] = createDateInstance(time.Date(1, 1, 1, 0, 0, 0, 0, time.Local))
	dateConstructorBuiltin.Attributes["max"] = createDateInstance(time.Date(9999, 12, 31, 0, 0, 0, 0, time.Local))
	timeConstructorBuiltin.Attributes["min"] = createTimeInstance(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC))
	timeConstructorBuiltin.Attributes["max"] = createTimeInstance(time.Date(1900, 1, 1, 23, 59, 59, 999999000, time.UTC))
	if timedeltaBuiltinNew.Attributes == nil {
		timedeltaBuiltinNew.Attributes = map[string]object.Object{}
	}
	timedeltaBuiltinNew.Attributes["resolution"] = createTimedeltaInstance(1e-6)
}

// timedeltaFloorMod builds timedelta // x and timedelta % x: a timedelta
// divisor gives an int quotient or a timedelta remainder; an int divisor
// (floor division only) gives a timedelta.
func timedeltaFloorMod(mod bool) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			l, err := timedeltaTotal(args, 0)
			if err != nil {
				return err
			}
			if len(args) < 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			lu := int64(math.Round(l * 1e6))
			if n, ok := args[1].(*object.Integer); ok && !mod {
				if n.IntValue() == 0 {
					return errors.NewZeroDivisionError()
				}
				return createTimedeltaInstance(float64(floorDiv(lu, n.IntValue())) / 1e6)
			}
			r, rerr := timedeltaTotal(args, 1)
			if rerr != nil {
				op := "//"
				if mod {
					op = "%"
				}
				return errors.NewTypeErrorTagged("unsupported operand type(s) for %s: 'datetime.timedelta' and '%s'", op, object.PythonTypeName(args[1]))
			}
			ru := int64(math.Round(r * 1e6))
			if ru == 0 {
				return errors.NewZeroDivisionError()
			}
			q := floorDiv(lu, ru)
			if mod {
				return createTimedeltaInstance(float64(lu-q*ru) / 1e6)
			}
			return object.NewInteger(q)
		},
		HelpText: "timedelta // timedelta (an int), timedelta // int, timedelta % timedelta",
	}
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}
