package command

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/provision/become"
)

func TestMain(m *testing.M) {
	become.ServeIfHelper()
	os.Exit(m.Run())
}

// fakeSudo runs its command without -n and --: the helper as this user.
func fakeSudo(t *testing.T) *become.Become {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sudo")
	script := "#!/bin/sh\nwhile [ \"$1\" = -n ] || [ \"$1\" = -- ]; do shift; done\nexec \"$@\"\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	b := become.New(become.Config{Sudo: []string{p}, Command: []string{os.Args[0], become.HelperArg}, NonRootHelper: true})
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func eval(t *testing.T, b *become.Become, code string) (string, error) {
	t.Helper()
	p := scriptling.New()
	if b != nil {
		RegisterConfigured(p, WithBecome(b))
	} else {
		Register(p)
	}
	v, err := p.Eval("import scriptling.provision.command as command\n" + code)
	if err != nil {
		return "", err
	}
	return v.Inspect(), nil
}

func TestRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	dir := t.TempDir()
	b := fakeSudo(t)
	for _, root := range []string{"False", "True"} {
		t.Run("become="+root, func(t *testing.T) {
			got, err := eval(t, b, `
r = command.run(["sh", "-c", "read x; echo $x $V; pwd; echo err >&2"], input="in\n", env={"V": 7},
                cwd="`+dir+`", become=`+root+`)
f = command.run("exit 4", check=False, become=`+root+`)
try:
    command.run(["sh", "-c", "echo first; echo why >&2; exit 9"], become=`+root+`)
    failed = ""
except Exception as e:
    failed = str(e)
try:
    command.run(["sleep", "10"], timeout=0.2, become=`+root+`)
    timed = ""
except Exception as e:
    timed = str(e)
[r.stdout.split("\n")[0], r.stderr, r.returncode, r.ok, r.args[0], f.returncode, f.ok, failed, timed]
`)
			if err != nil {
				t.Fatal(err)
			}
			want := `['in 7', 'err\n', 0, True, 'sh', 4, False, "run: sh -c 'echo first; echo why >&2; exit 9' exited with status 9:\nwhy", 'run: sleep timed out after 200ms']`
			if got != want {
				t.Errorf("got  %s\nwant %s", got, want)
			}
		})
	}
}

func TestRunArguments(t *testing.T) {
	for code, want := range map[string]string{
		`command.run([])`:                   "the command is empty",
		`command.run("  ")`:                 "the command is empty",
		`command.run(["echo", 1])`:          "must be a string",
		`command.run(42)`:                   "must be a list of strings or a string",
		`command.run(["true"], timeout=0)`:  "timeout must be",
		`command.run(["true"], env=[1])`:    "env must be a dict",
		`command.run(["true"], cwd=1)`:      "cwd must be a string",
		`command.run(["/no/such/program"])`: "/no/such/program",
	} {
		if _, err := eval(t, nil, code); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want %q", code, err, want)
		}
	}
}

func TestBecomeNotEnabled(t *testing.T) {
	if become.IsRoot() {
		t.Skip("running as root: become=True runs here")
	}
	// The refusal is a PermissionError the script cannot catch.
	_, err := eval(t, nil, `
try:
    command.run(["true"], become=True)
except Exception:
    pass
`)
	if err == nil || !strings.Contains(err.Error(), "--deny-become") {
		t.Fatalf("got %v", err)
	}
}

func TestRoot(t *testing.T) {
	got, err := eval(t, nil, `command.is_root()`)
	if err != nil || got != map[bool]string{true: "True", false: "False"}[become.IsRoot()] {
		t.Fatalf("%s %v", got, err)
	}
	_, err = eval(t, nil, `command.require_root()`)
	if become.IsRoot() != (err == nil) {
		t.Fatalf("require_root: %v", err)
	}
}
