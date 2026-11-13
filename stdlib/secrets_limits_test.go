package stdlib_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

// limitsTestLimit is the value limit used by TestLimitsSecrets: small
// enough that a few KB of data goes over it.
const limitsTestLimit = 1024

// limitsNewInterp returns an interpreter with the stdlib registered and the
// given value limit (0 = no limit).
func limitsNewInterp(t *testing.T, maxValue int64) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New(scriptling.WithMaxValueBytes(maxValue))
	stdlib.RegisterAll(p)
	return p
}

// limitsCatch runs setup followed by expr inside a try/except MemoryError and
// returns "ok:<str(expr)>" or "MemoryError:<message>".
func limitsCatch(t *testing.T, p *scriptling.Scriptling, setup, expr string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(setup)
	b.WriteString("\ntry:\n    __r = \"ok:\" + str(")
	b.WriteString(expr)
	b.WriteString(")\nexcept MemoryError as e:\n    __r = \"MemoryError:\" + str(e)\n__r\n")
	result, err := p.Eval(b.String())
	if err != nil {
		t.Fatalf("eval failed: %v\nscript:\n%s", err, b.String())
	}
	s, errObj := result.AsString()
	if errObj != nil {
		t.Fatalf("result is not a string: %v", result.Inspect())
	}
	return s
}

func limitsExpectOK(t *testing.T, got, want string) {
	t.Helper()
	if got != "ok:"+want {
		t.Fatalf("got %q, want %q", got, "ok:"+want)
	}
}

func limitsExpectMemoryError(t *testing.T, got, msgPart string) {
	t.Helper()
	if !strings.HasPrefix(got, "MemoryError:") {
		t.Fatalf("expected a MemoryError, got %q", got)
	}
	if msgPart != "" && !strings.Contains(got, msgPart) {
		t.Fatalf("MemoryError message %q does not contain %q", got, msgPart)
	}
}

func TestLimitsSecrets(t *testing.T) {
	// CheckElements charges one interface slot (16 bytes on 64-bit) per
	// requested byte, so with a 1024 byte limit 32 bytes is fine and 1000 is not.
	newP := func(t *testing.T, limit int64) *scriptling.Scriptling {
		p := limitsNewInterp(t, limit)
		p.RegisterLibrary(stdlib.SecretsLibrary)
		return p
	}

	t.Run("within limit", func(t *testing.T) {
		p := newP(t, limitsTestLimit)
		limitsExpectOK(t, limitsCatch(t, p, "import secrets", `len(secrets.token_bytes(32))`), "32")
		limitsExpectOK(t, limitsCatch(t, p, "import secrets", `len(secrets.token_hex(32))`), "64")
		limitsExpectOK(t, limitsCatch(t, p, "import secrets", `len(secrets.token_urlsafe(30))`), "40")
	})

	for _, fn := range []string{"token_bytes", "token_hex", "token_urlsafe"} {
		t.Run(fn+" over limit", func(t *testing.T) {
			got := limitsCatch(t, newP(t, limitsTestLimit), "import secrets", `len(secrets.`+fn+`(1000))`)
			limitsExpectMemoryError(t, got, "too large")
		})
	}

	t.Run("no limit", func(t *testing.T) {
		p := newP(t, 0)
		limitsExpectOK(t, limitsCatch(t, p, "import secrets", `len(secrets.token_hex(1000))`), "2000")
		limitsExpectOK(t, limitsCatch(t, p, "import secrets", `len(secrets.token_bytes(1000))`), "1000")
	})
}

func TestRandbitsBounds(t *testing.T) {
	p := scriptling.New()
	p.RegisterLibrary(stdlib.SecretsLibrary)
	result, err := p.Eval(`
import secrets
ok = []
for k in [1, 8, 62]:
    v = secrets.randbits(k)
    ok.append(v >= 0 and v < (1 << k))
ok.append(secrets.randbits(63) >= 0)
errors = []
for k in [64, 1 << 40]:
    try:
        secrets.randbits(k)
        errors.append("no error")
    except OverflowError:
        errors.append("overflow")
[ok, errors]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[[True, True, True, True], ['overflow', 'overflow']]" {
		t.Fatalf("got %s", got)
	}
}

// Encoded tokens are sized by their encoded length, not as containers.
func TestSecretsTokenSizes(t *testing.T) {
	p := scriptling.New(scriptling.WithMaxValueBytes(1024))
	p.RegisterLibrary(stdlib.SecretsLibrary)
	result, err := p.Eval(`
import secrets
out = [len(secrets.token_hex(512)), len(secrets.token_urlsafe(700))]
for f in [lambda: secrets.token_hex(513), lambda: secrets.token_urlsafe(800), lambda: secrets.token_bytes(65)]:
    try:
        f()
        out.append("no error")
    except MemoryError:
        out.append("caught")
out.append(len(secrets.token_bytes(64)))
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[1024, 934, 'caught', 'caught', 'caught', 64]" {
		t.Fatalf("got %s", got)
	}
}

// Extreme token sizes are refused cleanly, not overflowed or allocated.
func TestSecretsExtremeTokenSizes(t *testing.T) {
	p := scriptling.New()
	p.RegisterLibrary(stdlib.SecretsLibrary)
	result, err := p.Eval(`
import secrets
out = []
for f in [secrets.token_bytes, secrets.token_hex, secrets.token_urlsafe]:
    for n in [4611686018427387904, 9223372036854775807, 1 << 41]:
        try:
            f(n)
            out.append("no error")
        except MemoryError:
            out.append("refused")
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); strings.Count(got, "refused") != 9 {
		t.Fatalf("got %s", got)
	}
}

// A token whose raw bytes and encoding together do not fit in the memory
// left is refused before it is built.
func TestSecretsTokenPeakMemory(t *testing.T) {
	p := scriptling.New(scriptling.WithMemoryLimit(4<<20), scriptling.WithMaxValueBytes(0))
	p.RegisterLibrary(stdlib.SecretsLibrary)
	result, err := p.Eval(`
import secrets
n = len(secrets.token_hex(1 << 20))
try:
    secrets.token_hex(3 << 20)
    r = "no error"
except MemoryError:
    r = "refused"
[n, r]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[2097152, 'refused']" {
		t.Fatalf("got %s", got)
	}
}

// token_bytes lists stay charged for as long as they are held.
func TestSecretsTokenBytesRetainedIsCharged(t *testing.T) {
	p := scriptling.New(scriptling.WithMemoryLimit(8<<20), scriptling.WithMaxValueBytes(0))
	p.RegisterLibrary(stdlib.SecretsLibrary)
	_, err := p.Eval(`
import secrets
held = [secrets.token_bytes(200000) for i in range(3)]
`)
	if err == nil {
		t.Fatalf("three 3.2 MB lists held under an 8 MB limit; usage %+v", p.ResourceUsage())
	}
	if !strings.Contains(err.Error(), "memory limit exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}
