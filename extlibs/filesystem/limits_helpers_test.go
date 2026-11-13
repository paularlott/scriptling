package filesystem

// Copied from extlibs/limits_test.go (shared limits test helpers).

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

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
