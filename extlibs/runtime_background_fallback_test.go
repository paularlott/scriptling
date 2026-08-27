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
// stays isolated for mutable data: module-level lists (and dicts, instances)
// are not visible to the task, while scalar constants are.
func TestBackgroundWithoutFactoryIsolation(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterLoggingLibraryDefault(p)

	result, err := p.Eval(`
import scriptling.runtime as runtime

caller_items = ["a", "b"]

def task():
    return caller_items

p = runtime.background("t1", "task")
p.get()
`)

	// The failure surfaces either as a raised script error or as the trailing
	// promise value — an Error object.
	msg := ""
	if err != nil {
		msg = err.Error()
	} else if errObj, ok := result.(*object.Error); ok {
		msg = errObj.Message
	} else {
		t.Fatalf("expected the task to fail on the missing mutable global, got %v", result)
	}
	if !strings.Contains(msg, "identifier not found: caller_items") {
		t.Errorf("expected error about identifier items, got: %s", msg)
	}
}

// TestBackgroundQueueUntilReleaseWithFactory pins the server-bootstrap
// contract: when a factory IS configured and tasks are not yet released,
// background() queues the task and hands back a promise, and
// ReleaseBackgroundTasks() starts it so that promise resolves.
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

runtime.background("t1", "task")
`)

	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	// The queued task hands back a promise, not null, so the setup script can
	// await it once the host releases tasks.
	if _, ok := result.(*object.Builtin); !ok {
		t.Fatalf("expected a promise while tasks are queued, got %T", result)
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

// TestBackgroundDuplicateNameDoesNotStartSecondTask pins the task-name
// identity rule. A module that registers both a request handler and a
// background task is re-executed every time it is imported to serve a
// request, so a repeated background() call with a live name must start
// nothing and hand back the running task's promise.
func TestBackgroundDuplicateNameDoesNotStartSecondTask(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	// The task blocks on a named queue, so it is guaranteed to still be running
	// when the duplicate registrations happen — it cannot finish until the gate
	// is fed, which happens after all three calls. The gate is then fed once
	// per call so that a regression (three tasks actually starting) fails on
	// the run count below rather than deadlocking on an empty queue.
	result, err := p.Eval(`
import scriptling.runtime as runtime

runtime.sync.Atomic("dup_runs", initial=0)
runtime.sync.Queue("dup_gate")

def task():
    runtime.sync.Atomic("dup_runs", initial=0).add(1)
    runtime.sync.Queue("dup_gate").get()
    return "done"

first = runtime.background("dup", "task")
second = runtime.background("dup", "task")
third = runtime.background("dup", "task")
gate = runtime.sync.Queue("dup_gate")
gate.put(1)
gate.put(1)
gate.put(1)
[first.get(), second.get(), third.get()]
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}

	// All three calls observe the one task's result.
	list, ok := result.(*object.List)
	if !ok || len(list.Elements) != 3 {
		t.Fatalf("expected three awaited results, got %v", result)
	}
	for i, el := range list.Elements {
		s, ok := el.(*object.String)
		if !ok || s.StringValue() != "done" {
			t.Fatalf("promise %d resolved to %v, want \"done\"", i, el)
		}
	}

	RuntimeState.RLock()
	runs, runsOK := RuntimeState.Atomics["dup_runs"]
	RuntimeState.RUnlock()
	if !runsOK {
		t.Fatal("counter atomic missing")
	}
	if got := runs.Value(); got != 1 {
		t.Fatalf("handler ran %d times, want 1 — a duplicate name started another task", got)
	}
}

