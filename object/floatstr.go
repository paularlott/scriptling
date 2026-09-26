package object

import (
	"math"
	"strconv"
	"strings"
)

// FloatStr formats f the way Python's str() and repr() format floats:
// shortest round-trip digits, positional notation while 1e-4 <= |f| < 1e16,
// scientific notation outside that range with a two-digit exponent, ".0"
// appended to integral values, and "inf"/"nan" spellings. Go's 'g' format
// disagrees on all of these ("2" vs "2.0", "1.23e+08" vs "123456789.123").
func FloatStr(f float64) string {
	if math.IsNaN(f) {
		return "nan"
	}
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	sign := ""
	if math.Signbit(f) {
		sign = "-"
		f = -f
	}
	if f == 0 {
		return sign + "0.0"
	}

	// Shortest decimal digits that round-trip, as "d.dddde±dd".
	sci := strconv.FormatFloat(f, 'e', -1, 64)
	e := strings.IndexByte(sci, 'e')
	mantissa, exp := sci[:e], atoiSimple(sci[e+1:])
	digits := strings.Replace(mantissa, ".", "", 1)

	if exp < -4 || exp >= 16 {
		m := digits[:1]
		if len(digits) > 1 {
			m += "." + digits[1:]
		}
		e := exp
		esign := "+"
		if e < 0 {
			esign = "-"
			e = -e
		}
		es := strconv.Itoa(e)
		if len(es) < 2 {
			es = "0" + es
		}
		return sign + m + "e" + esign + es
	}
	if exp < 0 {
		return sign + "0." + strings.Repeat("0", -exp-1) + digits
	}
	if len(digits) > exp+1 {
		return sign + digits[:exp+1] + "." + digits[exp+1:]
	}
	return sign + digits + strings.Repeat("0", exp+1-len(digits)) + ".0"
}

func atoiSimple(s string) int {
	neg := false
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		neg = s[0] == '-'
		s = s[1:]
	}
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	if neg {
		return -n
	}
	return n
}
