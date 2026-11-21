package stdlib

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"weak"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/libutil"
	"github.com/paularlott/scriptling/object"
)

// Python's decimal module for money and other exact decimal arithmetic. The
// arithmetic is a port of CPython's _pydecimal for finite values: a value is
// sign, coefficient and exponent; every operation computes exactly and rounds
// once to the context, so results (and exponents: 1.10 + 2.20 is 3.30) match
// Python digit for digit. NaN and Infinity are not supported (they raise
// InvalidOperation), nor are sqrt, exp, ln, log10 or non-integral powers.
//
// Scripts may be untrusted, and math/big memory is invisible to the
// interpreter's accounting, so every value is bounded: results are rounded
// to at most decMaxPrec digits with exponents inside Python's Emin/Emax
// (beyond is decimal.Overflow), strings may hold at most decMaxDigits digits
// (checked against the value limit too), formatting checks its output size,
// and coefficients are charged to the interpreter's memory.

type decimal struct {
	neg   bool
	coeff *big.Int // >= 0
	exp   int64
}

type decContext struct {
	prec     int
	rounding string
}

const (
	decMaxPrec   = 10000  // getcontext().prec cap; Python's is 999999999999999999
	decMaxDigits = 100000 // digits a Decimal string may hold
	decEmax      = 999999 // Python's default context's exponent limits
	decEmin      = -999999
)

var defaultDecContext = decContext{28, "ROUND_HALF_EVEN"}

var decRoundings = []string{"ROUND_DOWN", "ROUND_HALF_UP", "ROUND_HALF_EVEN", "ROUND_CEILING",
	"ROUND_FLOOR", "ROUND_UP", "ROUND_HALF_DOWN", "ROUND_05UP"}

// decSignal is a trapped condition: the exception and the condition Python
// names in the message ("[<class 'decimal.DivisionUndefined'>]").
type decSignal struct{ class, condition string }

var (
	sigInvalid    = &decSignal{"InvalidOperation", "InvalidOperation"}
	sigConversion = &decSignal{"InvalidOperation", "ConversionSyntax"}
	sigUndefined  = &decSignal{"InvalidOperation", "DivisionUndefined"}
	sigImpossible = &decSignal{"InvalidOperation", "DivisionImpossible"}
	sigDivByZero  = &decSignal{"DivisionByZero", "DivisionByZero"}
	sigMemory     = &decSignal{"MemoryError", ""}
	sigOverflow   = &decSignal{"Overflow", "Overflow"}
)

var bigTen = big.NewInt(10)

var pow10Cache = func() []*big.Int {
	c := []*big.Int{big.NewInt(1)}
	for i := 1; i < 64; i++ {
		c = append(c, new(big.Int).Mul(c[i-1], bigTen))
	}
	return c
}()

// pow10 is 10**n; the result must not be modified.
func pow10(n int64) *big.Int {
	if n < int64(len(pow10Cache)) {
		return pow10Cache[n]
	}
	return new(big.Int).Exp(bigTen, big.NewInt(n), nil)
}

// numDigits is the number of decimal digits of x >= 0 (1 for zero).
func numDigits(x *big.Int) int64 {
	if x.IsUint64() {
		n, u := int64(1), x.Uint64()
		for ; u >= 10; u /= 10 {
			n++
		}
		return n
	}
	n := int64(float64(x.BitLen()) * math.Log10(2))
	if x.Cmp(pow10(n)) >= 0 {
		n++
	}
	if n > 1 && x.Cmp(pow10(n-1)) < 0 {
		n--
	}
	return n
}

func newDec(neg bool, coeff *big.Int, exp int64) *decimal { return &decimal{neg, coeff, exp} }

func decFromInt64(n int64) *decimal {
	c := big.NewInt(n)
	return newDec(n < 0, c.Abs(c), 0)
}

func (d *decimal) isZero() bool     { return d.coeff.Sign() == 0 }
func (d *decimal) adjusted() int64  { return d.exp + numDigits(d.coeff) - 1 }
func (d *decimal) negate() *decimal { return newDec(!d.neg, d.coeff, d.exp) }
func (d *decimal) abs() *decimal    { return newDec(false, d.coeff, d.exp) }

// parseDecimal reads Python's decimal string syntax for finite values.
func parseDecimal(s string) (*decimal, bool) {
	s = strings.TrimSpace(s)
	for i := 0; i < len(s); i++ { // underscores only between digits
		if s[i] == '_' && (i == 0 || i == len(s)-1 || !isDigitByte(s[i-1]) || !isDigitByte(s[i+1])) {
			return nil, false
		}
	}
	s = strings.ReplaceAll(s, "_", "")
	d := &decimal{}
	if s != "" && (s[0] == '+' || s[0] == '-') {
		d.neg, s = s[0] == '-', s[1:]
	}
	mant, expPart, hasExp := strings.Cut(strings.ToLower(s), "e")
	intPart, frac, _ := strings.Cut(mant, ".")
	digits := intPart + frac
	if digits == "" || (hasExp && expPart == "") {
		return nil, false
	}
	for i := 0; i < len(digits); i++ {
		if !isDigitByte(digits[i]) {
			return nil, false
		}
	}
	if hasExp {
		e, err := strconv.ParseInt(expPart, 10, 64)
		if err != nil || e > math.MaxInt64/4 || e < math.MinInt64/4 {
			return nil, false
		}
		d.exp = e
	}
	d.coeff, _ = new(big.Int).SetString(digits, 10)
	d.exp -= int64(len(frac))
	return d, true
}

func isDigitByte(c byte) bool { return c >= '0' && c <= '9' }

// decFromFloat is the float's exact value: n/2**k is n*5**k / 10**k.
func decFromFloat(f float64) (*decimal, *decSignal) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, sigInvalid
	}
	r := new(big.Rat).SetFloat64(math.Abs(f))
	k := int64(r.Denom().BitLen() - 1)
	c := new(big.Int).Mul(r.Num(), new(big.Int).Exp(big.NewInt(5), big.NewInt(k), nil))
	return newDec(math.Signbit(f), c, -k), nil
}

