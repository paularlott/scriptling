package object

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestParseTimeout(t *testing.T) {
	cases := []struct {
		v      Object
		rule   TimeoutRule
		want   time.Duration
		none   bool
		errHas string
	}{
		{NewInteger(5), 0, 5 * time.Second, false, ""},
		{&Float{value: 0.5}, 0, 500 * time.Millisecond, false, ""},
		{&Float{value: 90.0}, 0, 90 * time.Second, false, ""},
		{&Null{}, TimeoutAllowNone, 0, true, ""},
		{&Null{}, 0, 0, false, "must be an int or float, not NoneType"},
		{NewString("10"), TimeoutAllowNone, 0, false, "must be an int, float or None, not str"},
		{NewBoolean(true), 0, 0, false, "not bool"},
		{NewInteger(0), 0, 0, false, "greater than 0"},
		{NewInteger(0), TimeoutAllowZero, 0, false, ""},
		{NewInteger(-1), TimeoutAllowZero, 0, false, "0 or more"},
		{&Float{value: math.NaN()}, 0, 0, false, "greater than 0"},
		{&Float{value: math.Inf(1)}, 0, 0, false, "greater than 0"},
	}
	for _, c := range cases {
		d, none, err := ParseTimeout("timeout", c.v, c.rule)
		if c.errHas != "" {
			if err == nil || !strings.Contains(err.Inspect(), c.errHas) {
				t.Errorf("%s: want error containing %q, got %v", c.v.Inspect(), c.errHas, err)
			}
			continue
		}
		if err != nil || d != c.want || none != c.none {
			t.Errorf("%s: got %v, %v, %v", c.v.Inspect(), d, none, err)
		}
	}
}
