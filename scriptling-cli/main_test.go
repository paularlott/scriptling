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

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/lint"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/bootstrap"
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

	manager, err := loadPluginManager(context.Background(), []string{dir}, nil)
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

func TestSplitPluginSpec(t *testing.T) {
	cases := []struct {
		spec  string
		path  string
		args  []string
		fails bool
	}{
		{spec: "/usr/local/bin/knot", path: "/usr/local/bin/knot"},
		{spec: "/usr/local/bin/knot scriptling-server", path: "/usr/local/bin/knot", args: []string{"scriptling-server"}},
		{spec: "/usr/local/bin/knot scriptling-server --alias testing", path: "/usr/local/bin/knot", args: []string{"scriptling-server", "--alias", "testing"}},
		{spec: "'/opt/knot dir/knot' serve", path: "/opt/knot dir/knot", args: []string{"serve"}},
		{spec: `"/opt/knot dir/knot" serve`, path: "/opt/knot dir/knot", args: []string{"serve"}},
		{spec: `/opt/knot\ dir/knot serve`, path: "/opt/knot dir/knot", args: []string{"serve"}},
		{spec: "'unterminated", fails: true},
		{spec: "", fails: true},
		{spec: "   ", fails: true},
	}
	for _, tc := range cases {
		path, args, err := splitPluginSpec(tc.spec)
		if tc.fails {
			if err == nil {
				t.Errorf("splitPluginSpec(%q) expected an error", tc.spec)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitPluginSpec(%q): %v", tc.spec, err)
			continue
		}
		if path != tc.path {
			t.Errorf("splitPluginSpec(%q) path = %q, want %q", tc.spec, path, tc.path)
		}
		if len(args) != len(tc.args) {
			t.Errorf("splitPluginSpec(%q) args = %v, want %v", tc.spec, args, tc.args)
			continue
		}
		for i := range args {
			if args[i] != tc.args[i] {
				t.Errorf("splitPluginSpec(%q) args = %v, want %v", tc.spec, args, tc.args)
				break
			}
		}
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

	manager, err := loadPluginManager(context.Background(), nil, []string{helper + " --alias testing"})
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
	manager, err := loadPluginManager(context.Background(), []string{pluginDir}, []string{helper + " --alias explicit"})
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

// TestResolveScriptFileStagesSchemeSources covers the server-mode setup
// script path: a scheme source is fetched from its plugin and staged to a
// local file; a plain path passes through untouched.
func TestResolveScriptFileStagesSchemeSources(t *testing.T) {
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
	pluginSrv.DeclarePackage("ppmain://libs")
	httpSrv := httptest.NewServer(pluginSrv)
	t.Cleanup(httpSrv.Close)

	manager := scriptlingplugin.NewManager(nil)
	t.Cleanup(func() { _ = manager.Close() })
	if _, err := manager.LoadURL(context.Background(), "ppmainplugin", httpSrv.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	if err := pluginpack.Register(manager); err != nil {
		t.Fatalf("pluginpack.Register: %v", err)
	}

	// Plain paths pass through unchanged.
	got, err := resolveScriptFile(localScript)
	if err != nil {
		t.Fatalf("resolveScriptFile(local): %v", err)
	}
	if got != localScript {
		t.Fatalf("expected local path passthrough, got %s", got)
	}

	// Scheme sources are fetched (always fresh) and staged locally.
	staged, err := resolveScriptFile("ppmain://scripts/setup")
	if err != nil {
		t.Fatalf("resolveScriptFile(scheme): %v", err)
	}
	if staged == localScript || !strings.HasSuffix(staged, ".py") {
		t.Fatalf("expected a staged .py file, got %s", staged)
	}
	content, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("read staged file: %v", err)
	}
	if !strings.Contains(string(content), "runtime.jsonrpc") && !strings.Contains(string(content), "runtime") {
		t.Fatalf("unexpected staged content: %q", content)
	}
}

type stagingFetcher struct {
	files   map[string]string
	scripts map[string]string
}

func (f *stagingFetcher) Read(ctx context.Context, source, path, etag, lastModified string) (scriptlingplugin.FetchResult, error) {
	if path == "" {
		content, ok := f.scripts[source]
		if !ok {
			return scriptlingplugin.FetchResult{}, fmt.Errorf("%w: %s", scriptlingplugin.ErrFetchNotFound, source)
		}
		return scriptlingplugin.FetchResult{Data: []byte(content), ETag: "ppmain-v1"}, nil
	}
	content, ok := f.files[path]
	if !ok {
		return scriptlingplugin.FetchResult{}, fmt.Errorf("%w: %s", scriptlingplugin.ErrFetchNotFound, path)
	}
	return scriptlingplugin.FetchResult{Data: []byte(content), ETag: "ppmain-v1"}, nil
}

func (f *stagingFetcher) List(ctx context.Context, source, path string) ([]scriptlingplugin.FetchEntry, error) {
	if path == "" || path == "." {
		return []scriptlingplugin.FetchEntry{{Name: "manifest.toml"}}, nil
	}
	return nil, fmt.Errorf("%w: %s", scriptlingplugin.ErrFetchNotFound, path)
}