// String is Python's str(Decimal): plain notation unless the exponent is
// positive or the value is very small.
func (d *decimal) String() string {
	sign := ""
	if d.neg {
		sign = "-"
	}
	digits := d.coeff.String()
	n := int64(len(digits))
	left := d.exp + n
	dot := int64(1)
	if d.exp <= 0 && left > -6 {
		dot = left
	}
	var intpart, frac string
	switch {
	case dot <= 0:
		intpart, frac = "0", "."+strings.Repeat("0", int(-dot))+digits
	case dot >= n:
		intpart = digits + strings.Repeat("0", int(dot-n))
	default:
		intpart, frac = digits[:dot], "."+digits[dot:]
	}
	if left != dot {
		frac += "E" + fmtSigned(left-dot)
	}
	return sign + intpart + frac
}

func fmtSigned(n int64) string {
	if n >= 0 {
		return "+" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

// dropDigits removes coeff's last d digits, rounding by mode.
func dropDigits(coeff *big.Int, d int64, neg bool, mode string) *big.Int {
	if d <= 0 {
		return coeff
	}
	q, r := new(big.Int), new(big.Int)
	if d > numDigits(coeff) {
		// All dropped: only "something non-zero, under half" remains.
		r.SetInt64(int64(coeff.Sign()))
		d = 1
	} else {
		q.QuoRem(coeff, pow10(d), r)
	}
	if r.Sign() == 0 {
		return q
	}
	up := false
	switch mode {
	case "ROUND_UP":
		up = true
	case "ROUND_CEILING":
		up = !neg
	case "ROUND_FLOOR":
		up = neg
	case "ROUND_05UP":
		last := new(big.Int).Rem(q, bigTen).Int64()
		up = last == 0 || last == 5
	case "ROUND_HALF_UP", "ROUND_HALF_DOWN", "ROUND_HALF_EVEN":
		c := r.Cmp(new(big.Int).Mul(big.NewInt(5), pow10(d-1)))
		up = c > 0 || (c == 0 && (mode == "ROUND_HALF_UP" || (mode == "ROUND_HALF_EVEN" && q.Bit(0) == 1)))
	}
	if up {
		q.Add(q, big.NewInt(1))
	}
	return q
}

// fix rounds d to the context precision and clamps its exponent to the
// context's range, as _pydecimal's _fix does: an adjusted exponent above
// Emax is Overflow, one below Etiny rounds to a subnormal or zero.
func (d *decimal) fix(c decContext) (*decimal, *decSignal) {
	etiny, etop := decEmin-int64(c.prec)+1, decEmax-int64(c.prec)+1
	if d.isZero() {
		if e := min(max(d.exp, etiny), decEmax); e != d.exp {
			return newDec(d.neg, d.coeff, e), nil
		}
		return d, nil
	}
	expMin := max(numDigits(d.coeff)+d.exp-int64(c.prec), etiny)
	if expMin > etop {
		return nil, sigOverflow
	}
	if d.exp >= expMin {
		return d, nil
	}
	q := dropDigits(d.coeff, expMin-d.exp, d.neg, c.rounding)
	if numDigits(q) > int64(c.prec) { // rounded up to 10**prec
		q.Quo(q, bigTen)
		if expMin++; expMin > etop {
			return nil, sigOverflow
		}
	}
	return newDec(d.neg, q, expMin), nil
}

// rescale gives d exponent exp, rounding by mode.
func (d *decimal) rescale(exp int64, mode string) *decimal {
	if d.isZero() {
		return newDec(d.neg, new(big.Int), exp)
	}
	if d.exp >= exp {
		return newDec(d.neg, new(big.Int).Mul(d.coeff, pow10(d.exp-exp)), exp)
	}
	return newDec(d.neg, dropDigits(d.coeff, exp-d.exp, d.neg, mode), exp)
}

// roundSig rounds d to places significant digits.
func (d *decimal) roundSig(places int64, mode string) *decimal {
	if d.isZero() {
		return d
	}
	ans := d.rescale(d.adjusted()+1-places, mode)
	if ans.adjusted() != d.adjusted() {
		ans = ans.rescale(ans.adjusted()+1-places, mode)
	}
	return ans
}

func decAdd(a, b *decimal, c decContext) (*decimal, *decSignal) {
	exp := min(a.exp, b.exp)
	negZero := c.rounding == "ROUND_FLOOR" && a.neg != b.neg
	switch {
	case a.isZero() && b.isZero():
		return newDec(a.neg && b.neg || negZero, new(big.Int), exp).fix(c)
	case a.isZero():
		return b.rescale(max(exp, b.exp-int64(c.prec)-1), c.rounding).fix(c)
	case b.isZero():
		return a.rescale(max(exp, a.exp-int64(c.prec)-1), c.rounding).fix(c)
	}
	// Align the exponents; an operand far below the other's precision
	// stands in as a single sticky digit, so a huge gap costs nothing.
	hi, lo := *a, *b
	if hi.exp < lo.exp {
		hi, lo = lo, hi
	}
	e := hi.exp + min(-1, numDigits(hi.coeff)-int64(c.prec)-2)
	if numDigits(lo.coeff)+lo.exp-1 < e {
		lo.coeff, lo.exp = big.NewInt(1), e
	}
	hi.coeff, hi.exp = new(big.Int).Mul(hi.coeff, pow10(hi.exp-lo.exp)), lo.exp
	if hi.neg == lo.neg {
		return newDec(hi.neg, new(big.Int).Add(hi.coeff, lo.coeff), lo.exp).fix(c)
	}
	switch hi.coeff.Cmp(lo.coeff) {
	case 0:
		return newDec(negZero, new(big.Int), exp).fix(c)
	case -1:
		hi, lo = lo, hi
	}
	return newDec(hi.neg, new(big.Int).Sub(hi.coeff, lo.coeff), lo.exp).fix(c)
}

func decSub(a, b *decimal, c decContext) (*decimal, *decSignal) { return decAdd(a, b.negate(), c) }

func decMul(a, b *decimal, c decContext) (*decimal, *decSignal) {
	return newDec(a.neg != b.neg, new(big.Int).Mul(a.coeff, b.coeff), a.exp+b.exp).fix(c)
}

func decDiv(a, b *decimal, c decContext) (*decimal, *decSignal) {
	if b.isZero() {
		if a.isZero() {
			return nil, sigUndefined
		}
		return nil, sigDivByZero
	}
	if a.isZero() {
		return newDec(a.neg != b.neg, new(big.Int), a.exp-b.exp).fix(c)
	}
	shift := numDigits(b.coeff) - numDigits(a.coeff) + int64(c.prec) + 1
	exp := a.exp - b.exp - shift
	q, r := new(big.Int), new(big.Int)
	if shift >= 0 {
		q.QuoRem(new(big.Int).Mul(a.coeff, pow10(shift)), b.coeff, r)
	} else {
		q.QuoRem(a.coeff, new(big.Int).Mul(b.coeff, pow10(-shift)), r)
	}
	if r.Sign() != 0 {
		if new(big.Int).Rem(q, big.NewInt(5)).Sign() == 0 {
			q.Add(q, big.NewInt(1)) // keep the inexact result off a tie
		}
	} else {
		// Exact: as close to the ideal exponent as trailing zeros allow.
		for ideal := a.exp - b.exp; exp < ideal; exp++ {
			if new(big.Int).Rem(q, bigTen).Sign() != 0 {
				break
			}
			q.Quo(q, bigTen)
		}
	}
	return newDec(a.neg != b.neg, q, exp).fix(c)
}

// decDivmod is the truncated integer quotient and the remainder, which
// takes the dividend's sign.
func decDivmod(a, b *decimal, c decContext) (*decimal, *decimal, *decSignal) {
	if b.isZero() {
		if a.isZero() {
			return nil, nil, sigUndefined
		}
		return nil, nil, sigDivByZero
	}
	neg, ideal := a.neg != b.neg, min(a.exp, b.exp)
	expdiff := a.adjusted() - b.adjusted()
	if a.isZero() || expdiff <= -2 {
		r, sig := a.rescale(ideal, c.rounding).fix(c)
		return newDec(neg, new(big.Int), 0), r, sig
	}
	if expdiff <= int64(c.prec) {
		x, y := a.coeff, b.coeff
		if a.exp >= b.exp {
			x = new(big.Int).Mul(x, pow10(a.exp-b.exp))
		} else {
			y = new(big.Int).Mul(y, pow10(b.exp-a.exp))
		}
		q, r := new(big.Int).QuoRem(x, y, new(big.Int))
		if q.Cmp(pow10(int64(c.prec))) < 0 {
			rem, sig := newDec(a.neg, r, ideal).fix(c)
			return newDec(neg, q, 0), rem, sig
		}
	}
	return nil, nil, sigImpossible
}

func decFloorDiv(a, b *decimal, c decContext) (*decimal, *decSignal) {
	q, _, sig := decDivmod(a, b, c)
	return q, sig
}

func decMod(a, b *decimal, c decContext) (*decimal, *decSignal) {
	if b.isZero() && !a.isZero() {
		return nil, sigInvalid // x % 0
	}
	_, r, sig := decDivmod(a, b, c)
	return r, sig
}

// decPow is a ** b for an integral b.
func decPow(a, b *decimal, c decContext) (*decimal, *decSignal) {
	n, ok := decIntegral(b)
	if !ok {
		return nil, sigInvalid
	}
	m := n
	if m < 0 {
		m = -m
	}
	neg := a.neg && m%2 == 1
	switch {
	case a.isZero() && n == 0:
		return nil, sigInvalid
	case a.isZero() && n < 0:
		return nil, sigDivByZero
	case n == 0:
		return decFromInt64(1).fix(c)
	}
	if numDigits(a.coeff)*m <= 2*int64(c.prec)+20 && a.exp < 1<<40 && a.exp > -(1<<40) {
		// Small enough to compute exactly, then round once, as Python.
		p := newDec(neg, new(big.Int).Exp(a.coeff, big.NewInt(m), nil), a.exp*m)
		if n < 0 {
			return decDiv(decFromInt64(1), p, c)
		}
		return p.fix(c)
	}
	// Exponentiation by squaring, each step rounded (ROUND_05UP keeps the
	// final rounding correct) to guard digits past the precision, so no
	// intermediate outgrows the precision and Overflow stops a runaway
	// exponent early.
	wc := decContext{c.prec + 5 + len(strconv.FormatInt(m, 10)), "ROUND_05UP"}
	base := a.abs()
	var sig *decSignal
	if n < 0 {
		if base, sig = decDiv(decFromInt64(1), base, wc); sig != nil {
			return nil, sig
		}
	}
	acc := decFromInt64(1)
	for e := m; e > 0; e >>= 1 {
		if e&1 == 1 {
			if acc, sig = decMul(acc, base, wc); sig != nil {
				return nil, sig
			}
		}
		if e > 1 {
			if base, sig = decMul(base, base, wc); sig != nil {
				return nil, sig
			}
		}
	}
	return newDec(neg, acc.coeff, acc.exp).fix(c)
}

// decIntegral returns b as an int64 when it is an integer that fits.
func decIntegral(b *decimal) (int64, bool) {
	if b.exp > 18 || b.adjusted() > 18 || b.adjusted() < 0 {
		return 0, b.isZero()
	}
	r := b.toRat() // exponent within [-19, 18] here: small
	if !r.IsInt() || !r.Num().IsInt64() {
		return 0, false
	}
	return r.Num().Int64(), true
}

func decCmp(a, b *decimal) int {
	switch {
	case a.isZero() && b.isZero():
		return 0
	case a.isZero() || b.isZero() || a.neg != b.neg:
		return signOf(a) - signOf(b)
	}
	sign := 1
	if a.neg {
		sign = -1
	}
	if aa, ba := a.adjusted(), b.adjusted(); aa != ba {
		if aa < ba {
			return -sign
		}
		return sign
	}
	x, y := a.coeff, b.coeff
	if a.exp > b.exp {
		x = new(big.Int).Mul(x, pow10(a.exp-b.exp))
	} else if b.exp > a.exp {
		y = new(big.Int).Mul(y, pow10(b.exp-a.exp))
	}
	return sign * x.Cmp(y)
}

func signOf(d *decimal) int {
	switch {
	case d.isZero():
		return 0
	case d.neg:
		return -1
	}
	return 1
}

func decQuantize(a *decimal, exp int64, mode string, c decContext) (*decimal, *decSignal) {
	if exp < -999999-int64(c.prec)+1 || exp > 999999 {
		return nil, sigInvalid
	}
	if a.isZero() {
		return newDec(a.neg, new(big.Int), exp).fix(c)
	}
	if a.adjusted() > decEmax || a.adjusted()-exp+1 > int64(c.prec) {
		return nil, sigInvalid
	}
	ans := a.rescale(exp, mode)
	if numDigits(ans.coeff) > int64(c.prec) || ans.adjusted() > decEmax {
		return nil, sigInvalid
	}
	return ans.fix(c)
}

func decNormalize(a *decimal, c decContext) (*decimal, *decSignal) {
	d, sig := a.fix(c)
	if sig != nil {
		return nil, sig
	}
	if d.isZero() {
		return newDec(d.neg, new(big.Int), 0), nil
	}
	coeff, exp, r := new(big.Int).Set(d.coeff), d.exp, new(big.Int)
	for q := new(big.Int); exp < 999999; exp++ {
		if q.QuoRem(coeff, bigTen, r); r.Sign() != 0 {
			break
		}
		coeff.Set(q)
	}
	return newDec(d.neg, coeff, exp), nil
}

func (d *decimal) toRat() *big.Rat {
	r := new(big.Rat)
	if d.exp >= 0 {
		r.SetInt(new(big.Int).Mul(d.coeff, pow10(d.exp)))
	} else {
		r.SetFrac(d.coeff, pow10(-d.exp))
	}
	if d.neg {
		r.Neg(r)
	}
	return r
}

// toInt rounds to an integer by mode (ROUND_DOWN truncates); OverflowError
// past 64 bits. The size checks come first, so a huge exponent is never
// expanded.
func (d *decimal) toInt(mode string) object.Object {
	if d.isZero() || d.adjusted() < -1 {
		d = newDec(d.neg, dropDigits(d.coeff, numDigits(d.coeff)+1, d.neg, mode), 0) // |d| < 0.1: 0 or ±1
	}
	if d.adjusted() > 19 {
		return errors.NewOverflowError("int too large to convert to a 64-bit integer")
	}
	if d.exp < 0 {
		d = d.rescale(0, mode) // exponent >= -21 here
	}
	n := new(big.Int).Mul(d.coeff, pow10(d.exp))
	if d.neg {
		n.Neg(n)
	}
	if !n.IsInt64() {
		return errors.NewOverflowError("int too large to convert to a 64-bit integer")
	}
	return object.NewInteger(n.Int64())
}

func (d *decimal) float() float64 {
	f, _ := strconv.ParseFloat(d.coeff.String()+"e"+strconv.FormatInt(d.exp, 10), 64)
	if d.neg {
		return -f
	}
	return f
}

// decHash is the hash of an equal int or float, so they meet as dict keys
// and set members.
func decHash(d *decimal) int64 {
	if d.isZero() {
		return fnvKeyHash(object.DictKey(object.NewInteger(0)))
	}
	if adj := d.adjusted(); adj > 400 || adj < -400 || d.exp < -1200 {
		// No int or float equals it: hash the value with trailing zeros
		// stripped, so equal decimals agree, without expanding the exponent.
		coeff, exp := new(big.Int).Set(d.coeff), d.exp
		for q, r := new(big.Int), new(big.Int); ; exp++ {
			if q.QuoRem(coeff, bigTen, r); r.Sign() != 0 {
				break
			}
			coeff.Set(q)
		}
		return fnvKeyHash(fmt.Sprintf("d:%t:%se%d", d.neg, coeff, exp))
	}
	r := d.toRat()
	if r.IsInt() && r.Num().IsInt64() {
		return fnvKeyHash(object.DictKey(object.NewInteger(r.Num().Int64())))
	}
	if f, exact := r.Float64(); exact {
		return fnvKeyHash(object.DictKey(object.NewFloat(f)))
	}
	return fnvKeyHash("q:" + r.String()) // equal decimals: one canonical n/d
}

// fnvKeyHash is the builtin hash() of a value with the given dict key.
func fnvKeyHash(key string) int64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(key); i++ {
		h ^= uint64(key[i])
		h *= 1099511628211
	}
	return int64(h)
}

// ---- the interpreter's context ----

// decContexts holds each interpreter's getcontext(), keyed by its root
// environment; an interpreter that never asked for one uses the default.
var decContexts sync.Map // weak.Pointer[object.Environment] -> *object.Instance

// DecimalContextClass is the class of getcontext()'s result.
var DecimalContextClass = &object.Class{Name: "Context", Methods: map[string]object.Object{}}

func currentDecContext(ctx context.Context) decContext {
	if v, ok := decContexts.Load(weak.Make(libutil.EnvFromContext(ctx).Root())); ok {
		return *v.(*object.Instance).NativeData.(*decContext)
	}
	return defaultDecContext
}

func getDecContext(ctx context.Context) *object.Instance {
	root := libutil.EnvFromContext(ctx).Root()
	key := weak.Make(root)
	c := defaultDecContext
	v, loaded := decContexts.LoadOrStore(key, object.NewInstanceWithData(DecimalContextClass, map[string]object.Object{
		"prec": object.NewInteger(int64(c.prec)), "rounding": object.NewString(c.rounding)}, &c))
	if !loaded {
		runtime.AddCleanup(root, func(k weak.Pointer[object.Environment]) { decContexts.Delete(k) }, key)
	}
	return v.(*object.Instance)
}

// ---- Decimal values ----

// DecimalClass is decimal.Decimal.
var DecimalClass = &object.Class{Name: "Decimal", Methods: map[string]object.Object{}}

func newDecimalObject(d *decimal) object.Object {
	trackDecimal(d)
	return object.NewInstanceWithData(DecimalClass, nil, d)
}

// trackDecimal charges a coefficient's memory to the interpreter, which
// cannot see math/big's allocations.
func trackDecimal(d *decimal) {
	if words := len(d.coeff.Bits()); words > 2 {
		object.TrackAlloc(d, int64(words)*8+48)
	}
}

func decimalOf(o object.Object) (*decimal, bool) {
	if inst, ok := o.(*object.Instance); ok {
		d, ok := inst.NativeData.(*decimal)
		return d, ok
	}
	return nil, false
}

func args0Instance(args []object.Object) (*object.Instance, bool) {
	if len(args) == 0 {
		return nil, false
	}
	inst, ok := args[0].(*object.Instance)
	return inst, ok
}

// decResult returns d, or raises the signal as Python's default context
// traps it.
func decResult(d *decimal, sig *decSignal) object.Object {
	switch {
	case sig == sigMemory:
		return object.NewMemoryError("number too large")
	case sig == sigOverflow: // not in the evaluator's hierarchy: name its base
		return &object.Exception{Message: "[<class 'decimal.Overflow'>]", ExceptionType: "Overflow", TypeChain: []string{"Overflow", "ArithmeticError"}, Raised: true}
	case sig != nil:
		return &object.Exception{Message: "[<class 'decimal." + sig.condition + "'>]", ExceptionType: sig.class, Raised: true}
	}
	return newDecimalObject(d)
}

// decOperand converts an arithmetic operand: a Decimal, int or bool.
func decOperand(o object.Object) (*decimal, bool) {
	switch v := o.(type) {
	case *object.Integer:
		return decFromInt64(v.IntValue()), true
	case *object.Boolean:
		if v.BoolValue() {
			return decFromInt64(1), true
		}
		return decFromInt64(0), true
	}
	return decimalOf(o)
}

func roundingArg(o object.Object, c decContext) (string, object.Object) {
	if o == nil || o.Type() == object.NULL_OBJ {
		return c.rounding, nil
	}
	if s, ok := o.(*object.String); ok {
		for _, r := range decRoundings {
			if r == s.StringValue() {
				return r, nil
			}
		}
	}
	return "", errors.NewTypeErrorTagged("valid values for rounding are: [ROUND_CEILING, ROUND_FLOOR, ROUND_UP, ROUND_DOWN, ROUND_HALF_UP, ROUND_HALF_DOWN, ROUND_HALF_EVEN, ROUND_05UP]")
}

// decMethod builds a Decimal method taking the named optional arguments.
func decMethod(name, help string, params []string, fn func(ctx context.Context, d *decimal, args []object.Object) object.Object) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs(name, kwargs, params...); err != nil {
					return err
				}
			}
			d, ok := decimalOf(firstArg(args))
			if !ok {
				return errors.NewTypeErrorTagged("descriptor '%s' requires a 'decimal.Decimal' object", name)
			}
			vals, err := kwargsToPositional(name, args[1:], kwargs, params...)
			if err != nil {
				return err
			}
			if len(vals) > len(params) {
				return errors.NewTypeErrorTagged("%s() takes at most %d arguments (%d given)", name, len(params), len(vals))
			}
			return fn(ctx, d, append(vals, make([]object.Object, len(params)-len(vals))...))
		},
		HelpText: help,
	}
}

