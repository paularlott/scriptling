package subprocess_test

import (
	"runtime"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/subprocess"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNameConstant(t *testing.T) {
	if subprocess.LibraryName != "subprocess" {
		t.Errorf("subprocess.LibraryName = %q, want %q (scripts import it by this name)", subprocess.LibraryName, "subprocess")
	}
}

func TestSubprocessRegisterAndRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX echo")
	}
	p := scriptling.New()
	subprocess.Register(p)
	got := scripttest.Eval(t, p, `import subprocess
r = subprocess.run(["echo", "hi"], capture_output=True, text=True)
[r.returncode, r.stdout.strip()]`).Inspect()
	if want := "[0, 'hi']"; got != want {
		t.Errorf("subprocess.run(echo hi) = %s, want %s", got, want)
	}
}

func TestSubprocessMissingCommandIsCatchable(t *testing.T) {
	p := scriptling.New()
	subprocess.Register(p)
	if typ, _ := scripttest.Raises(t, p, "import subprocess\nsubprocess.run(['scriptling-no-such-command-xyz'], check=True)"); typ == "" {
		t.Error("running a missing command must raise")
	}
}

func TestSubprocessNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	scripttest.ImportFails(t, p, "subprocess", "it is only available after subprocess.Register(p)")
}
