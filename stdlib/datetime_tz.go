package stdlib

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// datetime.timezone: fixed-offset time zones, and aware datetimes.
//
// An aware datetime's time.Time carries one of the *time.Location values
// below, one per (offset, name), so createDatetimeInstance can tell an aware
// time from a naive one (naive times use time.Local, or time.UTC for the
// naive UTC values utcnow and strptime return) and give it its tzinfo.

type tzKey struct {
	offset int
	name   string // "" when the timezone was given no name
}

var (
	tzMu        sync.Mutex
	tzLocations = map[tzKey]*time.Location{}
	tzByLoc     = map[*time.Location]tzKey{}
)

// tzLocation returns the shared location for a fixed offset and name.
func tzLocation(k tzKey) *time.Location {
	tzMu.Lock()
	defer tzMu.Unlock()
	if loc, ok := tzLocations[k]; ok {
		return loc
	}
	loc := time.FixedZone(tzDisplayName(k), k.offset)
	tzLocations[k] = loc
	tzByLoc[loc] = k
	return loc
}

// awareKey reports the timezone of an aware time's location.
func awareKey(loc *time.Location) (tzKey, bool) {
	tzMu.Lock()
	defer tzMu.Unlock()
	k, ok := tzByLoc[loc]
	return k, ok
}

// tzDisplayName is Python's tzname(): the given name, "UTC" for a zero
// offset, else "UTC+HH:MM".
func tzDisplayName(k tzKey) string {
	if k.name != "" {
		return k.name
	}
	if k.offset == 0 {
		return "UTC"
	}
	return "UTC" + formatUTCOffset(k.offset, true)
}

// formatUTCOffset renders an offset as +HH:MM (with colon) or +HHMM, with
// :SS when the offset has seconds.
func formatUTCOffset(offset int, colon bool) string {
	sign := '+'
	if offset < 0 {
		sign = '-'
		offset = -offset
	}
	h, m, s := offset/3600, (offset%3600)/60, offset%60
	sep := ""
	if colon {
		sep = ":"
	}
	out := fmt.Sprintf("%c%02d%s%02d", sign, h, sep, m)
	if s != 0 {
		out += fmt.Sprintf("%s%02d", sep, s)
	}
	return out
}

// TimezoneClass is the class of datetime.timezone values.
var TimezoneClass = &object.Class{Name: "timezone", Methods: map[string]object.Object{}}

// utcTimezone is datetime.timezone.utc (and datetime.UTC).
var utcTimezone = newTimezoneInstance(tzKey{})

func newTimezoneInstance(k tzKey) *object.Instance {
	var name object.Object = &object.Null{}
	if k.name != "" {
		name = object.NewString(k.name)
	}
	return object.NewInstanceWithFields(TimezoneClass, map[string]object.Object{
		"_offset": object.NewInteger(int64(k.offset)),
		"_name":   name,
	})
}

// timezoneKey reads a timezone instance; ok is false for anything else.
func timezoneKey(o object.Object) (tzKey, bool) {
	inst, ok := o.(*object.Instance)
	if !ok || inst.Class != TimezoneClass {
		return tzKey{}, false
	}
	off, _ := inst.Field("_offset").(*object.Integer)
	if off == nil {
		return tzKey{}, false
	}
	k := tzKey{offset: int(off.IntValue())}
	if n, ok := inst.Field("_name").(*object.String); ok {
		k.name = n.StringValue()
	}
	return k, true
}

// tzinfoLocation converts a tzinfo argument: None is naive (ok, nil), a
// timezone its location; anything else is a TypeError.
func tzinfoLocation(o object.Object) (*time.Location, object.Object) {
	if o == nil || o.Type() == object.NULL_OBJ {
		return nil, nil
	}
	k, ok := timezoneKey(o)
	if !ok {
		return nil, errors.NewTypeErrorTagged("tzinfo argument must be None or of a tzinfo subclass, not type '%s'", object.PythonTypeName(o))
	}
	return tzLocation(k), nil
}

// tzinfoOf is the tzinfo attribute for a datetime's time.
func tzinfoOf(t time.Time) object.Object {
	k, ok := awareKey(t.Location())
	if !ok {
		return &object.Null{}
	}
	if k == (tzKey{}) {
		return utcTimezone
	}
	return newTimezoneInstance(k)
}

