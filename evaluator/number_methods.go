package evaluator

import (
	"math"
	"math/big"
	"math/bits"
	"strconv"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// callIntegerMethod dispatches int methods: bit_length (Python 3.1),
// bit_count (3.10) and is_integer (3.12).
func callIntegerMethod(i *object.Integer, method string, args []object.Object) object.Object {
	switch method {
	case "bit_length":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		v := i.IntValue()
		if v < 0 {
			v = -v
		}
		// uint64(-MinInt64) is 2^63 with bit length 64, matching Python.
		return object.NewInteger(int64(bits.Len64(uint64(v))))
	case "bit_count":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		v := i.IntValue()
		if v < 0 {
			v = -v
		}
		return object.NewInteger(int64(bits.OnesCount64(uint64(v))))
	case "is_integer":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		return TRUE
	}
	return attributeError(i, method)
}

// callFloatMethod dispatches float methods: is_integer, hex, fromhex and
// as_integer_ratio.
func callFloatMethod(f *object.Float, method string, args []object.Object) object.Object {
	switch method {
	case "is_integer":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		v := f.FloatValue()
		return nativeBoolToBooleanObject(!math.IsInf(v, 0) && !math.IsNaN(v) && v == math.Trunc(v))
	case "hex":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		return object.NewString(floatHex(f.FloatValue()))
	case "fromhex":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		s, err := args[0].AsString()
		if err != nil {
			return err
		}
		v, ok := parseFloatHex(s)
		if !ok {
			return &object.Exception{
				Message:       "could not convert string to float: '" + s + "'",
				ExceptionType: object.ExceptionTypeValueError,
				Raised:        true,
			}
		}
		return object.NewFloat(v)
	case "as_integer_ratio":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		v := f.FloatValue()
		if math.IsInf(v, 0) {
			return &object.Exception{
				Message:       "cannot convert Infinity to integer ratio",
				ExceptionType: object.ExceptionTypeOverflowError,
				Raised:        true,
			}
		}
		if math.IsNaN(v) {
			return &object.Exception{
				Message:       "cannot convert NaN to integer ratio",
				ExceptionType: object.ExceptionTypeOverflowError,
				Raised:        true,
			}
		}
		// big.Float carries the float's exact binary value; converting via
		// the shortest decimal repr would give 1/10 for 0.1 instead of
		// 3602879701896397/36028797018963968. SetFloat64 is exact, so the
		// conversion below is too.
		r := new(big.Rat)
		new(big.Float).SetFloat64(v).Rat(r)
		if !r.Num().IsInt64() || !r.Denom().IsInt64() {
			return &object.Exception{
				Message:       "integer ratio does not fit in int64",
				ExceptionType: object.ExceptionTypeOverflowError,
				Raised:        true,
			}
		}
		return &object.List{Elements: []object.Object{
			object.NewInteger(r.Num().Int64()),
			object.NewInteger(r.Denom().Int64()),
		}}
	}
	return attributeError(f, method)
}

// floatHex renders f in Python's float.hex() form: a full 13-hex-digit
// mantissa and an unpadded binary exponent, e.g. "0x1.0000000000000p+1".
func floatHex(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0x0.0p+0"
		}
		return "0x0.0p+0"
	}
	s := strconv.FormatFloat(f, 'x', 13, 64)
	// Go zero-pads the binary exponent ("p+01"); Python does not ("p+1").
	i := strings.IndexByte(s, 'p')
	if i < 0 {
		return s
	}
	exp := s[i+1:]
	sign := "+"
	if len(exp) > 0 && (exp[0] == '+' || exp[0] == '-') {
		sign, exp = string(exp[0]), exp[1:]
	}
	exp = strings.TrimLeft(exp, "0")
	if exp == "" {
		exp = "0"
	}
	return s[:i] + "p" + sign + exp
}

// parseFloatHex parses Python's float.fromhex() forms: optional sign, the
// 0x prefix, an optional p-exponent (defaulting to p+0), and the inf/nan
// keywords. Go's ParseFloat accepts the 0x…p… shape once the exponent is
// present.
func parseFloatHex(s string) (float64, bool) {
	t := strings.TrimSpace(strings.ToLower(s))
	sign := ""
	if t == "" {
		return 0, false
	}
	switch t[0] {
	case '+':
		sign, t = "+", t[1:]
	case '-':
		sign, t = "-", t[1:]
	}
	switch t {
	case "inf", "infinity":
		if sign == "-" {
			return math.Inf(-1), true
		}
		return math.Inf(1), true
	case "nan":
		return math.NaN(), true
	}
	if !strings.HasPrefix(t, "0x") {
		return 0, false
	}
	if !strings.Contains(t, "p") {
		t += "p0"
	}
	v, err := strconv.ParseFloat(sign+t, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
