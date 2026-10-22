package scriptlingcmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// help imports the module it documents, which runs the module's top-level
// code, but the interpreter it built ignored --no-subprocess: the flag was not
// global (so `help` rejected it) and the env and config forms were never
// folded into the disabled libraries, so a module could spawn processes
// under `scriptling help` that it could not as a script (CWE-863, strix 4250
// vuln-0018). Every form of the setting now reaches help.
func TestNoSubprocessReachesHelp(t *testing.T) {
	run := func(t *testing.T, before, after []string) string {
		t.Helper()
		dir := t.TempDir()
		result := filepath.Join(dir, "result.txt")
		if err := os.WriteFile(filepath.Join(dir, "probe.py"), []byte(`
try:
    import subprocess
    state = "registered"
except:
    state = "disabled"
import pathlib
pathlib.Path("`+result+`").write_text(state)

def f():
    """probe"""
    return 1
`), 0o644); err != nil {
			t.Fatal(err)
		}
		args := append([]string{"scriptling"}, before...)
		args = append(args, "-L", dir, "help")
		args = append(args, after...)
		args = append(args, "probe")

		saved := os.Args
		os.Args = args
		defer func() { os.Args = saved }()
		devnull, _ := os.Open(os.DevNull)
		savedOut := os.Stdout
		os.Stdout = devnull
		err := buildRootCommand().Execute(context.Background())
		os.Stdout = savedOut
		if err != nil {
			t.Fatalf("help %v: %v", args, err)
		}
		got, err := os.ReadFile(result)
		if err != nil {
			t.Fatalf("the probe module did not run: %v", err)
		}
		return string(got)
	}

	config := filepath.Join(t.TempDir(), "scriptling.toml")
	if err := os.WriteFile(config, []byte("[security]\nno_subprocess = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, form := range map[string]struct {
		before, after []string
		env           bool
	}{
		"flag before help": {before: []string{"--no-subprocess"}},
		"flag after help":  {after: []string{"--no-subprocess"}},
		"env":              {env: true},
		"config":           {before: []string{"--config", config}},
	} {
		t.Run(name, func(t *testing.T) {
			if form.env {
				t.Setenv("SCRIPTLING_NO_SUBPROCESS", "1")
			}
			if got := run(t, form.before, form.after); got != "disabled" {
				t.Fatalf("subprocess was %s under help", got)
			}
		})
	}
	t.Run("without the setting the library is there", func(t *testing.T) {
		if got := run(t, nil, nil); got != "registered" {
			t.Fatalf("control run: subprocess was %s", got)
		}
	})
}
