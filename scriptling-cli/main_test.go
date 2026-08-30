package main

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/cli"
	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/lint"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/bootstrap"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/pluginpack"
)

type cliLogEntry struct {
	level string
	msg   string
	args  []any
}

type cliCaptureLogger struct {
	mu      sync.Mutex
	entries []cliLogEntry
}

func (l *cliCaptureLogger) Trace(msg string, keysAndValues ...any) {
	l.record("trace", msg, keysAndValues...)
}
func (l *cliCaptureLogger) Debug(msg string, keysAndValues ...any) {
	l.record("debug", msg, keysAndValues...)
}
func (l *cliCaptureLogger) Info(msg string, keysAndValues ...any) {
	l.record("info", msg, keysAndValues...)
}
func (l *cliCaptureLogger) Warn(msg string, keysAndValues ...any) {
	l.record("warn", msg, keysAndValues...)
}
func (l *cliCaptureLogger) Error(msg string, keysAndValues ...any) {
	l.record("error", msg, keysAndValues...)
}
func (l *cliCaptureLogger) Fatal(msg string, keysAndValues ...any) {
	l.record("fatal", msg, keysAndValues...)
}
func (l *cliCaptureLogger) With(key string, value any) logger.Logger {
	return l
}
func (l *cliCaptureLogger) WithError(err error) logger.Logger { return l }
func (l *cliCaptureLogger) WithGroup(group string) logger.Logger {
	return l
}
func (l *cliCaptureLogger) record(level, msg string, keysAndValues ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	args := append([]any(nil), keysAndValues...)
	l.entries = append(l.entries, cliLogEntry{level: level, msg: msg, args: args})
}
func (l *cliCaptureLogger) snapshot() []cliLogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]cliLogEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

func TestBuildLibDirs(t *testing.T) {
	t.Run("base dir only when no extras", func(t *testing.T) {
		dirs := bootstrap.BuildLibDirs("/app/scripts", nil)
		if len(dirs) != 1 || dirs[0] != "/app/scripts" {
			t.Errorf("expected [/app/scripts], got %v", dirs)
		}
	})

	t.Run("base dir first then extras", func(t *testing.T) {
		dirs := bootstrap.BuildLibDirs("/app/scripts", []string{"/shared/libs", "/extra"})
		if len(dirs) != 3 {
			t.Fatalf("expected 3 dirs, got %d: %v", len(dirs), dirs)
		}
		if dirs[0] != "/app/scripts" {
			t.Errorf("expected base dir first, got %s", dirs[0])
		}
		if dirs[1] != "/shared/libs" {
			t.Errorf("expected /shared/libs second, got %s", dirs[1])
		}
		if dirs[2] != "/extra" {
			t.Errorf("expected /extra third, got %s", dirs[2])
		}
	})

	t.Run("empty strings in extras are skipped", func(t *testing.T) {
		dirs := bootstrap.BuildLibDirs("/base", []string{"", "/valid", ""})
		if len(dirs) != 2 {
			t.Fatalf("expected 2 dirs, got %d: %v", len(dirs), dirs)
		}
		if dirs[0] != "/base" {
			t.Errorf("expected /base first, got %s", dirs[0])
		}
		if dirs[1] != "/valid" {
			t.Errorf("expected /valid second, got %s", dirs[1])
		}
	})

	t.Run("empty extras slice", func(t *testing.T) {
		dirs := bootstrap.BuildLibDirs("/base", []string{})
		if len(dirs) != 1 || dirs[0] != "/base" {
			t.Errorf("expected [/base], got %v", dirs)
		}
	})

	t.Run("empty base dir is skipped", func(t *testing.T) {
		dirs := bootstrap.BuildLibDirs("", []string{"/extra"})
		if len(dirs) != 1 || dirs[0] != "/extra" {
			t.Errorf("expected [/extra], got %v", dirs)
		}
	})

	t.Run("empty base dir and no extras returns empty", func(t *testing.T) {
		dirs := bootstrap.BuildLibDirs("", nil)
		if len(dirs) != 0 {
			t.Errorf("expected empty slice, got %v", dirs)
		}
	})
}