func firstArg(args []object.Object) object.Object {
	if len(args) == 0 {
		return nil
	}
	return args[0]
}

var decOps = map[string]func(a, b *decimal, c decContext) (*decimal, *decSignal){
	"+": decAdd, "-": decSub, "*": decMul, "/": decDiv, "//": decFloorDiv, "%": decMod, "**": decPow,
}

// decBinary builds an arithmetic dunder; reflected swaps the operands.
func decBinary(name, sym string, reflected bool) *object.Builtin {
	return decMethod(name, name+"(other) - self "+sym+" other", []string{"other"}, func(ctx context.Context, d *decimal, args []object.Object) object.Object {
		o, ok := decOperand(args[0])
		if !ok {
			l, r := "decimal.Decimal", object.PythonTypeName(args[0])
			if reflected {
				l, r = r, l
			}
			if sym == "**" {
				sym = "** or pow()"
			}
			return errors.NewTypeErrorTagged("unsupported operand type(s) for %s: '%s' and '%s'", sym, l, r)
		}
		if reflected {
			d, o = o, d
		}
		return decResult(decOps[sym](d, o, currentDecContext(ctx)))
	})
}

// decCompare compares d with an int, float (exactly) or Decimal.
func decCompare(d *decimal, o object.Object) (int, bool) {
	if f, ok := o.(*object.Float); ok {
		if math.IsInf(f.FloatValue(), 0) {
			return -int(math.Copysign(1, f.FloatValue())), true
		}
		x, sig := decFromFloat(f.FloatValue())
		if sig != nil {
			return 0, false // NaN: unordered
		}
		return decCmp(d, x), true
	}
	if x, ok := decOperand(o); ok {
		return decCmp(d, x), true
	}
	return 0, false
}