// TestBackgroundDuplicateNameWhileQueued covers the server-mode shape of the
// duplicate-name rule, which is where it actually bites: the setup script runs
// before tasks are released, so both calls land in the queue maps. Those maps
// are keyed by name, so the second registration used to overwrite the first —
// two tasks queued, one silently discarded.
func TestBackgroundDuplicateNameWhileQueued(t *testing.T) {
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

	if _, err := p.Eval(`
import scriptling.runtime as runtime

runtime.sync.Atomic("queued_runs", initial=0)

def task():
    runtime.sync.Atomic("queued_runs", initial=0).add(1)
    return "done"

first = runtime.background("queued_dup", "task")
second = runtime.background("queued_dup", "task")
`); err != nil {
		t.Fatalf("script error: %v", err)
	}

	RuntimeState.RLock()
	queued := len(RuntimeState.Backgrounds)
	active := len(RuntimeState.ActiveTasks)
	RuntimeState.RUnlock()
	if queued != 1 {
		t.Fatalf("expected 1 queued task, got %d", queued)
	}
	if active != 1 {
		t.Fatalf("expected 1 active task name, got %d", active)
	}

	ReleaseBackgroundTasks()

	// Both calls must hand back the same live promise. The queue maps are keyed
	// by name, so without the name claim the second registration replaces the
	// first and only the surviving promise is ever resolved — leaving the first
	// caller's promise orphaned and any .get() on it blocked forever. Awaited
	// off the main goroutine so that regression fails on the timeout below
	// instead of hanging the suite.
	type awaitResult struct {
		value object.Object
		err   error
	}
	awaited := make(chan awaitResult, 1)
	go func() {
		value, err := p.Eval(`[first.get(), second.get()]`)
		awaited <- awaitResult{value, err}
	}()
	select {
	case got := <-awaited:
		if got.err != nil {
			t.Fatalf("awaiting both promises failed: %v", got.err)
		}
		list, ok := got.value.(*object.List)
		if !ok || len(list.Elements) != 2 {
			t.Fatalf("expected two awaited results, got %v", got.value)
		}
		for i, el := range list.Elements {
			if s, ok := el.(*object.String); !ok || s.StringValue() != "done" {
				t.Fatalf("promise %d resolved to %v, want \"done\"", i, el)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a promise handed to the script never resolved — both background() calls must share one promise")
	}

	RuntimeState.RLock()
	runs, ok := RuntimeState.Atomics["queued_runs"]
	RuntimeState.RUnlock()
	if !ok {
		t.Fatal("counter atomic missing")
	}
	if got := runs.Value(); got != 1 {
		t.Fatalf("handler ran %d times, want 1", got)
	}
}

// TestQueuedTaskThatCannotStartResolvesPromise pins the anti-hang guarantee.
// A queued task is validated at release, not at registration, so a handler that
// is missing from the environment it was queued with fails there — after the
// setup script has already been handed the promise. That promise must resolve
// with the error instead of leaving an awaiting script blocked forever.
func TestQueuedTaskThatCannotStartResolvesPromise(t *testing.T) {
	ResetRuntime()

	var mu sync.Mutex
	var reported []string
	SetTaskErrorLogger(func(name string, err error) {
		mu.Lock()
		defer mu.Unlock()
		reported = append(reported, name+": "+err.Error())
	})
	defer SetTaskErrorLogger(nil)

	SetBackgroundFactory(func() SandboxInstance {
		p2 := scriptling.New()
		RegisterRuntimeLibraryAll(p2, nil)
		return p2
	})

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	if _, err := p.Eval(`
import scriptling.runtime as runtime
pending = runtime.background("ghost_queued", "no_such_function")
`); err != nil {
		t.Fatalf("script error: %v", err)
	}

	RuntimeState.RLock()
	promise := RuntimeState.BackgroundPromises["ghost_queued"]
	RuntimeState.RUnlock()
	if promise == nil {
		t.Fatal("expected the queued task to have a promise")
	}

	ReleaseBackgroundTasks()

	resolved := make(chan error, 1)
	go func() {
		_, err := promise.get()
		resolved <- err
	}()
	select {
	case err := <-resolved:
		if err == nil {
			t.Fatal("expected the promise to resolve with an error")
		}
		if !strings.Contains(err.Error(), "function not found") {
			t.Fatalf("unexpected promise error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("promise never resolved — a script awaiting this task would hang forever")
	}

	waitFor(t, "the failure to be reported", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(reported) == 1 && strings.Contains(reported[0], "ghost_queued")
	})

	// The name must not stay claimed by a task that never ran.
	RuntimeState.RLock()
	_, held := RuntimeState.ActiveTasks["ghost_queued"]
	RuntimeState.RUnlock()
	if held {
		t.Fatal("name stayed claimed after the queued task failed to start")
	}
}

// TestBackgroundDuplicateNameShared covers the shared=True path, which starts
// its task on a different code path from the isolated one and so needs its own
// check that the name is claimed and released.
func TestBackgroundDuplicateNameShared(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	// Shared tasks run in the caller's environment, so the counter is directly
	// visible. The gate is fed once per call so a regression fails on the count
	// rather than deadlocking.
	result, err := p.Eval(`
import scriptling.runtime as runtime

runs = runtime.sync.Atomic("shared_runs", initial=0)
gate = runtime.sync.Queue("shared_gate")

def task():
    runs.add(1)
    gate.get()
    return "done"

first = runtime.background("shared_dup", "task", shared=True)
second = runtime.background("shared_dup", "task", shared=True)
gate.put(1)
gate.put(1)
[first.get(), second.get()]
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok || len(list.Elements) != 2 {
		t.Fatalf("expected two awaited results, got %v", result)
	}

	RuntimeState.RLock()
	runs, runsOK := RuntimeState.Atomics["shared_runs"]
	active := len(RuntimeState.ActiveTasks)
	RuntimeState.RUnlock()
	if !runsOK {
		t.Fatal("counter atomic missing")
	}
	if got := runs.Value(); got != 1 {
		t.Fatalf("shared handler ran %d times, want 1", got)
	}
	if active != 0 {
		t.Fatalf("expected the shared task's name to be released, got %d active", active)
	}
}

// TestBackgroundSharedNameFreedWhenHandlerMissing checks the shared path also
// gives the name back when it rejects the registration.
func TestBackgroundSharedNameFreedWhenHandlerMissing(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	result, err := p.Eval(`
import scriptling.runtime as runtime
runtime.background("shared_ghost", "no_such_function", shared=True)
`)
	if err == nil {
		if errObj, ok := result.(*object.Error); !ok {
			t.Fatalf("expected an error for a missing shared handler, got %v", result)
		} else if !strings.Contains(errObj.Message, "function not found") {
			t.Fatalf("unexpected error: %s", errObj.Message)
		}
	} else if !strings.Contains(err.Error(), "function not found") {
		t.Fatalf("unexpected error: %v", err)
	}

	RuntimeState.RLock()
	_, held := RuntimeState.ActiveTasks["shared_ghost"]
	RuntimeState.RUnlock()
	if held {
		t.Fatal("name stayed claimed after the shared task was rejected")
	}
}

// TestBackgroundNameReusableAfterTaskEnds checks the name is only held for as
// long as the task lives, so a later task may reuse it.
func TestBackgroundNameReusableAfterTaskEnds(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	_, err := p.Eval(`
import scriptling.runtime as runtime

runtime.sync.Atomic("reuse_runs", initial=0)

def task():
    runtime.sync.Atomic("reuse_runs", initial=0).add(1)
    return "done"

runtime.background("reused", "task").get()
runtime.background("reused", "task").get()
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}

	RuntimeState.RLock()
	runs, ok := RuntimeState.Atomics["reuse_runs"]
	RuntimeState.RUnlock()
	if !ok {
		t.Fatal("counter atomic missing")
	}
	if got := runs.Value(); got != 2 {
		t.Fatalf("handler ran %d times, want 2 — the name was not freed after the task ended", got)
	}

	RuntimeState.RLock()
	active := len(RuntimeState.ActiveTasks)
	RuntimeState.RUnlock()
	if active != 0 {
		t.Fatalf("expected no active tasks after both finished, got %d", active)
	}
}

// TestBackgroundNameFreedWhenArgsRejected checks a call rejected for an
// untransferable argument gives the name back, so a corrected call with the
// same name still runs instead of being mistaken for a duplicate.
func TestBackgroundNameFreedWhenArgsRejected(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	result, err := p.Eval(`
import scriptling.runtime as runtime

runtime.sync.Atomic("retry_runs", initial=0)

def task(value):
    runtime.sync.Atomic("retry_runs", initial=0).add(1)
    return value

def not_transferable():
    return 1

failed = runtime.background("retried", "task", not_transferable)
`)
	if err == nil {
		if _, ok := result.(*object.Error); !ok {
			t.Fatalf("expected an error for an untransferable arg, got %v", result)
		}
	} else if !strings.Contains(err.Error(), "not transferable") {
		t.Fatalf("unexpected error: %v", err)
	}

	RuntimeState.RLock()
	_, held := RuntimeState.ActiveTasks["retried"]
	RuntimeState.RUnlock()
	if held {
		t.Fatal("name stayed claimed after the call was rejected")
	}

	// The same name must still be usable.
	retried, err := p.Eval(`
import scriptling.runtime as runtime

def task(value):
    runtime.sync.Atomic("retry_runs", initial=0).add(1)
    return value

runtime.background("retried", "task", "ok").get()
`)
	if err != nil {
		t.Fatalf("retry script error: %v", err)
	}
	if s, ok := retried.(*object.String); !ok || s.StringValue() != "ok" {
		t.Fatalf("expected the retried task to run and return \"ok\", got %v", retried)
	}
}

// TestBackgroundNameFreedWhenTaskCannotStart checks a rejected registration
// does not leave its name claimed.
func TestBackgroundNameFreedWhenTaskCannotStart(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	result, err := p.Eval(`
import scriptling.runtime as runtime
runtime.background("ghost", "no_such_function")
`)
	if err == nil {
		if errObj, ok := result.(*object.Error); !ok {
			t.Fatalf("expected an error for a missing handler, got %v", result)
		} else if !strings.Contains(errObj.Message, "function not found") {
			t.Fatalf("unexpected error: %s", errObj.Message)
		}
	} else if !strings.Contains(err.Error(), "function not found") {
		t.Fatalf("unexpected error: %v", err)
	}

	RuntimeState.RLock()
	_, held := RuntimeState.ActiveTasks["ghost"]
	RuntimeState.RUnlock()
	if held {
		t.Fatal("name stayed claimed after the task failed to start")
	}
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

// TestBackgroundTaskSeesModuleScalars checks the fallback (factory-free) task
// environment receives module-level constants of every scalar type the
// snapshot claims to carry: int, float, str, bool and None.
func TestBackgroundTaskSeesModuleScalars(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	result, err := p.Eval(`
import scriptling.runtime as runtime

TICKS = 3
LABEL = "tick"
SCALE = 1.5
ENABLED = True
UNSET = None

def task():
    out = ""
    for i in range(TICKS):
        out = out + LABEL + str(i)
    out = out + "|" + str(SCALE)
    if ENABLED:
        out = out + "|enabled"
    if UNSET == None:
        out = out + "|unset"
    return out

pr = runtime.background("scalars", "task")
pr.get()
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	const want = "tick0tick1tick2|1.5|enabled|unset"
	if s, ok := result.(*object.String); !ok || s.StringValue() != want {
		t.Fatalf("expected task to read module constants (%q), got %v", want, result)
	}
}

// TestBackgroundNameReusableAfterTaskFails pins that a FAILED task releases
// its name too — a restart loop that re-registers the same name after a crash
// must be able to.
func TestBackgroundNameReusableAfterTaskFails(t *testing.T) {
	ResetRuntime()
	SetTaskErrorLogger(func(string, error) {}) // the first run fails on purpose
	defer SetTaskErrorLogger(nil)

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	result, err := p.Eval(`
import scriptling.runtime as runtime

runtime.sync.Atomic("boom_runs", initial=0)

def failer():
    runtime.sync.Atomic("boom_runs", initial=0).add(1)
    raise Exception("kaboom")

def retry():
    runtime.sync.Atomic("boom_runs", initial=0).add(1)
    return "ok"

try:
    runtime.background("boom", "failer").get()
except Exception as error:
    pass
runtime.background("boom", "retry").get()
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "ok" {
		t.Fatalf("expected the relaunched task to return \"ok\", got %v", result)
	}

	RuntimeState.RLock()
	runs, _ := RuntimeState.Atomics["boom_runs"]
	active := len(RuntimeState.ActiveTasks)
	RuntimeState.RUnlock()
	if runs.Value() != 2 {
		t.Fatalf("handlers ran %d times, want 2 — the name was not reusable after failure", runs.Value())
	}
	if active != 0 {
		t.Fatalf("expected no active tasks after both finished, got %d", active)
	}
}

// TestBackgroundConcurrentDuplicateClaims has two concurrent shared tasks
// register the same inner task name. Exactly one inner task may start,
// whichever side wins the claim.
func TestBackgroundConcurrentDuplicateClaims(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	p.RegisterLibrary(stdlib.TimeLibrary)

	result, err := p.Eval(`
import time
import scriptling.runtime as runtime

runtime.sync.Atomic("raced_runs", initial=0)

def inner():
    time.sleep(0.2)
    runtime.sync.Atomic("raced_runs", initial=0).add(1)
    return "once"

def registrar_a():
    return runtime.background("raced", "inner").get()

def registrar_b():
    return runtime.background("raced", "inner").get()

ra = runtime.background("ra", "registrar_a", shared=True)
rb = runtime.background("rb", "registrar_b", shared=True)
[ra.get(), rb.get()]
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}

	list, ok := result.(*object.List)
	if !ok || len(list.Elements) != 2 {
		t.Fatalf("expected two awaited results, got %v", result)
	}
	for i, el := range list.Elements {
		if s, ok := el.(*object.String); !ok || s.StringValue() != "once" {
			t.Fatalf("registrar %d resolved to %v, want \"once\"", i, el)
		}
	}

	RuntimeState.RLock()
	runs, _ := RuntimeState.Atomics["raced_runs"]
	RuntimeState.RUnlock()
	if runs.Value() != 1 {
		t.Fatalf("inner handler ran %d times, want 1 — concurrent claims launched duplicates", runs.Value())
	}
}