func TestLoadPluginManagerLogsPluginCrash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell plugin helper is unix-only")
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "cli-crash-plugin")
	script := `#!/bin/sh
read req
echo '{"jsonrpc":"2.0","id":1,"result":{"protocol":"1.0","transport":"json","library":{"name":"cli-crash","version":"1.0.0","description":"crash test"},"capabilities":[],"schema":{"functions":[],"classes":[],"constants":[]}}}'
sleep 0.05
exit 2
`
	if err := os.WriteFile(helper, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}

	previousLogger := globalLogger
	logs := &cliCaptureLogger{}
	globalLogger = logs
	defer func() {
		globalLogger = previousLogger
	}()

	manager, err := loadPluginManager(context.Background(), []string{dir}, nil, nil, nil, false)
	if err != nil {
		t.Fatalf("loadPluginManager: %v", err)
	}
	if manager == nil {
		t.Fatal("expected plugin manager")
	}
	defer manager.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, entry := range logs.snapshot() {
			if entry.level == "error" && entry.msg == "Plugin process exited" && containsLogPair(entry.args, "plugin", "plugin.cli-crash") {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected crash log entry, got %#v", logs.snapshot())
}

func containsLogPair(args []any, key string, value any) bool {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}

func TestParseAllowedPaths(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		if bootstrap.ParseAllowedPaths("") != nil {
			t.Error("expected nil for empty string")
		}
	})

	t.Run("dash returns empty slice (deny all)", func(t *testing.T) {
		result := bootstrap.ParseAllowedPaths("-")
		if result == nil || len(result) != 0 {
			t.Errorf("expected empty slice, got %v", result)
		}
	})

	t.Run("single path", func(t *testing.T) {
		result := bootstrap.ParseAllowedPaths("/tmp")
		if len(result) != 1 || result[0] != "/tmp" {
			t.Errorf("expected [/tmp], got %v", result)
		}
	})

	t.Run("multiple paths", func(t *testing.T) {
		result := bootstrap.ParseAllowedPaths("/tmp,/var/data, /home/user")
		if len(result) != 3 {
			t.Fatalf("expected 3 paths, got %d: %v", len(result), result)
		}
		if result[0] != "/tmp" || result[1] != "/var/data" || result[2] != "/home/user" {
			t.Errorf("unexpected paths: %v", result)
		}
	})

	t.Run("whitespace-only entries are ignored", func(t *testing.T) {
		result := bootstrap.ParseAllowedPaths("/tmp, , /var")
		if len(result) != 2 {
			t.Fatalf("expected 2 paths, got %d: %v", len(result), result)
		}
	})
}

func TestGetExitCode(t *testing.T) {
	t.Run("plain exit error", func(t *testing.T) {
		code, ok := getExitCode(exitCodeError{code: 7})
		if !ok || code != 7 {
			t.Fatalf("expected exit code 7, got code=%d ok=%v", code, ok)
		}
	})

	t.Run("wrapped exit error", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", exitCodeError{code: 3, err: errors.New("boom")})
		code, ok := getExitCode(err)
		if !ok || code != 3 {
			t.Fatalf("expected exit code 3, got code=%d ok=%v", code, ok)
		}
	})
}

func TestOutputLintResultReturnsExitError(t *testing.T) {
	result := &lint.Result{
		HasErrors: true,
		Errors: []lint.LintError{
			{Line: 1, Message: "bad", Severity: lint.SeverityError},
		},
	}

	err := outputLintResult(result, "text")
	code, ok := getExitCode(err)
	if !ok || code != 1 {
		t.Fatalf("expected exit code 1, got code=%d ok=%v err=%v", code, ok, err)
	}
}

