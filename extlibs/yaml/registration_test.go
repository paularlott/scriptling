package yaml_test

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/yaml"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNameConstant(t *testing.T) {
	if yaml.LibraryName != "yaml" {
		t.Errorf("yaml.LibraryName = %q, want %q (scripts import it by this name)", yaml.LibraryName, "yaml")
	}
}

func TestYAMLRegisterAndRoundTrip(t *testing.T) {
	p := scriptling.New()
	yaml.Register(p)
	got := scripttest.Eval(t, p, `import yaml
d = yaml.safe_load("a: 1\nb: [x, y]\n")
back = yaml.safe_load(yaml.safe_dump(d))
[back == d, back["a"], back["b"][1], yaml.load("k: v")["k"], yaml.safe_load(yaml.dump([1, 2]))[1]]`).Inspect()
	if want := "[True, 1, 'y', 'v', 2]"; got != want {
		t.Errorf("yaml round trip = %s, want %s", got, want)
	}
}

func TestYAMLNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	scripttest.ImportFails(t, p, "yaml", "it is only available after yaml.Register(p)")
}

func TestYAMLRegisterTwice(t *testing.T) {
	p := scriptling.New()
	yaml.Register(p)
	yaml.Register(p)
	if got := scripttest.EvalString(t, p, "import yaml\nyaml.safe_load('x: ok')['x']"); got != "ok" {
		t.Errorf("after registering twice: got %q", got)
	}
}

func TestYAMLBadInputIsCatchable(t *testing.T) {
	p := scriptling.New()
	yaml.Register(p)
	if typ, _ := scripttest.Raises(t, p, "import yaml\nyaml.safe_load('a: [unclosed')"); typ == "" {
		t.Error("malformed YAML must raise")
	}
}
