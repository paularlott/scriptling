package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// A deeply nested script used to overflow the Go stack in the passes after
// parsing, killing the whole process, including from inside the sandbox
// library (strix 4250 vuln-0009). It is now an ordinary error and the
// interpreter keeps working.
func TestDeepScriptIsAnErrorNotACrash(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)
	for _, src := range []string{
		"x = " + strings.Repeat("[", 800000) + strings.Repeat("]", 800000),
		"a = 1\nx = a" + strings.Repeat(".b", 800000),
		"x = " + strings.Repeat("-", 1600000) + "1",
	} {
		if _, err := p.Eval(src); err == nil || !strings.Contains(err.Error(), "parser errors") {
			t.Fatalf("expected a parse error, got %v", err)
		}
	}
	if r, err := p.Eval("1 + 1"); err != nil || r.Inspect() != "2" {
		t.Fatalf("interpreter unusable afterwards: %v %v", r, err)
	}

}
