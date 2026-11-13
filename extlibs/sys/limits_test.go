package sys_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/sys"
	"github.com/paularlott/scriptling/stdlib"
)

// limitsTestLimit is the value limit used by the tests in this file: small
// enough that a few KB of data goes over it.
const limitsTestLimit = 1024

// limitsBigSize is comfortably over limitsTestLimit.
const limitsBigSize = 4096

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

func TestLimitsSysStdinRead(t *testing.T) {
	newP := func(t *testing.T, limit int64, stdin string) *scriptling.Scriptling {
		p := limitsNewInterp(t, limit)
		sys.Register(p, []string{"script"}, strings.NewReader(stdin))
		return p
	}

	t.Run("within limit", func(t *testing.T) {
		got := limitsCatch(t, newP(t, limitsTestLimit, "line one\nline two\n"), "import sys", `sys.stdin.read()`)
		limitsExpectOK(t, got, "line one\nline two\n")
	})
	t.Run("over limit", func(t *testing.T) {
		got := limitsCatch(t, newP(t, limitsTestLimit, strings.Repeat("i", limitsBigSize)), "import sys", `len(sys.stdin.read())`)
		limitsExpectMemoryError(t, got, "too large")
	})
	t.Run("no limit", func(t *testing.T) {
		got := limitsCatch(t, newP(t, 0, strings.Repeat("i", limitsBigSize)), "import sys", `len(sys.stdin.read())`)
		limitsExpectOK(t, got, "4096")
	})
}
