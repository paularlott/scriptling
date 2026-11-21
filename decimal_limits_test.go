package scriptling

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/stdlib"
)

// The decimal module runs untrusted scripts in sandboxes, and math/big's
// memory is invisible to the interpreter's accounting: these attacks must
// each fail fast with a catchable error or stay bounded.

// newDecimalSandbox is an interpreter with sandbox-like limits: a 64 MB
// memory limit and a 1 MB single-value limit.
func newDecimalSandbox(t *testing.T) *Scriptling {
	t.Helper()
	p := New(WithMemoryLimit(64<<20), WithMaxValueBytes(1<<20))
	stdlib.RegisterAll(p)
	return p
}

// evalDecimal runs script with a timeout, failing the test when it errors,
// runs past the timeout, or holds more than the memory limit.
func evalDecimal(t *testing.T, p *Scriptling, timeout time.Duration, script string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	start := time.Now()
	result, err := p.EvalWithContext(ctx, script)
	if err != nil {
		t.Fatalf("script failed after %v: %v", time.Since(start), err)
	}
	if peak := p.ResourceUsage().PeakMemoryBytes; peak > 64<<20 {
		t.Fatalf("peak memory %d above the 64 MB limit", peak)
	}
	return result.Inspect()
}

func TestDecimalHugeStringIsRefused(t *testing.T) {
	p := New(WithMemoryLimit(256<<20), WithMaxValueBytes(8<<20))
	stdlib.RegisterAll(p)
	got := evalDecimal(t, p, 10*time.Second, `
import decimal
s = "9" * 5_000_000
try:
    decimal.Decimal(s)
    r = "accepted"
except MemoryError as e:
    r = "MemoryError"
ok = decimal.Decimal("9" * 1000) + 1
[r, len(str(ok)), str(ok)[:5]]
`)
	if got != "['MemoryError', 35, '1.000']" {
		t.Fatalf("got %s", got)
	}
}

func TestDecimalHugePowerOverflows(t *testing.T) {
	p := newDecimalSandbox(t)
	start := time.Now()
	got := evalDecimal(t, p, 5*time.Second, `
import decimal
out = []
for base, exp in [(10, 10**9), (2, 10**18), ("1.0000001", 10**15), ("0.5", -(10**9))]:
    try:
        decimal.Decimal(base) ** exp
        out.append("no error")
    except decimal.Overflow as e:
        out.append(str(e))
small = decimal.Decimal("0.5") ** (10**9)
try:
    decimal.Decimal(10) ** 10**9
except ArithmeticError:
    out.append("ArithmeticError")
[out, str(small), str(decimal.Decimal("1.0001") ** 1000000)]
`)
	want := "[[\"[<class 'decimal.Overflow'>]\", \"[<class 'decimal.Overflow'>]\", \"[<class 'decimal.Overflow'>]\", \"[<class 'decimal.Overflow'>]\", 'ArithmeticError'], '0E-1000026', '2.674710993142140172948354482E+43'"
	if got != want+"]" {
		t.Fatalf("got %s", got)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("powers took %v", d)
	}
}

func TestDecimalPrecisionIsCapped(t *testing.T) {
	p := newDecimalSandbox(t)
	got := evalDecimal(t, p, 10*time.Second, `
import decimal
c = decimal.getcontext()
errs = []
for prec in [10**9, decimal.MAX_PREC + 1, 0]:
    try:
        c.prec = prec
        errs.append("accepted")
    except ValueError as e:
        errs.append(str(e))
c.prec = decimal.MAX_PREC
third = decimal.Decimal(1) / decimal.Decimal(3)
[errs, decimal.MAX_PREC, c.prec, len(str(third))]
`)
	want := "[['valid range for prec is [1, MAX_PREC]', 'valid range for prec is [1, MAX_PREC]', 'valid range for prec is [1, MAX_PREC]'], 10000, 10000, 10002]"
	if got != want {
		t.Fatalf("got %s", got)
	}
}

func TestDecimalSquaringLoopStaysBounded(t *testing.T) {
	p := newDecimalSandbox(t)
	// Squaring grows until Overflow; shrinking squares bottom out at the
	// smallest subnormal; both in a handful of iterations' worth of time.
	got := evalDecimal(t, p, 10*time.Second, `
import decimal
x = decimal.Decimal("1.0000001")
grew = 0
try:
    for i in range(10000):
        x = x * x
        grew += 1
except decimal.Overflow:
    pass
y = decimal.Decimal("0.9999999")
for i in range(10000):
    y = y * y
decimal.getcontext().prec = decimal.MAX_PREC
z = decimal.Decimal(2) / decimal.Decimal(3)
for i in range(100):
    z = z * z + decimal.Decimal(1) / decimal.Decimal(7)
[grew, str(y), len(str(z)) <= 10010]
`)
	if got != "[44, '0E-1000026', True]" {
		t.Fatalf("got %s", got)
	}
}

func TestDecimalLoopObservesTimeout(t *testing.T) {
	p := newDecimalSandbox(t)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := p.EvalWithContext(ctx, `
import decimal
decimal.getcontext().prec = decimal.MAX_PREC
z = decimal.Decimal(2) / decimal.Decimal(3)
while True:
    z = (z * z) ** 3 / decimal.Decimal(7)
`)
	if err == nil {
		t.Fatal("endless decimal loop finished")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("timeout honoured only after %v", d)
	}
}

func TestDecimalCoefficientsAreCharged(t *testing.T) {
	p := New(WithMemoryLimit(32<<20), WithMaxValueBytes(0))
	stdlib.RegisterAll(p)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Each value holds a 10,000-digit coefficient (about 4 KB of math/big
	// memory the interpreter cannot otherwise see); 100,000 of them would
	// be 400 MB.
	_, err := p.EvalWithContext(ctx, `
import decimal
decimal.getcontext().prec = decimal.MAX_PREC
seventh = decimal.Decimal(1) / decimal.Decimal(7)
held = []
for i in range(100000):
    held.append(seventh + i)
`)
	if err == nil {
		t.Fatal("held 100,000 10,000-digit decimals under a 32 MB limit")
	}
	if !strings.Contains(err.Error(), ErrMemoryLimitExceeded.Error()) && !strings.Contains(err.Error(), "MemoryError") {
		t.Fatalf("expected the memory limit to stop the script: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("the caller's timeout expired; the memory limit did not stop the script")
	}
}

func TestDecimalHugeExponentInputsStayCheap(t *testing.T) {
	p := newDecimalSandbox(t)
	start := time.Now()
	got := evalDecimal(t, p, 5*time.Second, `
import decimal, math
D = decimal.Decimal
big, tiny = D("7E+999999999999"), D("3E-999999999999")
out = [str(big), str(tiny), big > tiny, tiny < 1, hash(big) == hash(D("70E+999999999998")), int(tiny), math.floor(tiny), round(tiny)]
for f in [lambda: int(big), lambda: big + 1, lambda: big.quantize(D("0.01")), lambda: format(big, "f"),
          lambda: format(tiny, "f"), lambda: format(D("1.5"), ".100000000f"), lambda: tiny.quantize(D("1E-999999"))]:
    try:
        f()
        out.append("ok")
    except Exception as e:
        out.append(type(e).__name__)
out.append(str(tiny + 1))
out
`)
	want := "['7E+999999999999', '3E-999999999999', True, True, True, 0, 0, 0, 'OverflowError', 'Overflow', 'InvalidOperation', 'MemoryError', 'MemoryError', 'MemoryError', 'ok', '1.000000000000000000000000000']"
	if got != want {
		t.Fatalf("got %s", got)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("took %v", d)
	}
}
