package object

import (
	"fmt"
	"math"
	"time"
)

// Timeout argument rules shared by every library, so a timeout means the same
// wherever a script passes one: a number of seconds, int or float (0.5 is half
// a second). Anything else is a TypeError and an out-of-range number a
// ValueError, never a silent fall back to the default.
type TimeoutRule uint8

const (
	// TimeoutAllowNone accepts None for no time limit.
	TimeoutAllowNone TimeoutRule = 1 << iota
	// TimeoutAllowZero accepts 0, whose meaning the function documents.
	TimeoutAllowZero
)

// maxTimeoutSeconds keeps a timeout well inside time.Duration's range.
const maxTimeoutSeconds = 1 << 32

// ParseTimeout converts v, the timeout argument called name, to a duration.
// With TimeoutAllowNone, None gives none = true and a zero duration.
func ParseTimeout(name string, v Object, rule TimeoutRule) (d time.Duration, none bool, errObj Object) {
	var secs float64
	switch t := v.(type) {
	case *Null:
		if rule&TimeoutAllowNone != 0 {
			return 0, true, nil
		}
		return 0, false, timeoutTypeError(name, v, rule)
	case *Integer:
		secs = float64(t.value)
	case *Float:
		secs = t.value
	default:
		return 0, false, timeoutTypeError(name, v, rule)
	}
	if math.IsNaN(secs) || secs < 0 || secs > maxTimeoutSeconds || (secs == 0 && rule&TimeoutAllowZero == 0) {
		bound := "greater than 0"
		if rule&TimeoutAllowZero != 0 {
			bound = "0 or more"
		}
		return 0, false, &Error{
			Message:       fmt.Sprintf("%s must be a number of seconds %s, got %v", name, bound, v.Inspect()),
			ExceptionType: ExceptionTypeValueError,
		}
	}
	return time.Duration(secs * float64(time.Second)), false, nil
}

func timeoutTypeError(name string, v Object, rule TimeoutRule) Object {
	want := "an int or float"
	if rule&TimeoutAllowNone != 0 {
		want = "an int, float or None"
	}
	return &Error{
		Message:       fmt.Sprintf("%s must be %s, not %s", name, want, PythonTypeName(v)),
		ExceptionType: ExceptionTypeTypeError,
	}
}

// GetTimeout reads the timeout keyword called name with ParseTimeout, giving
// def when the keyword is absent.
func (k Kwargs) GetTimeout(name string, def time.Duration, rule TimeoutRule) (d time.Duration, none bool, errObj Object) {
	v, ok := k.Lookup(name)
	if !ok {
		return def, false, nil
	}
	return ParseTimeout(name, v, rule)
}
