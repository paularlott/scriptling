package extlibs

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// bgTestWriter is a goroutine-safe output writer for capturing print output.
type bgTestWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *bgTestWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *bgTestWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// bgTestLogger is a logger.Logger that records "LEVEL msg" lines.
type bgTestLogger struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *bgTestLogger) log(level, msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(&l.buf, "%s %s\n", level, msg)
}

func (l *bgTestLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func (l *bgTestLogger) Trace(msg string, kv ...any) { l.log("TRACE", msg, kv...) }
func (l *bgTestLogger) Debug(msg string, kv ...any) { l.log("DEBUG", msg, kv...) }
func (l *bgTestLogger) Info(msg string, kv ...any)  { l.log("INFO", msg, kv...) }
func (l *bgTestLogger) Warn(msg string, kv ...any)  { l.log("WARN", msg, kv...) }
func (l *bgTestLogger) Error(msg string, kv ...any) { l.log("ERROR", msg, kv...) }
func (l *bgTestLogger) Fatal(msg string, kv ...any) { l.log("FATAL", msg, kv...) }

func (l *bgTestLogger) With(string, any) logger.Logger { return l }
func (l *bgTestLogger) WithError(error) logger.Logger  { return l }
func (l *bgTestLogger) WithGroup(string) logger.Logger { return l }

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestBackgroundWithoutFactoryRunsAndLogs is the regression test for the
// silent-swallow bug: in an embedded host that never calls
// SetBackgroundFactory/ReleaseBackgroundTasks, runtime.background() used to
// return null and queue the task forever — no output, no logging, no error.
// The task must now run immediately, with print output reaching the caller's
// output writer and logging reaching the host logger, exactly as the same
// calls do in the calling script.
func TestBackgroundWithoutFactoryRunsAndLogs(t *testing.T) {
	ResetRuntime()

	out := &bgTestWriter{}
	logs := &bgTestLogger{}

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterLoggingLibrary(p, logs)
	p.SetOutputWriter(out)

	result, err := p.Eval(`
import scriptling.runtime as runtime
import logging

def task(items):
    print("BG-PRINT: running")
    logging.info("BG-LOGINFO: running")
    logging.error("BG-LOGERROR: running")
    logger = logging.getLogger("bg")
    logger.info("BG-NAMED-INFO: running")
    items.append("mutated-by-task")
    return "done"

caller_items = ["original"]
logging.info("MAIN-LOGINFO: running")
print("MAIN-PRINT: running")
p = runtime.background("t1", "task", caller_items)
p.get()
`)

	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "done" {
		t.Fatalf("expected background result \"done\", got %v", result)
	}

	for _, line := range []string{
		"INFO BG-LOGINFO: running",
		"ERROR BG-LOGERROR: running",
		"INFO BG-NAMED-INFO: running",
		"INFO MAIN-LOGINFO: running",
	} {
		if !strings.Contains(logs.String(), line) {
			t.Errorf("host logger missing task log line %q; got:\n%s", line, logs.String())
		}
	}

	captured := out.String()
	for _, line := range []string{"MAIN-PRINT: running", "BG-PRINT: running"} {
		if !strings.Contains(captured, line) {
			t.Errorf("captured output missing %q; got:\n%s", line, captured)
		}
	}

	// Args are deep-copied: the task's mutation must not leak back.
	if len(callerItems(t, p)) != 1 || callerItems(t, p)[0] != "original" {
		t.Errorf("task mutated the caller's argument list: %v", callerItems(t, p))
	}
}

func callerItems(t *testing.T, p *scriptling.Scriptling) []string {
	t.Helper()
	obj, err := p.GetVarAsList("caller_items")
	if err != nil {
		t.Fatalf("caller_items: %v", err)
	}
	out := make([]string, 0, len(obj))
	for _, v := range obj {
		s, e := v.AsString()
		if e != nil {
			t.Fatalf("caller_items contains non-string: %v", v)
		}
		out = append(out, s)
	}
	return out
}

// TestBackgroundWithoutFactoryImportInsideTask covers the delegated import
// path: a task that imports a library for the first time (the caller never
// imported it) must get the caller host's registration — here, the custom
// logger — not a silent failure or a default sink.
func TestBackgroundWithoutFactoryImportInsideTask(t *testing.T) {
	ResetRuntime()

	logs := &bgTestLogger{}

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterLoggingLibrary(p, logs)

	result, err := p.Eval(`
import scriptling.runtime as runtime

def task():
    import logging
    logging.info("BG-IMPORTED-LOGINFO: running")
    return "done"

p = runtime.background("t1", "task")
p.get()
`)

	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "done" {
		t.Fatalf("expected background result \"done\", got %v", result)
	}
	if !strings.Contains(logs.String(), "INFO BG-IMPORTED-LOGINFO: running") {
		t.Errorf("host logger missing task log line; got:\n%s", logs.String())
	}
}

// TestBackgroundWithoutFactoryIsolation checks the derived task environment
// stays isolated: non-callable caller globals are not visible to the task.
func TestBackgroundWithoutFactoryIsolation(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterLoggingLibraryDefault(p)

	_, err := p.Eval(`
import scriptling.runtime as runtime

x = 42

def task():
    return x

p = runtime.background("t1", "task")
p.get()
`)

	if err == nil {
		t.Fatal("expected the task to fail on the missing global, got success")
	}
	if !strings.Contains(err.Error(), "identifier not found: x") {
		t.Errorf("expected error about identifier x, got: %v", err)
	}
}

// TestBackgroundQueueUntilReleaseWithFactory pins the server-bootstrap
// contract: when a factory IS configured and tasks are not yet released,
// background() queues them (returning null) and ReleaseBackgroundTasks()
// starts them.
func TestBackgroundQueueUntilReleaseWithFactory(t *testing.T) {
	ResetRuntime()

	logs := &bgTestLogger{}

	SetBackgroundFactory(func() SandboxInstance {
		p2 := scriptling.New()
		RegisterRuntimeLibraryAll(p2, nil)
		RegisterLoggingLibrary(p2, logs)
		return p2
	})

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterLoggingLibrary(p, logs)

	result, err := p.Eval(`
import scriptling.runtime as runtime
import logging

def task():
    logging.info("BG-QUEUED-LOGINFO: running")
    print("BG-QUEUED-PRINT: running")
    return "done"

result = runtime.background("t1", "task")
`)

	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if _, ok := result.(*object.Null); !ok {
		t.Fatalf("expected null while tasks are queued, got %v", result)
	}

	RuntimeState.RLock()
	queued := len(RuntimeState.Backgrounds)
	RuntimeState.RUnlock()
	if queued != 1 {
		t.Fatalf("expected 1 queued task, got %d", queued)
	}

	ReleaseBackgroundTasks()

	waitFor(t, "queued task to log", func() bool {
		return strings.Contains(logs.String(), "INFO BG-QUEUED-LOGINFO: running")
	})
}

// TestBackgroundSharedWithoutFactory makes sure the shared=True path (which
// never needed a factory) still logs through the host logger.
func TestBackgroundSharedWithoutFactory(t *testing.T) {
	ResetRuntime()

	logs := &bgTestLogger{}

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterLoggingLibrary(p, logs)

	result, err := p.Eval(`
import scriptling.runtime as runtime
import logging

def task():
    logging.info("BG-SHARED-LOGINFO: running")
    return "done"

p = runtime.background("t1", "task", shared=True)
p.get()
`)

	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "done" {
		t.Fatalf("expected background result \"done\", got %v", result)
	}
	if !strings.Contains(logs.String(), "INFO BG-SHARED-LOGINFO: running") {
		t.Errorf("host logger missing shared-task log line; got:\n%s", logs.String())
	}
}

// TestBackgroundWaitCompletesFireAndForgetTasks pins the CLI reporter's bug:
// a background() call that is never awaited used to be killed when the host
// finished running the script (the CLI process exited mid-task), silently
// swallowing everything the task did — logging included. Hosts call
// WaitBackgroundTasks() before exiting; afterwards every non-daemon task must
// have completed.
func TestBackgroundWaitCompletesFireAndForgetTasks(t *testing.T) {
	ResetRuntime()

	logs := &bgTestLogger{}

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRuntimeLibraryAll(p, nil)
	RegisterLoggingLibrary(p, logs)

	_, err := p.Eval(`
import scriptling.runtime as runtime
import time
import logging

def task():
    time.sleep(0.2)
    logging.info("FF-LOGINFO: finished")
    return "done"

pr = runtime.background("ff", "task")
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}

	WaitBackgroundTasks()

	if !strings.Contains(logs.String(), "INFO FF-LOGINFO: finished") {
		t.Errorf("fire-and-forget task did not complete before WaitBackgroundTasks returned; got:\n%s", logs.String())
	}

	result, err := p.Eval("pr.get()")
	if err != nil {
		t.Fatalf("promise get error: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "done" {
		t.Fatalf("expected resolved promise \"done\", got %v", result)
	}
}

// TestBackgroundDaemonTasksNotWaitedFor checks that daemon=True tasks do not
// block WaitBackgroundTasks, and that the daemon control kwarg is not
// forwarded to the handler (other kwargs still are).
func TestBackgroundDaemonTasksNotWaitedFor(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRuntimeLibraryAll(p, nil)

	_, err := p.Eval(`
import scriptling.runtime as runtime
import time

def slow(**kw):
    time.sleep(1)
    return len(kw)

pr = runtime.background("d1", "slow", daemon=True)
pr2 = runtime.background("d2", "slow", daemon=True, x=1)
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}

	start := time.Now()
	WaitBackgroundTasks()
	if elapsed := time.Since(start); elapsed > 700*time.Millisecond {
		t.Fatalf("WaitBackgroundTasks blocked on daemon tasks: %v", elapsed)
	}

	result, err := p.Eval("pr2.get()")
	if err != nil {
		t.Fatalf("promise get error: %v", err)
	}
	if n, ok := result.(*object.Integer); !ok || n.IntValue() != 1 {
		t.Fatalf("expected handler to see only the forwarded kwarg (len(kw)==1), got %v", result)
	}
}
