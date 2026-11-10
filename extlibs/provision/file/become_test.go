package file

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/provision/become"
)

func TestMain(m *testing.M) {
	become.ServeIfHelper()
	os.Exit(m.Run())
}

// helper runs the root helper as this user, through a stand-in for sudo.
func helper(t *testing.T, paths []string) *become.Become {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	p := filepath.Join(t.TempDir(), "sudo")
	script := "#!/bin/sh\nwhile [ \"$1\" = -n ] || [ \"$1\" = -- ]; do shift; done\nexec \"$@\"\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	b := become.New(become.Config{Sudo: []string{p}, Command: []string{os.Args[0], become.HelperArg},
		Paths: fssecurity.Config{AllowedPaths: paths}, NonRootHelper: true})
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func evalWith(t *testing.T, paths []string, b *become.Become, code string) (string, error) {
	t.Helper()
	p := scriptling.New()
	RegisterConfigured(p, fssecurity.Config{AllowedPaths: paths}, WithBecome(b))
	v, err := p.Eval("import scriptling.provision.file as file\n" + code)
	if err != nil {
		return "", err
	}
	return v.Inspect(), nil
}

// Through the helper every operation does what it does here.
func TestBecomeOperations(t *testing.T) {
	dir := t.TempDir()
	me, _ := user.Current()
	got, err := evalWith(t, nil, helper(t, nil), strings.ReplaceAll(`
r = []
r.append(file.ensure("D/a/conf", "x = 1\n", become=True))
r.append(file.ensure("D/a/conf", "x = 1\n", become=True))
r.append(file.ensure("D/a/conf", "x = 1\n", mode=0o600, become=True))
r.append(file.ensure("D/a/conf", "x = 1\n", owner="`+me.Username+`", group=`+me.Gid+`, become=True))
r.append(file.ensure_block("D/a/conf", "y = 2", id="t", become=True))
r.append(file.absent_block("D/a/conf", id="t", become=True))
r.append(file.ensure_directory("D/b", mode=0o700, become=True))
r.append(file.ensure_directory("D/b", mode=0o750, become=True))
r.append(file.absent_directory("D/b", become=True))
r.append(file.absent("D/a/conf", become=True))
r.append(file.absent("D/a/conf", become=True))
r
`, "D", dir))
	if err != nil {
		t.Fatal(err)
	}
	want := "['created', 'unchanged', 'updated', 'unchanged', 'created', 'removed', 'created', 'updated', 'removed', 'removed', 'absent']"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestBecomeErrors(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := helper(t, []string{dir})
	// A bad owner is an ordinary error, before anything is written.
	_, err = evalWith(t, []string{dir}, b, `file.ensure("`+dir+`/f", "x", owner="no-such-user-here", become=True)`)
	if err == nil || !strings.Contains(err.Error(), `ensure: no user "no-such-user-here"`) {
		t.Fatalf("got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "f")); statErr == nil {
		t.Fatal("the file was written")
	}
	// The helper holds to the allowed paths: an uncatchable PermissionError.
	_, err = evalWith(t, nil, b, `
try:
    file.ensure("/etc/scriptling-test-denied", "x", become=True)
except Exception:
    pass
`)
	if err == nil || !strings.Contains(err.Error(), "outside allowed") {
		t.Fatalf("got %v", err)
	}
	if !become.IsRoot() {
		_, err = evalWith(t, nil, nil, `
try:
    file.ensure("`+dir+`/g", "x", become=True)
except Exception:
    pass
`)
		if err == nil || !strings.Contains(err.Error(), "--deny-become") {
			t.Fatalf("got %v", err)
		}
	}
	_, err = evalWith(t, nil, nil, `file.ensure("`+dir+`/h", "x", owner=[1])`)
	if err == nil || !strings.Contains(err.Error(), "owner must be a name or a number") {
		t.Fatalf("got %v", err)
	}
}

// An explicit mode is exactly what a new directory gets, whatever the umask.
func TestDirectoryModeExact(t *testing.T) {
	dir := t.TempDir()
	if _, err := evalWith(t, nil, nil, `file.ensure_directory("`+dir+`/x", mode=0o777)`); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(filepath.Join(dir, "x")); info.Mode().Perm() != 0o777 {
		t.Errorf("mode %o", info.Mode().Perm())
	}
}
