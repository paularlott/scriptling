package extlibs

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
)

func TestKVDefaultStoreNotOpenRaises(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	// Using the default store before anything opened it must be a catchable
	// error, not a host crash (previously: nil-pointer SIGSEGV).
	result, err := p.Eval(`
import scriptling.runtime as runtime
r = "none"
try:
    runtime.kv.default.set("k", 1)
    r = "not-raised"
except Exception as e:
    r = "caught:" + str(e)[0:30]
r
`)
	if err != nil {
		t.Fatalf("expected catchable error, got host error: %v", err)
	}
	got := result.Inspect()
	if !strings.Contains(got, "caught:kv store is not open") {
		t.Fatalf("result = %q, want catchable 'kv store is not open'", got)
	}

	// Named stores keep working through the same object construction path.
	result, err = p.Eval(`
s = runtime.kv.open(":memory:probe")
s.set("k", 42)
str(s.get("k"))
`)
	if err != nil {
		t.Fatalf("named store failed: %v", err)
	}
	if result.Inspect() != "42" {
		t.Fatalf("named store get = %q, want 42", result.Inspect())
	}
}
