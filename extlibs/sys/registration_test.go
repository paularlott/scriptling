package sys_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/sys"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNameConstant(t *testing.T) {
	if sys.LibraryName != "sys" {
		t.Errorf("sys.LibraryName = %q, want %q (scripts import it by this name)", sys.LibraryName, "sys")
	}
}

func TestSysRegisterArgv(t *testing.T) {
	p := scriptling.New()
	sys.Register(p, []string{"script.py", "one", "two"}, nil)
	if got := scripttest.Eval(t, p, "import sys\nsys.argv").Inspect(); got != "['script.py', 'one', 'two']" {
		t.Errorf("sys.argv = %s, want the argv passed to sys.Register", got)
	}
}

func TestSysRegisterStdinProvidesInput(t *testing.T) {
	p := scriptling.New()
	sys.Register(p, nil, strings.NewReader("first line\nsecond\n"))
	if got := scripttest.EvalString(t, p, "input()"); got != "first line" {
		t.Errorf("input() = %q, want the first line of the stdin given to sys.Register", got)
	}
}

func TestSysWithoutStdinHasNoInputBuiltin(t *testing.T) {
	p := scriptling.New()
	sys.Register(p, nil, nil)
	if typ, _ := scripttest.Raises(t, p, "input()"); typ != "NameError" {
		t.Errorf("input() with no stdin registered raised %s, want NameError", typ)
	}
}

func TestSysNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	scripttest.ImportFails(t, p, "sys", "it is only available after sys.Register(p, argv, stdin)")
}