func TestResolvePluginSpecs(t *testing.T) {
	cases := []struct {
		name    string
		plugins []string
		args    []string
		envs    []string
		want    []pluginSpec
		fails   string
	}{
		{
			name:    "path only",
			plugins: []string{"/usr/local/bin/knot"},
			want:    []pluginSpec{{Path: "/usr/local/bin/knot"}},
		},
		{
			name:    "bare env goes to the sole plugin",
			plugins: []string{"/usr/local/bin/knot"},
			envs:    []string{"KNOT_DB=/var/lib/knot", "LOG=debug"},
			want:    []pluginSpec{{Path: "/usr/local/bin/knot", Env: []string{"KNOT_DB=/var/lib/knot", "LOG=debug"}}},
		},
		{
			name:    "qualified env names its plugin, value keeps its equals",
			plugins: []string{"/usr/local/bin/knot", "/usr/local/bin/other"},
			envs:    []string{"knot=KNOT_DB=/x", "other=--flag=1"},
			want: []pluginSpec{
				{Path: "/usr/local/bin/knot", Env: []string{"KNOT_DB=/x"}},
				{Path: "/usr/local/bin/other", Env: []string{"--flag=1"}},
			},
		},
		{
			name:  "env without a plugin fails",
			envs:  []string{"KNOT_DB=/x"},
			fails: "without any --plugin",
		},
		{
			name:    "bare env with several plugins is ambiguous",
			plugins: []string{"/usr/local/bin/knot", "/usr/local/bin/other"},
			envs:    []string{"KNOT_DB=/x"},
			fails:   "ambiguous",
		},
		{
			name:    "path with spaces needs no quoting",
			plugins: []string{"/opt/knot dir/knot"},
			args:    []string{"serve"},
			want:    []pluginSpec{{Path: "/opt/knot dir/knot", Args: []string{"serve"}}},
		},
		{
			name:    "bare args go to the sole plugin in order",
			plugins: []string{"/usr/local/bin/knot"},
			args:    []string{"scriptling-server", "--alias", "testing"},
			want:    []pluginSpec{{Path: "/usr/local/bin/knot", Args: []string{"scriptling-server", "--alias", "testing"}}},
		},
		{
			name:    "flag containing = is not mistaken for a qualifier",
			plugins: []string{"/usr/local/bin/knot"},
			args:    []string{"--alias=testing", "--port=8080"},
			want:    []pluginSpec{{Path: "/usr/local/bin/knot", Args: []string{"--alias=testing", "--port=8080"}}},
		},
		{
			name:    "base name qualifies with several plugins",
			plugins: []string{"/usr/local/bin/knot", "/usr/local/bin/other"},
			args:    []string{"knot=serve", "other=--port=8080", "knot=--alias=x"},
			want: []pluginSpec{
				{Path: "/usr/local/bin/knot", Args: []string{"serve", "--alias=x"}},
				{Path: "/usr/local/bin/other", Args: []string{"--port=8080"}},
			},
		},
		{
			name:    "full path qualifies too",
			plugins: []string{"/a/knot", "/b/knot"},
			args:    []string{"/a/knot=first", "/b/knot=second"},
			want: []pluginSpec{
				{Path: "/a/knot", Args: []string{"first"}},
				{Path: "/b/knot", Args: []string{"second"}},
			},
		},
		{
			name:    "no plugins at all",
			plugins: nil,
			want:    []pluginSpec{},
		},
		{
			name:  "arg without any plugin",
			args:  []string{"serve"},
			fails: "without any --plugin",
		},
		{
			name:    "ambiguous bare arg with several plugins",
			plugins: []string{"/usr/local/bin/knot", "/usr/local/bin/other"},
			args:    []string{"serve"},
			fails:   "ambiguous",
		},
		{
			name:    "ambiguous base name",
			plugins: []string{"/a/knot", "/b/knot"},
			args:    []string{"knot=serve"},
			fails:   "ambiguous",
		},
		{
			name:    "empty plugin value",
			plugins: []string{"  "},
			fails:   "empty --plugin value",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolvePluginSpecs(tc.plugins, tc.args, tc.envs)
			if tc.fails != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got %+v", tc.fails, got)
				}
				if !strings.Contains(err.Error(), tc.fails) {
					t.Fatalf("expected an error containing %q, got: %v", tc.fails, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolvePluginSpecs: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d specs, want %d: %+v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i].Path != tc.want[i].Path {
					t.Errorf("spec %d path = %q, want %q", i, got[i].Path, tc.want[i].Path)
				}
				if strings.Join(got[i].Args, "\x00") != strings.Join(tc.want[i].Args, "\x00") {
					t.Errorf("spec %d args = %v, want %v", i, got[i].Args, tc.want[i].Args)
				}
			}
		})
	}
}

