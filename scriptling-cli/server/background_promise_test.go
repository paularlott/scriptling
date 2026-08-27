package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
)

// A background task started from a setup script (the queued/released path) and
// a shared watcher that awaits its promise must both complete: the watcher's
// promise.get() resolves after the task's handler returns. Regression test for
// a hang where the task's goroutine and the watcher's goroutine both vanished
// without the promise ever resolving in server mode.
func TestBackgroundTaskPromiseResolvesInServer(t *testing.T) {
	extlibs.ResetRuntime()

	script := writeSetup(t, `
import time
import logging
import scriptling.runtime as runtime

def loop():
    logger = logging.getLogger("s")
    i = 0
    while i < 3:
        i = i + 1
        logger.error("tick %d" % i)
        time.sleep(0.1)
    done = runtime.sync.Atomic("task_done", 0)
    done.set(1)
    return "done"

pr = runtime.background("s", "loop")

def watcher():
    time.sleep(0.5)
    r = pr.get()
    flag = runtime.sync.Atomic("watcher_done", 0)
    flag.set(1)

runtime.background("w", "watcher", shared=True)
`)

	s, err := NewServer(ServerConfig{ScriptFile: script})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() {
		select {
		case <-s.scriptDone:
		case <-time.After(2 * time.Second):
		}
	}()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		extlibs.RuntimeState.RLock()
		taskDone, taskOK := extlibs.RuntimeState.Atomics["task_done"]
		watcherDone, watcherOK := extlibs.RuntimeState.Atomics["watcher_done"]
		extlibs.RuntimeState.RUnlock()
		if taskOK && taskDone.Value() != 0 && watcherOK && watcherDone.Value() != 0 {
			return // both completed
		}
		time.Sleep(50 * time.Millisecond)
	}

	extlibs.RuntimeState.RLock()
	taskDone, taskOK := extlibs.RuntimeState.Atomics["task_done"]
	watcherDone, watcherOK := extlibs.RuntimeState.Atomics["watcher_done"]
	extlibs.RuntimeState.RUnlock()
	if !taskOK || taskDone.Value() == 0 {
		t.Fatal("task handler never completed")
	}
	if !watcherOK || watcherDone.Value() == 0 {
		t.Fatal("watcher never completed — pr.get() on a queued background task did not resolve")
	}
}

// A setup-script background task referencing a module-level constant must see
// it (regression: NameError killed the task silently on its first sleep), and
// a failing fire-and-forget task must be reported through the task error
// logger instead of dying silently.
func TestBackgroundTaskScalarsAndErrorReporting(t *testing.T) {
	extlibs.ResetRuntime()

	var mu sync.Mutex
	var reported []string
	extlibs.SetTaskErrorLogger(func(name string, err error) {
		mu.Lock()
		defer mu.Unlock()
		reported = append(reported, name+": "+err.Error())
	})
	defer extlibs.SetTaskErrorLogger(nil)

	script := writeSetup(t, `
import time
import logging
import scriptling.runtime as runtime

TICK_SECONDS = 0.1
GREETING = "ready"

def ok_task():
    logger = logging.getLogger("t")
    logger.info(GREETING)
    time.sleep(TICK_SECONDS)
    done = runtime.sync.Atomic("scalar_done", 0)
    done.set(1)
    return "ok"

def bad_task():
    return missing_variable_name

runtime.background("ok", "ok_task")
runtime.background("bad", "bad_task")
`)

	if _, err := NewServer(ServerConfig{ScriptFile: script}); err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		extlibs.RuntimeState.RLock()
		done, ok := extlibs.RuntimeState.Atomics["scalar_done"]
		extlibs.RuntimeState.RUnlock()
		if ok && done.Value() != 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	extlibs.RuntimeState.RLock()
	done, ok := extlibs.RuntimeState.Atomics["scalar_done"]
	extlibs.RuntimeState.RUnlock()
	if !ok || done.Value() == 0 {
		t.Fatal("task referencing module-level constants never completed")
	}

	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(reported)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(reported) == 0 {
		t.Fatal("failing fire-and-forget task was not reported")
	}
	if !strings.Contains(reported[0], "bad") || !strings.Contains(reported[0], "missing_variable_name") {
		t.Fatalf("unexpected report: %v", reported)
	}
}

// TestBackgroundTaskNotRespawnedByRequests pins the motivating case for the
// task-name identity rule: a module whose body registers a background task is
// re-executed every time one of its handlers is imported to serve a request
// (each request gets a fresh interpreter). The task must not be re-launched —
// every request observes the one running task.
func TestBackgroundTaskNotRespawnedByRequests(t *testing.T) {
	extlibs.ResetRuntime()

	dir := t.TempDir()
	handlersPy := `import scriptling.runtime as runtime

def tick():
    runtime.sync.Atomic("respawn_starts", initial=0).add(1)
    runtime.sync.Queue("respawn_gate").get()
    return "ticked"

# The module body runs on every import — per request, in a fresh interpreter.
runtime.background("respawn_ticker", "tick")

def starts(request):
    n = runtime.sync.Atomic("respawn_starts", initial=0).get()
    return runtime.http.json(200, {"starts": n})

runtime.http.get("/starts", "handlers.starts")
`
	if err := os.WriteFile(filepath.Join(dir, "handlers.py"), []byte(handlersPy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte("import handlers\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(ServerConfig{
		ScriptFile: filepath.Join(dir, "setup.py"),
		LibDirs:    []string{dir},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	// Wait for the released task to start exactly once.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		extlibs.RuntimeState.RLock()
		started, ok := extlibs.RuntimeState.Atomics["respawn_starts"]
		extlibs.RuntimeState.RUnlock()
		if ok && started.Value() == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	extlibs.RuntimeState.RLock()
	started, ok := extlibs.RuntimeState.Atomics["respawn_starts"]
	extlibs.RuntimeState.RUnlock()
	if !ok || started.Value() != 1 {
		t.Fatalf("ticker started %d times after release, want 1", started.Value())
	}

	// Each request re-imports the module body; none may launch a second copy.
	getStarts := func() int {
		t.Helper()
		resp, err := http.Get(ts.URL + "/starts")
		if err != nil {
			t.Fatalf("GET /starts: %v", err)
		}
		defer resp.Body.Close()
		var out struct {
			Starts int `json:"starts"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("GET /starts: %v", err)
		}
		return out.Starts
	}
	for i := 0; i < 3; i++ {
		if n := getStarts(); n != 1 {
			t.Fatalf("after request %d the ticker had started %d times, want 1 — a request respawned the task", i+1, n)
		}
	}

	// Let the one task finish so no goroutine stays parked on the gate.
	feeder := scriptling.New()
	extlibs.RegisterRuntimeLibraryAll(feeder, nil)
	if _, err := feeder.Eval(`import scriptling.runtime as runtime
runtime.sync.Queue("respawn_gate").put(1)
`); err != nil {
		t.Fatalf("feed gate: %v", err)
	}
}
