package scriptlingcmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/extlibs/provision/become"
)

// --deny-become (or its environment variable or config setting) refuses
// become=True steps, as disabling the subprocess library does.
func TestDenyBecome(t *testing.T) {
	if become.IsRoot() {
		t.Skip("as root, become=True needs no helper and is never refused")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "s.py")
	if err := os.WriteFile(script, []byte(`import scriptling.provision.file as file
file.ensure("`+dir+`/f", "x", become=True)
`), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "scriptling.toml")
	if err := os.WriteFile(config, []byte("[security]\ndeny_become = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, form := range map[string]struct {
		args []string
		env  bool
	}{
		"flag":       {args: []string{"--deny-become"}},
		"env":        {env: true},
		"config":     {args: []string{"--config", config}},
		"subprocess": {args: []string{"--disable-lib", "subprocess"}},
	} {
		t.Run(name, func(t *testing.T) {
			if form.env {
				t.Setenv("SCRIPTLING_DENY_BECOME", "1")
			}
			saved := os.Args
			os.Args = append(append([]string{"scriptling"}, form.args...), script)
			defer func() { os.Args = saved }()
			err := buildRootCommand().Execute(context.Background())
			if err == nil || !strings.Contains(err.Error(), "turned off") {
				t.Fatalf("got %v", err)
			}
		})
	}
}
