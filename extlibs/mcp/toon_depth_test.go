package mcp_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/mcp"
)

// toon.encode silently truncated a value nested past 10000 levels; it now
// raises RecursionError (vuln-0018).
func TestToonEncodeRejectsDeepNesting(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToon(sl)
	for _, op := range []string{"toon.encode(x)", "toon.encode_options(x, 2, ',')"} {
		result, err := sl.Eval(`
import scriptling.toon as toon
x = 1
for i in range(1000000):
    x = [x]
try:
    ` + op + `
    r = "encoded"
except RecursionError as e:
    r = "RecursionError: " + str(e)
r`)
		if err != nil {
			t.Fatalf("%s: eval: %v", op, err)
		}
		if got := result.Inspect(); !strings.Contains(got, "RecursionError: value too deeply nested to serialize") {
			t.Errorf("%s: expected a catchable RecursionError, got %.120s", op, got)
		}
	}
}
