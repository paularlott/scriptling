package toml_test

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/toml"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNameConstant(t *testing.T) {
	if toml.LibraryName != "toml" {
		t.Errorf("toml.LibraryName = %q, want %q (scripts import it by this name)", toml.LibraryName, "toml")
	}
}

func TestTOMLRegisterAndLoads(t *testing.T) {
	p := scriptling.New()
	toml.Register(p)
	got := scripttest.Eval(t, p, `import toml
d = toml.loads('title = "x"\n[server]\nport = 8080\n')
back = toml.loads(toml.dumps(d))
[d["title"], d["server"]["port"], back == d]`).Inspect()
	if want := "['x', 8080, True]"; got != want {
		t.Errorf("toml loads/dumps = %s, want %s", got, want)
	}
}

func TestTOMLNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	scripttest.ImportFails(t, p, "toml", "it is only available after toml.Register(p)")
}

func TestTOMLBadInputIsCatchable(t *testing.T) {
	p := scriptling.New()
	toml.Register(p)
	if typ, _ := scripttest.Raises(t, p, "import toml\ntoml.loads('= no key')"); typ == "" {
		t.Error("malformed TOML must raise")
	}
}