func timezoneSelf(args []object.Object) (tzKey, object.Object) {
	if len(args) < 1 {
		return tzKey{}, errors.NewTypeErrorTagged("timezone method requires self")
	}
	k, ok := timezoneKey(args[0])
	if !ok {
		return tzKey{}, errors.NewTypeError("timezone instance", args[0].Type().String())
	}
	return k, nil
}

// datetimeSelf reads the time of a datetime method's receiver.
func datetimeSelf(args []object.Object) (time.Time, object.Object) {
	if len(args) < 1 {
		return time.Time{}, errors.NewTypeErrorTagged("datetime method requires self")
	}
	inst, ok := args[0].(*object.Instance)
	if !ok {
		return time.Time{}, errors.NewTypeError("datetime instance", args[0].Type().String())
	}
	return getTimeFromInstance(inst)
}

// This init runs after datetime.go's (files initialise in name order), so
// DatetimeClass exists.
func init() {
	dm := DatetimeClass.Methods
	dm["astimezone"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("astimezone", kwargs, "tz"); err != nil {
					return err
				}
			}
			t, err := datetimeSelf(args)
			if err != nil {
				return err
			}
			tz := kwargs.Get("tz")
			if len(args) > 1 {
				tz = args[1]
			}
			loc, err := tzinfoLocation(tz)
			if err != nil {
				return err
			}
			if loc == nil {
				// The local zone, as a fixed offset.
				name, off := t.In(time.Local).Zone()
				loc = tzLocation(tzKey{offset: off, name: name})
			}
			return createDatetimeInstance(t.In(loc))
		},
		HelpText: "astimezone(tz=None) - The same instant in tz (the local zone when omitted), aware",
	}
	dm["utcoffset"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("utcoffset", kwargs); err != nil {
				return err
			}
			t, err := datetimeSelf(args)
			if err != nil {
				return err
			}
			if _, aware := awareKey(t.Location()); !aware {
				return &object.Null{}
			}
			_, off := t.Zone()
			return createTimedeltaInstance(float64(off))
		},
		HelpText: "utcoffset() - The offset from UTC as a timedelta, None when naive",
	}
	dm["tzname"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("tzname", kwargs); err != nil {
				return err
			}
			t, err := datetimeSelf(args)
			if err != nil {
				return err
			}
			k, aware := awareKey(t.Location())
			if !aware {
				return &object.Null{}
			}
			return object.NewString(tzDisplayName(k))
		},
		HelpText: "tzname() - The zone's name, None when naive",
	}

	m := TimezoneClass.Methods
	m["utcoffset"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("utcoffset", kwargs); err != nil {
				return err
			}
			k, err := timezoneSelf(args)
			if err != nil {
				return err
			}
			return createTimedeltaInstance(float64(k.offset))
		},
		HelpText: "utcoffset(dt) - The fixed offset from UTC, as a timedelta",
	}
	m["tzname"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("tzname", kwargs); err != nil {
				return err
			}
			k, err := timezoneSelf(args)
			if err != nil {
				return err
			}
			return object.NewString(tzDisplayName(k))
		},
		HelpText: "tzname(dt) - The zone's name: the one given, or UTC / UTC+HH:MM",
	}
	m["dst"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("dst", kwargs); err != nil {
				return err
			}
			return &object.Null{}
		},
		HelpText: "dst(dt) - None: a fixed-offset zone has no daylight saving",
	}
	m["__str__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("__str__", kwargs); err != nil {
				return err
			}
			k, err := timezoneSelf(args)
			if err != nil {
				return err
			}
			return object.NewString(tzDisplayName(k))
		},
		HelpText: "__str__() - The zone's name",
	}
	m["__repr__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("__repr__", kwargs); err != nil {
				return err
			}
			k, err := timezoneSelf(args)
			if err != nil {
				return err
			}
			if k == (tzKey{}) {
				return object.NewString("datetime.timezone.utc")
			}
			s := fmt.Sprintf("datetime.timezone(%s)", timedeltaRepr(k.offset))
			if k.name != "" {
				s = fmt.Sprintf("datetime.timezone(%s, %s)", timedeltaRepr(k.offset), object.ReprString(k.name))
			}
			return object.NewString(s)
		},
		HelpText: "__repr__() - datetime.timezone(...) as Python shows it",
	}
	m["__eq__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("__eq__", kwargs); err != nil {
				return err
			}
			k, err := timezoneSelf(args)
			if err != nil {
				return err
			}
			if len(args) < 2 {
				return object.NewBoolean(false)
			}
			o, ok := timezoneKey(args[1])
			// Python compares fixed-offset zones by offset only.
			return object.NewBoolean(ok && o.offset == k.offset)
		},
		HelpText: "__eq__(other) - Zones with the same offset are equal",
	}
	m["__hash__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("__hash__", kwargs); err != nil {
				return err
			}
			k, err := timezoneSelf(args)
			if err != nil {
				return err
			}
			return object.NewInteger(int64(k.offset))
		},
		HelpText: "__hash__() - Hash of the offset",
	}
}