func decOrder(name, sym string, keep func(int) bool) *object.Builtin {
	return decMethod(name, name+"(other) - self "+sym+" other", []string{"other"}, func(ctx context.Context, d *decimal, args []object.Object) object.Object {
		cmp, ok := decCompare(d, args[0])
		if !ok {
			if f, isF := args[0].(*object.Float); isF && math.IsNaN(f.FloatValue()) {
				return decResult(nil, sigInvalid)
			}
			return errors.NewTypeErrorTagged("'%s' not supported between instances of 'decimal.Decimal' and '%s'", sym, object.PythonTypeName(args[0]))
		}
		return object.NewBoolean(keep(cmp))
	})
}

func init() {
	m := DecimalClass.Methods
	noArgs := []string{}
	m["__init__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("Decimal", kwargs, "value", "context"); err != nil {
					return err
				}
			}
			inst, ok := args0Instance(args)
			if !ok {
				return errors.NewTypeErrorTagged("Decimal.__init__ requires self")
			}
			vals, err := kwargsToPositional("Decimal", args[1:], kwargs, "value", "context")
			if err != nil {
				return err
			}
			if len(vals) > 2 {
				return errors.NewTypeErrorTagged("Decimal() takes at most 2 arguments (%d given)", len(vals))
			}
			d, ok := decFromInt64(0), true
			if len(vals) > 0 {
				switch v := vals[0].(type) {
				case *object.String:
					// Bound the digits before parsing: math/big's parse is
					// superlinear and its memory unaccounted.
					if n := len(v.StringValue()); n > decMaxDigits+100 {
						return object.NewMemoryError("Decimal string of %d characters is more than the %d-digit limit", n, decMaxDigits)
					} else if err := object.CheckAllocFor("decimal", int64(n)/2); n > 1000 && err != nil {
						return err
					}
					if d, ok = parseDecimal(v.StringValue()); !ok {
						return decResult(nil, sigConversion)
					}
					trackDecimal(d)
				case *object.Float:
					var sig *decSignal
					if d, sig = decFromFloat(v.FloatValue()); sig != nil {
						return decResult(nil, sig)
					}
				case *object.List, *object.Tuple:
					return errors.NewValueError("argument must be a sequence of length 3")
				default:
					if d, ok = decOperand(v); !ok {
						return errors.NewTypeErrorTagged("conversion from %s to Decimal is not supported", object.PythonTypeName(v))
					}
				}
			}
			inst.NativeData = d
			return &object.Null{}
		},
		HelpText: `Decimal(value='0') - An exact decimal number

value is a string ('1.10', '-2.5E+3', '1_000'), an int, a float (converted
exactly: Decimal(0.1) shows the binary float's true value) or a Decimal.`,
	}
	simple := map[string]struct {
		help string
		fn   func(ctx context.Context, d *decimal) object.Object
	}{
		"__str__": {"__str__() - The decimal's string form", func(_ context.Context, d *decimal) object.Object { return object.NewString(d.String()) }},
		"__repr__": {"__repr__() - Decimal('...')", func(_ context.Context, d *decimal) object.Object {
			return object.NewString("Decimal('" + d.String() + "')")
		}},
		"__hash__": {"__hash__() - Hash equal to that of an equal int or float", func(_ context.Context, d *decimal) object.Object { return object.NewInteger(decHash(d)) }},
		"__bool__": {"__bool__() - False for zero", func(_ context.Context, d *decimal) object.Object { return object.NewBoolean(!d.isZero()) }},
		"__neg__": {"__neg__() - The negation, rounded to the context", func(ctx context.Context, d *decimal) object.Object {
			if d.isZero() {
				return decResult(d.abs().fix(currentDecContext(ctx)))
			}
			return decResult(d.negate().fix(currentDecContext(ctx)))
		}},
		"__pos__": {"__pos__() - The value rounded to the context", func(ctx context.Context, d *decimal) object.Object {
			if d.isZero() {
				d = d.abs()
			}
			return decResult(d.fix(currentDecContext(ctx)))
		}},
		"__abs__": {"__abs__() - The absolute value, rounded to the context", func(ctx context.Context, d *decimal) object.Object {
			return decResult(d.abs().fix(currentDecContext(ctx)))
		}},
		"__int__":     {"__int__() - Truncate to an int", func(_ context.Context, d *decimal) object.Object { return d.toInt("ROUND_DOWN") }},
		"__trunc__":   {"__trunc__() - Truncate to an int", func(_ context.Context, d *decimal) object.Object { return d.toInt("ROUND_DOWN") }},
		"__floor__":   {"__floor__() - The largest int <= self", func(_ context.Context, d *decimal) object.Object { return d.toInt("ROUND_FLOOR") }},
		"__ceil__":    {"__ceil__() - The smallest int >= self", func(_ context.Context, d *decimal) object.Object { return d.toInt("ROUND_CEILING") }},
		"__float__":   {"__float__() - The nearest float", func(_ context.Context, d *decimal) object.Object { return object.NewFloat(d.float()) }},
		"__copy__":    {"__copy__() - The decimal itself: decimals are immutable", nil},
		"is_zero":     {"is_zero() - True for zero", func(_ context.Context, d *decimal) object.Object { return object.NewBoolean(d.isZero()) }},
		"is_signed":   {"is_signed() - True when negative (including -0)", func(_ context.Context, d *decimal) object.Object { return object.NewBoolean(d.neg) }},
		"adjusted":    {"adjusted() - The exponent of the most significant digit", func(_ context.Context, d *decimal) object.Object { return object.NewInteger(d.adjusted()) }},
		"copy_abs":    {"copy_abs() - The absolute value, unrounded", func(_ context.Context, d *decimal) object.Object { return newDecimalObject(d.abs()) }},
		"copy_negate": {"copy_negate() - The negation, unrounded", func(_ context.Context, d *decimal) object.Object { return newDecimalObject(d.negate()) }},
		"normalize": {"normalize() - The value with trailing zeros stripped", func(ctx context.Context, d *decimal) object.Object {
			return decResult(decNormalize(d, currentDecContext(ctx)))
		}},
	}
	for name, s := range simple {
		fn := s.fn
		m[name] = decMethod(name, s.help, noArgs, func(ctx context.Context, d *decimal, _ []object.Object) object.Object {
			if fn == nil {
				return newDecimalObject(d)
			}
			return fn(ctx, d)
		})
	}
	m["__deepcopy__"] = decMethod("__deepcopy__", "__deepcopy__(memo) - The decimal itself: decimals are immutable", []string{"memo"}, func(_ context.Context, d *decimal, _ []object.Object) object.Object {
		return newDecimalObject(d)
	})
	m["__format__"] = decMethod("__format__", "__format__(spec) - Format with the format mini-language", []string{"spec"}, func(ctx context.Context, d *decimal, args []object.Object) object.Object {
		spec, ok := args[0].(*object.String)
		if !ok {
			return errors.NewTypeErrorTagged("format spec must be a str")
		}
		return formatDecimal(d, spec.StringValue(), currentDecContext(ctx))
	})
	equal := func(d *decimal, o object.Object) bool {
		cmp, ok := decCompare(d, o)
		return ok && cmp == 0
	}
	m["__eq__"] = decMethod("__eq__", "__eq__(other) - Numeric equality", []string{"other"}, func(_ context.Context, d *decimal, args []object.Object) object.Object {
		return object.NewBoolean(equal(d, args[0]))
	})
	m["__ne__"] = decMethod("__ne__", "__ne__(other) - Numeric inequality", []string{"other"}, func(_ context.Context, d *decimal, args []object.Object) object.Object {
		return object.NewBoolean(!equal(d, args[0]))
	})
	m["__lt__"] = decOrder("__lt__", "<", func(c int) bool { return c < 0 })
	m["__le__"] = decOrder("__le__", "<=", func(c int) bool { return c <= 0 })
	m["__gt__"] = decOrder("__gt__", ">", func(c int) bool { return c > 0 })
	m["__ge__"] = decOrder("__ge__", ">=", func(c int) bool { return c >= 0 })
	for name, sym := range map[string]string{"add": "+", "sub": "-", "mul": "*", "truediv": "/", "floordiv": "//", "mod": "%", "pow": "**"} {
		m["__"+name+"__"] = decBinary("__"+name+"__", sym, false)
		m["__r"+name+"__"] = decBinary("__r"+name+"__", sym, true)
	}
	for _, reflected := range []bool{false, true} {
		name := map[bool]string{false: "__divmod__", true: "__rdivmod__"}[reflected]
		m[name] = decMethod(name, name+"(other) - (a // b, a % b)", []string{"other"}, func(ctx context.Context, d *decimal, args []object.Object) object.Object {
			o, ok := decOperand(args[0])
			if !ok {
				return errors.NewTypeErrorTagged("unsupported operand type(s) for divmod(): 'decimal.Decimal' and '%s'", object.PythonTypeName(args[0]))
			}
			if reflected {
				d, o = o, d
			}
			q, r, sig := decDivmod(d, o, currentDecContext(ctx))
			if sig != nil {
				return decResult(nil, sig)
			}
			return &object.Tuple{Elements: []object.Object{newDecimalObject(q), newDecimalObject(r)}}
		})
	}
	m["__round__"] = decMethod("__round__", "__round__([n]) - round(d) is an int (ties to even); round(d, n) a Decimal with n places", []string{"ndigits"}, func(ctx context.Context, d *decimal, args []object.Object) object.Object {
		if args[0] == nil || args[0].Type() == object.NULL_OBJ {
			return d.toInt("ROUND_HALF_EVEN")
		}
		n, ok := args[0].(*object.Integer)
		if !ok {
			return errors.NewTypeErrorTagged("Second argument to round should be integral")
		}
		c := currentDecContext(ctx)
		return decResult(decQuantize(d, -n.IntValue(), c.rounding, c))
	})
	m["quantize"] = decMethod("quantize", `quantize(exp, rounding=None) - Round to the exponent of exp

Decimal('7.325').quantize(Decimal('0.01'), rounding=ROUND_HALF_UP) is
Decimal('7.33'); rounding defaults to the context's (ROUND_HALF_EVEN).`, []string{"exp", "rounding", "context"}, func(ctx context.Context, d *decimal, args []object.Object) object.Object {
		e, ok := decOperand(args[0])
		if !ok {
			return errors.NewTypeErrorTagged("quantize() argument 'exp' must be a Decimal or int")
		}
		c := currentDecContext(ctx)
		mode, err := roundingArg(args[1], c)
		if err != nil {
			return err
		}
		return decResult(decQuantize(d, e.exp, mode, c))
	})
	toIntegral := decMethod("to_integral_value", "to_integral_value(rounding=None) - Round to an integral value, keeping it a Decimal", []string{"rounding", "context"}, func(ctx context.Context, d *decimal, args []object.Object) object.Object {
		mode, err := roundingArg(args[0], currentDecContext(ctx))
		if err != nil {
			return err
		}
		if d.exp >= 0 {
			return newDecimalObject(d)
		}
		return newDecimalObject(d.rescale(0, mode))
	})
	m["to_integral_value"], m["to_integral"] = toIntegral, toIntegral

	DecimalContextClass.Methods["__setattr__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("__setattr__", kwargs); err != nil {
				return err
			}
			inst, _ := args0Instance(args)
			if inst == nil || len(args) != 3 {
				return errors.NewTypeErrorTagged("__setattr__ needs a context, a name and a value")
			}
			c, _ := inst.NativeData.(*decContext)
			name, _ := args[1].(*object.String)
			switch {
			case c == nil || name == nil:
				return errors.NewTypeErrorTagged("__setattr__ needs a context, a name and a value")
			case name.StringValue() == "prec":
				p, ok := args[2].(*object.Integer)
				if !ok {
					return errors.NewTypeErrorTagged("'%s' object cannot be interpreted as an integer", object.PythonTypeName(args[2]))
				}
				if p.IntValue() < 1 || p.IntValue() > decMaxPrec {
					return errors.NewValueError("valid range for prec is [1, MAX_PREC]")
				}
				c.prec = int(p.IntValue())
			case name.StringValue() == "rounding":
				r, err := roundingArg(args[2], *c)
				if err != nil || args[2].Type() == object.NULL_OBJ {
					return errors.NewTypeErrorTagged("rounding must be one of the ROUND_* constants")
				}
				c.rounding = r
			default:
				return errors.NewAttributeErrorTagged("'decimal.Context' object has no attribute '%s'", name.StringValue())
			}
			inst.SetField(name.StringValue(), args[2])
			return &object.Null{}
		},
		HelpText: "Set prec (1 to decimal.MAX_PREC, 10000) or rounding (a ROUND_* constant)",
	}
}

