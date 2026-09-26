package extlibs_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/stdlib"
)

// TestSysExecutable: the sys library exposes the running interpreter's path,
// so scripts can relaunch their own binary instead of relying on PATH.
func TestSysExecutable(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	extlibs.RegisterSysLibrary(p, []string{"scriptling", "script.py"}, nil)

	result, err := p.Eval(`
import sys
exe = sys.executable
ok = len(exe) > 0 and ("/" in exe or "\\" in exe)
str(ok) + "|" + str(len(sys.argv))
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	got, _ := result.AsString()
	if !strings.HasPrefix(got, "True|2") {
		t.Fatalf("got %q, want True|2...", got)
	}
}
