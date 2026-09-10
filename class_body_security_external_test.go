package scriptling_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/stdlib"
)

// A security violation (denied path) inside a class body must NOT be swallowed
// by the class defining successfully — it must propagate. This is the concrete
// reason class bodies must execute every statement, not just method defs: the
// uncatchable-bypass guarantee is moot if the statement never runs.
func TestClassBodySecurityViolationPropagates(t *testing.T) {
	allowedDir := t.TempDir()
	p := scriptling.New()
	extlibs.RegisterOSLibrary(p, []string{allowedDir})

	_, err := p.Eval(`
import os
class C:
    files = os.listdir("/etc")
`)
	if err == nil {
		t.Fatal("expected the security violation in the class body to propagate, not be swallowed")
	}
	if !strings.Contains(err.Error(), "denied") && !strings.Contains(err.Error(), "outside allowed") {
		t.Fatalf("error = %q, want a security denial", err.Error())
	}
}

// A security violation inside __init__ must propagate out of instantiation.
// Moving a denied operation from the class body into the constructor must not
// let it evaporate — this is the security-critical form of the __init__ raise
// propagation fix.
func TestInitSecurityViolationPropagates(t *testing.T) {
	allowedDir := t.TempDir()
	p := scriptling.New()
	extlibs.RegisterOSLibrary(p, []string{allowedDir})

	_, err := p.Eval(`
import os
class C:
    def __init__(self):
        self.data = os.listdir("/etc")
c = C()
`)
	if err == nil {
		t.Fatal("expected the security violation in __init__ to propagate, not be swallowed")
	}
	if !strings.Contains(err.Error(), "denied") && !strings.Contains(err.Error(), "outside allowed") {
		t.Fatalf("error = %q, want a security denial", err.Error())
	}
}

// re.sub with a def replacement callback that raises must propagate the raise,
// not splice the exception's text into the result string (data corruption).
func TestReSubDefCallbackRaisePropagates(t *testing.T) {
	p := scriptling.New()
	p.RegisterLibrary(stdlib.ReLibrary)
	_, err := p.Eval(`
import re
def repl(m):
    raise ValueError("repl-boom")
re.sub("a", repl, "aaa")
`)
	if err == nil || !strings.Contains(err.Error(), "repl-boom") {
		t.Fatalf("expected the replacement raise to propagate, got err=%v", err)
	}
}