// writeArgsPluginHelper writes a shell plugin that records its command line
// arguments to argsFile and then speaks a minimal plugin handshake.
func writeArgsPluginHelper(t *testing.T, path, argsFile string) {
	t.Helper()
	script := `#!/bin/sh
printf '%s\n' "$@" > '` + argsFile + `'
read req
echo '{"jsonrpc":"2.0","id":1,"result":{"protocol":"1.0","transport":"json","library":{"name":"args-plugin","version":"1.0.0","description":"args"},"capabilities":[],"schema":{"functions":[],"classes":[],"constants":[]}}}'
read req
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

func TestLoadPluginManagerExplicitPlugin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell plugin helper is unix-only")
	}
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	helper := filepath.Join(dir, "args-plugin")
	writeArgsPluginHelper(t, helper, argsFile)

	manager, err := loadPluginManager(context.Background(), nil, []string{helper}, []string{"--alias", "testing"}, nil, false)
	if err != nil {
		t.Fatalf("loadPluginManager: %v", err)
	}
	defer manager.Close()

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if got := string(data); got != "--alias\ntesting\n" {
		t.Fatalf("expected the plugin to receive [--alias testing], got %q", got)
	}
	if _, ok := manager.Get("plugin.args-plugin"); !ok {
		t.Fatal("expected the plugin registered under its declared name")
	}
}

func TestLoadPluginManagerExplicitWinsOverDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell plugin helper is unix-only")
	}
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	pluginDir := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(pluginDir, "args-plugin")
	writeArgsPluginHelper(t, helper, argsFile)

	// The same executable, loaded explicitly WITH arguments and also
	// discoverable via --plugin-dir: the explicit entry must win.
	manager, err := loadPluginManager(context.Background(), []string{pluginDir}, []string{helper}, []string{"--alias", "explicit"}, nil, false)
	if err != nil {
		t.Fatalf("loadPluginManager: %v", err)
	}
	defer manager.Close()

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "--alias\nexplicit" {
		t.Fatalf("expected the explicit entry's arguments to win, got %q", got)
	}
	if _, ok := manager.Get("plugin.args-plugin"); !ok {
		t.Fatal("expected the plugin registered under its declared name")
	}
}

// TestSetupScriptReturnsSourceWithoutStaging covers the server-mode setup
// script path: a scheme source is fetched from its plugin and returned as
// source text, with nothing written to disk; a plain path passes through
// untouched.
func TestSetupScriptReturnsSourceWithoutStaging(t *testing.T) {
	dir := t.TempDir()
	localScript := filepath.Join(dir, "local.py")
	if err := os.WriteFile(localScript, []byte("print('local')\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// An in-process fetcher plugin registered through the real bridge.
	fetcher := &stagingFetcher{
		files: map[string]string{
			"manifest.toml": "name = \"ppmain\"\nversion = \"1.0.0\"\n",
		},
		scripts: map[string]string{
			"ppmain://scripts/setup": "import scriptling.runtime as runtime\n",
		},
	}
	pluginSrv := scriptlingplugin.NewServer("ppmain-plugin", "1.0.0", "main test plugin")
	pluginSrv.RegisterFetcher("ppmain", fetcher)
	httpSrv := httptest.NewServer(pluginSrv)
	t.Cleanup(httpSrv.Close)

	manager := scriptlingplugin.NewManager(nil)
	t.Cleanup(func() { _ = manager.Close() })
	if _, err := manager.LoadURL(context.Background(), "ppmainplugin", httpSrv.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	bridge := pluginpack.New(pluginpack.Options{Manager: manager, Context: context.Background()})
	if err := bridge.Register(); err != nil {
		t.Fatalf("Bridge.Register: %v", err)
	}
	pluginBridge = bridge
	t.Cleanup(func() {
		_ = bridge.Close()
		pluginBridge = nil
	})

	// Plain paths pass through unchanged, as a path.
	path, source, name, err := setupScript(context.Background(), localScript)
	if err != nil {
		t.Fatalf("setupScript(local): %v", err)
	}
	if path != localScript || source != nil || name != "" {
		t.Fatalf("expected local path passthrough, got path=%q source=%q name=%q", path, source, name)
	}

	// Count the temp files before and after: a scheme source must not add one.
	tempBefore := countTempScripts(t)

	path, source, name, err = setupScript(context.Background(), "ppmain://scripts/setup")
	if err != nil {
		t.Fatalf("setupScript(scheme): %v", err)
	}
	if path != "" {
		t.Fatalf("expected no file path for a scheme source, got %q", path)
	}
	if name != "ppmain://scripts/setup" {
		t.Fatalf("expected the source to label the script, got %q", name)
	}
	if !strings.Contains(string(source), "runtime") {
		t.Fatalf("unexpected script source: %q", source)
	}
	if got := countTempScripts(t); got != tempBefore {
		t.Fatalf("scheme source staged %d temp file(s); expected none", got-tempBefore)
	}
}

// countTempScripts counts staged scriptling script files left in the temp dir.
func countTempScripts(t *testing.T) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "scriptling-script-*.py"))
	if err != nil {
		t.Fatalf("glob temp scripts: %v", err)
	}
	return len(matches)
}

// TestSetupScriptWithoutPluginNamesTheScheme checks the error a user gets when
// they ask for a scheme source without loading the plugin that serves it: it
// must name the scheme and the flag, not report a missing file.
func TestSetupScriptWithoutPluginNamesTheScheme(t *testing.T) {
	previous := pluginBridge
	pluginBridge = nil
	t.Cleanup(func() { pluginBridge = previous })

	_, _, _, err := setupScript(context.Background(), "notloaded://scripts/setup")
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "notloaded") || !strings.Contains(msg, "--plugin") {
		t.Fatalf("expected the scheme and --plugin named, got: %v", msg)
	}
	if strings.Contains(msg, "no such file") {
		t.Fatalf("expected a plugin error, not a file error: %v", msg)
	}
}

// TestUnregisteredSchemePackageErrorIsActionable is the --package half of the
// same problem: --package does not take plugin scheme sources at all, and the
// error must say why (the plugin declares its packages) rather than mention a
// missing file.
func TestPackageRejectsSchemeSources(t *testing.T) {
	_, _, err := openBundles([]string{"knot://libs"}, false, t.TempDir())
	if err == nil {
		t.Fatal("expected --package to reject a scheme source")
	}
	msg := err.Error()
	if !strings.Contains(msg, "knot") {
		t.Errorf("expected the scheme named, got: %v", msg)
	}
	if !strings.Contains(msg, "automatically") {
		t.Errorf("expected the message to say the library attaches automatically, got: %v", msg)
	}
	if strings.Contains(msg, "no such file") {
		t.Errorf("expected a clear rejection, not a file error: %v", msg)
	}

	// It is rejected before any fetch, so no plugin needs to be loaded and a
	// nonexistent scheme is treated the same way.
	if _, _, err := openBundles([]string{"anything://x"}, false, t.TempDir()); err == nil {
		t.Fatal("expected any scheme source to be rejected")
	}

	// A plain .zip / dir / URL still goes through.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte("name=\"p\"\nversion=\"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := openBundles([]string{dir}, false, t.TempDir()); err != nil {
		t.Fatalf("a directory package must still open: %v", err)
	}
}

// TestWithPluginFlagHintOnlyTouchesUnknownScheme checks the CLI hint is added
// exactly where it applies and nowhere else.
func TestWithPluginFlagHintOnlyTouchesUnknownScheme(t *testing.T) {
	if got := withPluginFlagHint(nil); got != nil {
		t.Errorf("expected nil to pass through, got %v", got)
	}

	other := errors.New("disk on fire")
	if got := withPluginFlagHint(other); got != other {
		t.Errorf("expected an unrelated error to pass through unchanged, got %v", got)
	}

	unknown := fmt.Errorf("failed to load package x://y: %w", pack.ErrUnknownScheme)
	got := withPluginFlagHint(unknown)
	if !errors.Is(got, pack.ErrUnknownScheme) {
		t.Error("expected the hint to preserve the wrapped sentinel")
	}
	if !strings.Contains(got.Error(), "--plugin") {
		t.Errorf("expected the flag hint appended, got: %v", got)
	}
	// Added once, not per layer.
	if n := strings.Count(got.Error(), "--plugin or --plugin-dir"); n != 1 {
		t.Errorf("hint appears %d times, want 1: %v", n, got)
	}
}

type stagingFetcher struct {
	files   map[string]string
	scripts map[string]string
}

func (f *stagingFetcher) Read(ctx context.Context, source, path string) ([]byte, error) {
	if path == "" {
		content, ok := f.scripts[source]
		if !ok {
			return nil, fmt.Errorf("%w: %s", scriptlingplugin.ErrFetchNotFound, source)
		}
		return []byte(content), nil
	}
	content, ok := f.files[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s", scriptlingplugin.ErrFetchNotFound, path)
	}
	return []byte(content), nil
}

func (f *stagingFetcher) List(ctx context.Context, source, path string) ([]scriptlingplugin.FetchEntry, error) {
	if path == "" || path == "." {
		return []scriptlingplugin.FetchEntry{{Name: "manifest.toml"}}, nil
	}
	return nil, fmt.Errorf("%w: %s", scriptlingplugin.ErrFetchNotFound, path)
}

// TestPluginDiscoveryWantedSkipsCheapCommands walks the real command tree and
// asserts, for every command at every depth, whether it may start plugin
// executables. The manager itself is always built — scriptling.plugin.load()
// needs it even with no plugins configured — so this only gates discovery.
//
// It walks the tree rather than checking pluginFreeCommands directly because
// PreRun receives the *leaf* command: `cache clear` arrives as "clear" and
// `pack manifest` as "manifest", so a name-only check silently let nested
// subcommands spawn plugins.
func TestPluginDiscoveryWantedSkipsCheapCommands(t *testing.T) {
	root := buildRootCommand()

	// Every command in a plugin-free subtree must resolve to its top-level
	// ancestor, at any depth. `cache clear` resolving to "clear" instead of
	// "cache" is exactly the bug this guards.
	for _, top := range root.Commands {
		want := top.Name
		forEachCommand(top, func(c *cli.Command) {
			if got := topLevelCommandNameIn(root, c); got != want {
				t.Errorf("topLevelCommandNameIn(%q) = %q, want %q", c.Name, got, want)
			}
		})
	}

	// The root itself has no top-level ancestor.
	if got := topLevelCommandNameIn(root, root); got != "" {
		t.Errorf("topLevelCommandNameIn(root) = %q, want \"\"", got)
	}

	// A command outside the tree falls back to its own name rather than
	// silently claiming to be the root.
	orphan := &cli.Command{Name: "orphan"}
	if got := topLevelCommandNameIn(root, orphan); got != "orphan" {
		t.Errorf("topLevelCommandNameIn(orphan) = %q, want \"orphan\"", got)
	}

	// The nested forms people actually type must land on a plugin-free
	// top-level command.
	for _, tc := range []struct{ parent, child string }{
		{"pack", "manifest"},
		{"pack", "docs"},
		{"cache", "clear"},
	} {
		leaf := findCommand(root, tc.child)
		if leaf == nil {
			t.Errorf("expected the command tree to contain %s %s", tc.parent, tc.child)
			continue
		}
		if got := topLevelCommandNameIn(root, leaf); got != tc.parent {
			t.Errorf("topLevelCommandNameIn(%s) = %q, want %q", tc.child, got, tc.parent)
		}
		if !pluginFreeCommands[tc.parent] {
			t.Errorf("expected %s to be plugin-free", tc.parent)
		}
	}

	// help and the root run command both resolve --package sources, so neither
	// may be plugin-free.
	if pluginFreeCommands[""] {
		t.Error("the root command must allow plugin discovery")
	}
	if help := findCommand(root, "help"); help == nil {
		t.Error("expected a help command")
	} else if pluginFreeCommands[topLevelCommandNameIn(root, help)] {
		t.Error("help resolves --package sources, so it must allow plugin discovery")
	}
}

// TestPluginDiscoveryThroughExecute drives the real command tree with real
// argv and records what PreRun decided, which is the only way to catch the
// leaf-vs-ancestor mistake: PreRun receives `clear`, not `cache`.
func TestPluginDiscoveryThroughExecute(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "manifest.toml"), []byte("name = \"crpkg\"\nversion = \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "s.py")
	if err := os.WriteFile(script, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"bare run", []string{"scriptling", "-c", "pass"}, true},
		{"help topic", []string{"scriptling", "help", "os"}, true},
		{"lint", []string{"scriptling", "--lint", script}, false},
		{"list-libs", []string{"scriptling", "--list-libs"}, false},
		{"pack bare", []string{"scriptling", "pack"}, false},
		{"pack manifest", []string{"scriptling", "pack", "manifest", srcDir}, false},
		{"pack docs", []string{"scriptling", "pack", "docs", srcDir}, false},
		{"cache bare", []string{"scriptling", "cache"}, false},
		{"cache clear", []string{"scriptling", "cache", "clear"}, false},
		{"unknown cache subcommand", []string{"scriptling", "cache", "nope"}, false},
		{"unpack", []string{"scriptling", "unpack", filepath.Join(dir, "x.zip")}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := buildRootCommand()

			// Replace Run everywhere so nothing actually executes, and stub
			// PreRun to record only the discovery decision.
			var got, recorded bool
			forEachCommand(root, func(c *cli.Command) {
				c.Run = func(ctx context.Context, cmd *cli.Command) error { return nil }
			})
			root.PreRun = func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
				got = pluginDiscoveryWanted(cmd)
				recorded = true
				return ctx, nil
			}
			root.PostRun = nil

			os.Args = tc.args
			_ = root.Execute(context.Background())

			// A command that fails argument validation never reaches PreRun,
			// which also means no plugins start — that satisfies want=false.
			if tc.want && !recorded {
				t.Fatalf("PreRun did not run for %v, so plugins would never load", tc.args)
			}
			if recorded && got != tc.want {
				t.Errorf("pluginDiscoveryWanted for %v = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

// findCommand returns the first command with the given name in the tree.
func findCommand(root *cli.Command, name string) *cli.Command {
	var found *cli.Command
	forEachCommand(root, func(c *cli.Command) {
		if found == nil && c != root && c.Name == name {
			found = c
		}
	})
	return found
}

// forEachCommand visits cmd and every descendant.
func forEachCommand(cmd *cli.Command, fn func(*cli.Command)) {
	fn(cmd)
	for _, child := range cmd.Commands {
		forEachCommand(child, fn)
	}
}

// TestLoadPluginManagerAlwaysReturnsAManager locks in the behaviour that
// scriptling.plugin.load() depends on: with nothing configured the manager
// still exists (and starts no processes), so the library can be registered.
func TestLoadPluginManagerAlwaysReturnsAManager(t *testing.T) {
	manager, err := loadPluginManager(context.Background(), nil, nil, nil, nil, false)
	if err != nil {
		t.Fatalf("loadPluginManager with no plugins: %v", err)
	}
	if manager == nil {
		t.Fatal("expected a manager even with no plugins configured")
	}
	defer manager.Close()
	if got := manager.List(); len(got) != 0 {
		t.Fatalf("expected no plugins loaded, got %d", len(got))
	}
}

// TestLoadPluginManagerRejectsBadPluginArgs checks argument resolution failures
// surface before any process is started.
func TestLoadPluginManagerRejectsBadPluginArgs(t *testing.T) {
	_, err := loadPluginManager(context.Background(), nil, nil, []string{"orphan-arg"}, nil, false)
	if err == nil {
		t.Fatal("expected an error for a --plugin-arg with no --plugin")
	}
	if !strings.Contains(err.Error(), "without any --plugin") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPluginLoggerWiredAfterStartup guards an ordering trap in PreRun: plugins
// must start before --package sources are opened (so fetcher schemes exist),
// but the logger cannot be built until the app bundle is known (it decides
// whether logs go to stderr). Plugins are therefore created with no logger, and
// everything they log is dropped unless the logger is wired in afterwards.
func TestPluginLoggerWiredAfterStartup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell plugin helper is unix-only")
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "logging-plugin")
	// Handshake, then on every call emit a host.log notification and answer
	// with the caller's own id so the call returns promptly.
	script := `#!/bin/sh
read req
echo '{"jsonrpc":"2.0","id":1,"result":{"protocol":"1.0","transport":"json","library":{"name":"logger-plug","version":"1.0.0","description":"logs"},"capabilities":[],"schema":{"functions":[{"name":"noisy"}],"classes":[],"constants":[]}}}'
while read req; do
  id=$(printf '%s' "$req" | sed -n 's/.*"id":\([0-9]*\).*/\1/p')
  echo '{"jsonrpc":"2.0","method":"host.log","params":{"level":"info","message":"plugin log reached the host"}}'
  echo "{\"jsonrpc\":\"2.0\",\"id\":${id:-1},\"result\":{\"type\":\"string\",\"value\":\"ok\"}}"
done
`
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatalf("write helper: %v", err)
	}

	// Start the plugin with NO logger, exactly as PreRun does.
	previousLogger := globalLogger
	globalLogger = nil
	manager, err := loadPluginManager(context.Background(), []string{dir}, nil, nil, nil, false)
	globalLogger = previousLogger
	if err != nil {
		t.Fatalf("loadPluginManager: %v", err)
	}
	defer manager.Close()

	client, ok := manager.Get("plugin.logger-plug")
	if !ok {
		t.Fatal("expected the plugin to load")
	}

	// Now wire the logger in, which is the step PreRun must not forget.
	logs := &cliCaptureLogger{}
	manager.SetLogger(logs)

	if _, err := client.CallFunction(context.Background(), "noisy", nil, nil); err != nil {
		t.Fatalf("CallFunction: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, entry := range logs.snapshot() {
			if strings.Contains(entry.msg, "plugin log reached the host") {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("plugin log records were dropped; the host logger was never wired to the manager. got %#v", logs.snapshot())
}
