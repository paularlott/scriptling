package setup

import (
	"os"
	"path/filepath"
	"testing"

	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
)

func TestScriptlingRegistersLoaderAndMCPLibrary(t *testing.T) {
	p := scriptling.New()
	log := logslog.New(logslog.Config{Level: "error"})

	Scriptling(p, []string{"/tmp/scriptling-lib"}, false, nil, nil, nil, log, "", "")

	if p.GetLibraryLoader() == nil {
		t.Fatal("expected library loader to be configured")
	}

	if err := p.Import("scriptling.mcp.tool"); err != nil {
		t.Fatalf("expected MCP tool helpers to be registered: %v", err)
	}
}

// With --allowed-paths, a library reached through a symlink in a library
// directory must land inside the library directories or an allowed path; with
// no restriction (nil) the loader follows symlinks as it always has.
func TestScriptlingLibraryLoaderHonoursAllowedPaths(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	libs := filepath.Join(base, "libs")
	data := filepath.Join(base, "data")
	secret := filepath.Join(base, "secret")
	for _, d := range []string{libs, data, secret} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(libs, "good.py"), "VALUE = 1\n")
	write(filepath.Join(data, "shared.py"), "SHARED = 2\n")
	write(filepath.Join(secret, "token.py"), "SECRET = 3\n")
	if err := os.Symlink(filepath.Join(data, "shared.py"), filepath.Join(libs, "shared.py")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if err := os.Symlink(filepath.Join(secret, "token.py"), filepath.Join(libs, "leak.py")); err != nil {
		t.Fatal(err)
	}

	load := func(allowed []string, name string) bool {
		p := scriptling.New()
		Scriptling(p, []string{libs}, false, allowed, nil, nil, logslog.New(logslog.Config{Level: "error"}), "", "")
		_, found, err := p.GetLibraryLoader().Load(name)
		if err != nil {
			t.Fatalf("Load(%q): %v", name, err)
		}
		return found
	}

	if !load(nil, "leak") {
		t.Error("with no allowed paths the loader should follow the link, as before")
	}
	for _, allowed := range [][]string{{data}, {}} {
		if load(allowed, "leak") {
			t.Errorf("allowed=%v: a link out of the library directory was followed", allowed)
		}
		if !load(allowed, "good") {
			t.Errorf("allowed=%v: a normal library was refused", allowed)
		}
	}
	if !load([]string{data}, "shared") {
		t.Error("a link into an allowed path should load")
	}
	if load([]string{}, "shared") {
		t.Error("under deny-all a link out of the library directory should be refused")
	}
}

// The CLI passes the script's own directory as a relative path ("." for
// "scriptling main.py"); imports from it must keep working under
// --allowed-paths, including a relative allowed path.
func TestScriptlingLibraryLoaderRelativeDirs(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "libs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "sibling.py"), []byte("A = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "libs", "mylib.py"), []byte("B = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(base)
	for _, allowed := range [][]string{{"./data"}, {}} {
		p := scriptling.New()
		Scriptling(p, []string{".", "libs"}, false, allowed, nil, nil, logslog.New(logslog.Config{Level: "error"}), "", "")
		for _, name := range []string{"sibling", "mylib"} {
			if _, found, err := p.GetLibraryLoader().Load(name); err != nil || !found {
				t.Errorf("allowed=%v: %q not loadable from a relative library directory (found=%v err=%v)", allowed, name, found, err)
			}
		}
	}
}