// timedeltaRepr is repr(timedelta(seconds=n)) for whole seconds.
func timedeltaRepr(seconds int) string {
	days := seconds / 86400
	rem := seconds % 86400
	if rem < 0 {
		rem += 86400
		days--
	}
	switch {
	case days != 0 && rem != 0:
		return fmt.Sprintf("datetime.timedelta(days=%d, seconds=%d)", days, rem)
	case days != 0:
		return fmt.Sprintf("datetime.timedelta(days=%d)", days)
	}
	return fmt.Sprintf("datetime.timedelta(seconds=%d)", rem)
}

// timezoneConstructorBuiltin is datetime.timezone(offset, name=None).
var timezoneConstructorBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		var offsetObj, nameObj object.Object
		if len(args) > 0 {
			offsetObj = args[0]
		}
		if len(args) > 1 {
			nameObj = args[1]
		}
		if len(args) > 2 {
			return errors.NewTypeErrorTagged("timezone() takes at most 2 arguments (%d given)", len(args))
		}
		for key, v := range kwargs.All() {
			switch key {
			case "offset":
				offsetObj = v
			case "name":
				nameObj = v
			default:
				return errors.NewTypeErrorTagged("timezone() got an unexpected keyword argument '%s'", key)
			}
		}
		inst, ok := offsetObj.(*object.Instance)
		var total *object.Float
		if ok {
			total, _ = inst.Field("_total").(*object.Float)
		}
		if total == nil {
			return errors.NewTypeErrorTagged("timezone() argument 1 must be datetime.timedelta")
		}
		secs := total.FloatValue()
		if secs <= -86400 || secs >= 86400 {
			return errors.NewValueError("offset must be a timedelta strictly between -timedelta(hours=24) and timedelta(hours=24), not %s", timedeltaRepr(int(secs)))
		}
		k := tzKey{offset: int(secs)}
		if nameObj != nil && nameObj.Type() != object.NULL_OBJ {
			s, ok := nameObj.(*object.String)
			if !ok {
				return errors.NewTypeErrorTagged("timezone() argument 2 must be str, not %s", object.PythonTypeName(nameObj))
			}
			k.name = s.StringValue()
		}
		if k == (tzKey{}) {
			return utcTimezone
		}
		return newTimezoneInstance(k)
	},
	HelpText: `timezone(offset, name=None) - A fixed offset from UTC

offset is a timedelta strictly between -24 and 24 hours. timezone.utc is UTC.

  from datetime import datetime, timezone, timedelta
  datetime.now(timezone.utc).isoformat()          # '...+00:00'
  ist = timezone(timedelta(hours=5, minutes=30))
  datetime(2024, 1, 1, 12, tzinfo=ist).isoformat() # '2024-01-01T12:00:00+05:30'`,
	Attributes: map[string]object.Object{
		"utc": utcTimezone,
	},
}

// pyDatetimeString renders a datetime as Python's str() (sep " ") and
// isoformat() (sep "T") do: microseconds when non-zero, the UTC offset when
// aware.
func pyDatetimeString(t time.Time, sep string) string {
	s := t.Format("2006-01-02") + sep + t.Format("15:04:05")
	if us := t.Nanosecond() / 1000; us != 0 {
		s += fmt.Sprintf(".%06d", us)
	}
	if _, aware := awareKey(t.Location()); aware {
		_, off := t.Zone()
		s += formatUTCOffset(off, true)
	}
	return s
}