func decExceptionClass(name string) *object.Builtin {
	return object.RegisterExceptionClass(&object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs(name, kwargs); err != nil {
				return err
			}
			msg := ""
			if len(args) > 0 {
				msg = args[0].Inspect()
				if s, ok := args[0].(*object.String); ok {
					msg = s.StringValue()
				}
			}
			ex := &object.Exception{Message: msg, ExceptionType: name, Args: args}
			if name == "Overflow" {
				ex.TypeChain = []string{"Overflow", "ArithmeticError"}
			}
			return ex
		},
		HelpText: name + "([message]) - A trapped decimal arithmetic condition",
		Repr:     "<class 'decimal." + name + "'>",
	}, name)
}

// DecimalLibrary is Python's decimal module.
var DecimalLibrary = object.NewLibrary(DecimalLibraryName, map[string]*object.Builtin{
	"getcontext": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("getcontext", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			return getDecContext(ctx)
		},
		HelpText: `getcontext() - This interpreter's decimal context

getcontext().prec (default 28) and getcontext().rounding (default
ROUND_HALF_EVEN) are readable and settable.`,
	},
}, func() map[string]object.Object {
	consts := map[string]object.Object{
		"Decimal":          DecimalClass,
		"InvalidOperation": decExceptionClass("InvalidOperation"),
		"DivisionByZero":   decExceptionClass("DivisionByZero"),
		"Overflow":         decExceptionClass("Overflow"),
		"MAX_PREC":         object.NewInteger(decMaxPrec),
	}
	for _, r := range decRoundings {
		consts[r] = object.NewString(r)
	}
	return consts
}(), "Exact decimal arithmetic (Decimal, getcontext, ROUND_* modes)")

