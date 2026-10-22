package extlibs

import (
	"strconv"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

func newDumpsScriptling() *scriptling.Scriptling {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterYAMLLibrary(p)
	RegisterTOMLLibrary(p)
	return p
}

func nestedListScript(n int) string {
	return "x = 1\nfor i in range(" + strconv.Itoa(n) + "):\n    x = [x]\n"
}

var deepDumps = map[string]string{
	"msgpack.packb": "import msgpack\nmsgpack.packb(x)",
	"yaml.dump":     "import yaml\nyaml.dump(x)",
	"toml.dumps":    "import toml\ntoml.dumps({'a': x})",
}

// Serializers silently truncated a value nested past 10000 levels to a
// placeholder and wrote corrupt output. They now raise RecursionError, as
// json.dumps raises an error (vuln-0018).
func TestDumpsRejectDeepNesting(t *testing.T) {
	p := newDumpsScriptling()
	if _, err := p.Eval(nestedListScript(1000000)); err != nil {
		t.Fatalf("build: %v", err)
	}
	for name, op := range deepDumps {
		result, err := p.Eval(`
try:
    ` + strings.ReplaceAll(op, "\n", "\n    ") + `
    r = "serialized"
except RecursionError as e:
    r = "RecursionError: " + str(e)
r`)
		if err != nil {
			t.Fatalf("%s: eval: %v", name, err)
		}
		if got := result.Inspect(); !strings.Contains(got, "RecursionError: value too deeply nested to serialize (more than 10000 levels)") {
			t.Errorf("%s: expected a catchable RecursionError, got %.120s", name, got)
		}
	}
}

// The cap is exact: one level past it raises, and at it the value serializes;
// msgpack and yaml round-trip it intact.
func TestDumpsDepthBoundary(t *testing.T) {
	p := newDumpsScriptling()
	for name, op := range deepDumps {
		// 10001 containers puts the innermost value one past the cap.
		if _, err := p.Eval(nestedListScript(10001) + op); err == nil || !strings.Contains(err.Error(), "too deeply nested to serialize") {
			t.Errorf("%s: one level past the cap should raise, got: %v", name, err)
		}
	}
	for name, script := range map[string]string{
		"msgpack": "import msgpack\nmsgpack.unpackb(msgpack.packb(x)) == x",
		"yaml":    "import yaml\nyaml.safe_load(yaml.dump(x)) == x",
	} {
		result, err := p.Eval(nestedListScript(9999) + script)
		if err != nil {
			t.Fatalf("%s at the cap: %v", name, err)
		}
		if result.Inspect() != "True" {
			t.Fatalf("%s did not round-trip a value at the cap", name)
		}
	}
	for name, op := range deepDumps {
		n := 10000
		if name == "toml.dumps" {
			n-- // the wrapping table is a level
		}
		if _, err := p.Eval(nestedListScript(n) + op); err != nil {
			t.Errorf("%s at the cap: %v", name, err)
		}
	}
}

// Ordinary values serialize as before.
func TestDumpsShallowUnchanged(t *testing.T) {
	p := newDumpsScriptling()
	result, err := p.Eval(`
import msgpack, yaml, toml
v = {"a": [1, 2, {"b": "c"}], "d": True}
[msgpack.unpackb(msgpack.packb(v)) == v, yaml.safe_load(yaml.dump(v)) == v, toml.loads(toml.dumps(v)) == v]`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got := result.Inspect(); got != "[True, True, True]" {
		t.Fatalf("got %s", got)
	}
}
