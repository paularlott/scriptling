package extlibs

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

// A sandboxed interpreter runs in the same process, so a script that made the
// sandbox parse a deeply nested program killed the host (strix 4250
// vuln-0009). The sandbox now reports the parse error and the host carries on.
func TestSandboxContainsDeepScript(t *testing.T) {
	ResetRuntime()
	SetSandboxFactory(func() SandboxInstance {
		p := scriptling.New()
		stdlib.RegisterAll(p)
		return p
	})
	defer SetSandboxFactory(nil)

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRuntimeLibraryAll(p, nil)
	p.EnableOutputCapture()
	if _, err := p.Eval(`import scriptling.runtime.sandbox as sb
box = sb.create()
box.exec("x = " + "["*800000 + "]"*800000)
print("survived", box.exit_code())`); err != nil {
		t.Fatal(err)
	}
	if out := p.GetOutput(); !strings.Contains(out, "survived 1") {
		t.Fatalf("output %q", out)
	}
}