// formatDecimal is Decimal.__format__, ported from _pydecimal.
func formatDecimal(d *decimal, specStr string, c decContext) object.Object {
	spec, err := parseNumberSpec(specStr)
	if err != nil {
		return err
	}
	switch {
	case spec.typ == 0:
		spec.typ = 'G' // Python's context has capitals=1
	case spec.typ == 'n':
		spec.typ = 'g'
	case !strings.ContainsRune("eEfFgG%", rune(spec.typ)):
		return errors.NewValueError("Invalid format specifier '%s' for object of type 'decimal.Decimal'", specStr)
	}
	if spec.typ == '%' {
		d = newDec(d.neg, d.coeff, d.exp+2)
	}
	// Check the output's size first: 'f' writes out the exponent's zeros
	// and the precision's digits, and the width pads.
	size := numDigits(d.coeff) + 30 + int64(spec.width)
	if p := int64(spec.precision); p >= 0 {
		size += p
	}
	if strings.ContainsRune("fF%", rune(spec.typ)) {
		size += max(d.exp, -d.exp, 0)
	}
	if size > 1<<20 {
		if err := object.CheckAllocFor("formatted decimal", size); err != nil {
			return err
		}
	}
	if p := int64(spec.precision); p >= 0 {
		switch spec.typ {
		case 'e', 'E':
			d = d.roundSig(p+1, c.rounding)
		case 'f', 'F', '%':
			d = d.rescale(-p, c.rounding)
		default:
			if numDigits(d.coeff) > max(p, 1) {
				d = d.roundSig(max(p, 1), c.rounding)
			}
		}
	}
	if d.isZero() && d.exp > 0 && strings.ContainsRune("fF%", rune(spec.typ)) {
		d = d.rescale(0, c.rounding)
	}
	digits := d.coeff.String()
	n := int64(len(digits))
	left := d.exp + n
	dot := left
	switch spec.typ {
	case 'e', 'E':
		dot = 1
		if d.isZero() && spec.precision >= 0 {
			dot = 1 - int64(spec.precision)
		}
	case 'g', 'G':
		if d.exp > 0 || left <= -6 {
			dot = 1
		}
	}
	intpart, frac := "0", ""
	switch {
	case dot < 0:
		frac = strings.Repeat("0", int(-dot)) + digits
	case dot > n:
		intpart = digits + strings.Repeat("0", int(dot-n))
	case dot > 0:
		intpart, frac = digits[:dot], digits[dot:]
	default:
		frac = digits
	}
	if frac != "" || spec.alt {
		frac = "." + frac
	}
	if exp := left - dot; exp != 0 || spec.typ == 'e' || spec.typ == 'E' {
		frac += map[bool]string{true: "E", false: "e"}[spec.typ == 'E' || spec.typ == 'G'] + fmtSigned(exp)
	}
	if spec.typ == '%' {
		frac += "%"
	}
	sign := spec.sign(d.neg && !(d.isZero() && spec.noNegZero))
	minWidth := 0
	if spec.zeroPad {
		minWidth = spec.width - len(frac) - len(sign)
	}
	return object.NewString(spec.align(sign, insertThousands(intpart, spec.thousands, minWidth)+frac))
}

