package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
)

func TestScriptlingRegistersLoaderAndMCPLibrary(t *testing.T) {
	p := scriptling.New()
	log := logslog.New(logslog.Config{Level: "error"})

	Scriptling(p, []string{"/tmp/scriptling-lib"}, false, nil, nil, nil, log, "", "", false)

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
		Scriptling(p, []string{libs}, false, allowed, nil, nil, logslog.New(logslog.Config{Level: "error"}), "", "", false)
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
		Scriptling(p, []string{".", "libs"}, false, allowed, nil, nil, logslog.New(logslog.Config{Level: "error"}), "", "", false)
		for _, name := range []string{"sibling", "mylib"} {
			if _, found, err := p.GetLibraryLoader().Load(name); err != nil || !found {
				t.Errorf("allowed=%v: %q not loadable from a relative library directory (found=%v err=%v)", allowed, name, found, err)
			}
		}
	}
}

// Disabling the subprocess library (--no-subprocess, --disable-lib subprocess)
// also refuses stdio MCP servers, which are subprocesses; HTTP MCP clients and
// the default (subprocesses allowed) are unaffected (vuln-0013).
func TestScriptlingMCPStdioFollowsSubprocessPermission(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	// The stdio client starts its process on first use, so call it: /bin/sh
	// is not an MCP server and the call fails, but only after it has run.
	script := `import scriptling.mcp as mcp
c = mcp.Client("/bin/sh", args=["-c", "touch ` + marker + `"])
try:
    c.tools()
except Exception:
    pass`

	run := func(disabled []string) error {
		p := scriptling.New()
		Scriptling(p, nil, false, nil, disabled, nil, logslog.New(logslog.Config{Level: "error"}), "", "", false)
		_, err := p.Eval(script)
		return err
	}

	err := run([]string{"subprocess"})
	if err == nil || !strings.Contains(err.Error(), "subprocess") {
		t.Fatalf("with subprocess disabled, a stdio MCP client must be refused, got: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("the command ran despite subprocess being disabled")
	}

	p := scriptling.New()
	Scriptling(p, nil, false, nil, []string{"subprocess"}, nil, logslog.New(logslog.Config{Level: "error"}), "", "", false)
	if _, err := p.Eval(`import scriptling.mcp as mcp
mcp.Client("https://example.com/mcp")`); err != nil {
		t.Fatalf("HTTP MCP clients should still work with subprocess disabled: %v", err)
	}

	// Default: subprocesses allowed, so the stdio server is started.
	if err := run(nil); err != nil {
		t.Fatalf("with subprocess allowed the stdio client must not be refused: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Fatal("with subprocess allowed the stdio server should have been started")
	}
}

// Disabling the subprocess library also refuses script-driven plugin.load() of
// an executable (a subprocess); HTTP plugins are unaffected (vuln-0015).
func TestScriptlingPluginLoadFollowsSubprocessPermission(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	launcher := filepath.Join(t.TempDir(), "launch.sh")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := `import scriptling.plugin as P
P.load("s", "` + launcher + `", scriptling=False)`

	// The CLI registers the plugin control library with a real (if empty)
	// manager and ProcessesAllowed derived from the disabled-libs list.
	disabled := []string{"subprocess"}
	p := scriptling.New()
	Scriptling(p, nil, false, nil, disabled, nil, logslog.New(logslog.Config{Level: "error"}), "", "", false)
	scriptlingplugin.RegisterLibrariesConfigured(p, scriptlingplugin.NewManager(nil), nil, scriptlingplugin.ProcessesAllowed(disabled))
	if _, err := p.Eval(script); err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("with subprocess disabled, plugin.load of an executable must be refused, got: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("the launcher ran despite subprocess being disabled")
	}
}
