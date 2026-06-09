package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/lint"
	"github.com/paularlott/scriptling/scriptling-cli/bootstrap"
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

	manager, err := loadPluginManager(context.Background(), []string{dir})
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