// numberSpec is a parsed format spec: [[fill]align][sign][z][#][0][width][,|_][.precision][type].
type numberSpec struct {
	fill                    rune
	alignCh, signCh, typ    byte
	noNegZero, alt, zeroPad bool
	width, precision        int
	thousands               string
}

func parseNumberSpec(s string) (*numberSpec, object.Object) {
	spec := &numberSpec{fill: ' ', alignCh: '>', precision: -1}
	bad := errors.NewValueError("Invalid format specifier '%s'", s)
	r := []rune(s)
	i, explicit := 0, false
	isAlign := func(c rune) bool { return strings.ContainsRune("<>^=", c) }
	if len(r) >= 2 && isAlign(r[1]) {
		spec.fill, spec.alignCh, i, explicit = r[0], byte(r[1]), 2, true
	} else if len(r) >= 1 && isAlign(r[0]) {
		spec.alignCh, i, explicit = byte(r[0]), 1, true
	}
	flag := func(c rune) bool {
		if i < len(r) && r[i] == c {
			i++
			return true
		}
		return false
	}
	for _, c := range "+- " {
		if flag(c) {
			spec.signCh = byte(c)
			break
		}
	}
	spec.noNegZero, spec.alt = flag('z'), flag('#')
	if flag('0') && !explicit {
		spec.zeroPad, spec.fill, spec.alignCh = true, '0', '='
	}
	num := func() (int, bool) {
		start := i
		for i < len(r) && r[i] >= '0' && r[i] <= '9' {
			i++
		}
		n, err := strconv.Atoi(string(r[start:i]))
		return n, err == nil
	}
	if w, ok := num(); ok {
		spec.width = w
	}
	if flag(',') {
		spec.thousands = ","
	} else if flag('_') {
		spec.thousands = "_"
	}
	if flag('.') {
		p, ok := num()
		if !ok {
			return nil, errors.NewValueError("Format specifier missing precision")
		}
		spec.precision = p
	}
	if i < len(r) && r[i] < 128 {
		spec.typ, i = byte(r[i]), i+1
	}
	if i != len(r) {
		return nil, bad
	}
	return spec, nil
}

