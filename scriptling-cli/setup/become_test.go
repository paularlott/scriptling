package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/provision/become"
)

func TestMain(m *testing.M) {
	become.ServeIfHelper()
	os.Exit(m.Run())
}

// become=True runs where the subprocess library does, and is refused where
// it is disabled: running as root through sudo is a subprocess.
func TestBecomeFollowsSubprocess(t *testing.T) {
	sudo := filepath.Join(t.TempDir(), "sudo")
	if err := os.WriteFile(sudo, []byte("#!/bin/sh\nwhile [ \"$1\" = -n ] || [ \"$1\" = -- ]; do shift; done\nexec \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := become.New(become.Config{Sudo: []string{sudo}, Command: []string{os.Args[0], become.HelperArg}, NonRootHelper: true})
	defer b.Close()
	SetBecome(b)
	defer SetBecome(nil)

	run := func(disabled []string, code string) error {
		p := scriptling.New()
		Scriptling(p, nil, false, nil, disabled, nil, logslog.New(logslog.Config{Level: "error"}), "", "", false)
		_, err := p.Eval(code)
		return err
	}
	dir := t.TempDir()
	write := `import scriptling.provision.file as file
file.ensure("` + dir + `/f", "x", become=True)`
	if err := run(nil, write); err != nil {
		t.Fatalf("with subprocess allowed: %v", err)
	}
	if become.IsRoot() {
		return // as root, become=True needs no helper and is never refused
	}
	err := run([]string{"subprocess"}, write)
	if err == nil || !strings.Contains(err.Error(), "turned off") {
		t.Fatalf("with subprocess disabled: %v", err)
	}
	for _, lib := range []string{"scriptling.provision.command", "scriptling.provision.package"} {
		if err := run([]string{"subprocess"}, "import "+lib); err == nil {
			t.Errorf("%s is there with subprocess disabled", lib)
		}
	}
}