func (s *numberSpec) sign(neg bool) string {
	switch {
	case neg:
		return "-"
	case s.signCh == '+' || s.signCh == ' ':
		return string(s.signCh)
	}
	return ""
}

// align pads sign+body to the width.
func (s *numberSpec) align(sign, body string) string {
	pad := []rune(strings.Repeat(string(s.fill), max(0, s.width-len([]rune(sign+body)))))
	switch s.alignCh {
	case '<':
		return sign + body + string(pad)
	case '^':
		return string(pad[:len(pad)/2]) + sign + body + string(pad[len(pad)/2:])
	case '=':
		return sign + string(pad) + body
	}
	return string(pad) + sign + body
}

// insertThousands groups digits by three with sep, zero-padding to minWidth.
func insertThousands(digits, sep string, minWidth int) string {
	if sep == "" {
		return strings.Repeat("0", max(0, minWidth-len(digits))) + digits
	}
	var groups []string
	for {
		l := min(max(len(digits), minWidth, 1), 3)
		g := strings.Repeat("0", max(0, l-len(digits))) + digits[max(0, len(digits)-l):]
		groups = append([]string{g}, groups...)
		digits = digits[:max(0, len(digits)-l)]
		if minWidth -= l; digits == "" && minWidth <= 0 {
			break
		}
		minWidth -= len(sep)
	}
	return strings.Join(groups, sep)
}
