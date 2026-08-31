package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/paularlott/scriptling/object"
)

// These tests document the batch-load resource contract: LoadPlugins starts
// every spec concurrently (via startBatch) before it inspects results, so if
// registration stops early — on a failed start or a duplicate name — any spec
// that started successfully but was never reached must still be closed, not
// leaked as an orphan subprocess.
//
// The helper below re-execs the test binary as a plugin that writes its PID to
// the file named in SCRIPTLING_LEAK_PLUGIN_PIDFILE at startup, so a test can
// check afterwards whether that process is still alive.

func runLeakPluginTestHelper() {
	name := os.Getenv("SCRIPTLING_LEAK_PLUGIN_NAME")
	if name == "" {
		name = "leak"
	}
	if pidFile := os.Getenv("SCRIPTLING_LEAK_PLUGIN_PIDFILE"); pidFile != "" {
		_ = os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	}
	server := NewServer(name, "1.0.0", "leak test helper")
	server.RegisterLibrary(object.NewLibrary(name, map[string]*object.Builtin{
		"ping": {Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return object.NewString(name)
		}},
	}, nil, "leak test helper"))
	if err := server.Run(); err != nil {
		panic(err)
	}
}

func writeLeakPluginHelper(t *testing.T, path, library, pidFile string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("leak helper uses a POSIX shell script")
	}
	script := "#!/bin/sh\n" +
		"SCRIPTLING_LEAK_PLUGIN_HELPER=1 " +
		"SCRIPTLING_LEAK_PLUGIN_NAME=" + library + " " +
		"SCRIPTLING_LEAK_PLUGIN_PIDFILE=" + pidFile + " " +
		"exec \"" + exe + "\" -test.run=TestLoadPluginsClosesStartedClientsOnMidBatchFailure --\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

func pidAlive(pid int) bool {
	// On unix, signal 0 probes existence without delivering a signal.
	return syscall.Kill(pid, 0) == nil
}

func waitPidGone(pid int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return !pidAlive(pid)
}

// TestLoadPluginsClosesStartedClientsOnMidBatchFailure proves that when a spec
// in the middle of a batch fails to start, LoadPlugins does not orphan the
// later specs that startBatch already started successfully.
//
// Batch layout: [good-early, MISSING, good-late]. startBatch starts good-early
// and good-late (MISSING fails to exec). LoadPlugins walks results in order,
// hits MISSING, and returns. good-late's process was started and handshaked but
// never registered — it must be closed, so its PID must be gone after the call.
func TestLoadPluginsClosesStartedClientsOnMidBatchFailure(t *testing.T) {
	if os.Getenv("SCRIPTLING_LEAK_PLUGIN_HELPER") == "1" {
		runLeakPluginTestHelper()
		return
	}

	dir := t.TempDir()
	latePid := filepath.Join(dir, "late.pid")

	earlyPath := filepath.Join(dir, "a-early")
	writeLeakPluginHelper(t, earlyPath, "early", filepath.Join(dir, "early.pid"))
	latePath := filepath.Join(dir, "c-late")
	writeLeakPluginHelper(t, latePath, "late", latePid)

	m := NewManager(nil)
	defer m.Close()

	err := m.LoadPlugins(context.Background(), []PluginSpec{
		{Path: earlyPath},
		{Path: filepath.Join(dir, "b-missing")}, // fails to start
		{Path: latePath},                        // started by startBatch, never registered
	})
	if err == nil || !strings.Contains(err.Error(), "failed to load") {
		t.Fatalf("expected a failed-to-load error, got: %v", err)
	}

	// The late plugin started (it wrote its pid) but was never registered.
	data, readErr := os.ReadFile(latePid)
	if readErr != nil {
		// It never started at all — nothing to leak; the test's premise
		// (startBatch starts every spec) would be violated, so surface it.
		t.Fatalf("late plugin did not start (no pidfile): %v", readErr)
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if convErr != nil {
		t.Fatalf("bad pidfile contents %q: %v", data, convErr)
	}

	if !waitPidGone(pid, 5*time.Second) {
		_ = syscall.Kill(pid, syscall.SIGKILL) // don't leave a real orphan behind
		t.Fatalf("late plugin process %d is still alive after LoadPlugins returned: "+
			"startBatch started it but the mid-batch failure path leaked it instead of closing it", pid)
	}
}

// TestLoadPluginsClosesStartedClientsOnDuplicateName is the duplicate-name
// analogue: [dup-a, dup-b(same name), good-late]. registerLoaded rejects dup-b
// as "already in use" and LoadPlugins returns, so good-late — started but never
// reached — must be closed rather than orphaned.
func TestLoadPluginsClosesStartedClientsOnDuplicateName(t *testing.T) {
	if os.Getenv("SCRIPTLING_LEAK_PLUGIN_HELPER") == "1" {
		runLeakPluginTestHelper()
		return
	}

	dir := t.TempDir()
	latePid := filepath.Join(dir, "late.pid")

	dupA := filepath.Join(dir, "a-dup")
	writeLeakPluginHelper(t, dupA, "twin", filepath.Join(dir, "a.pid"))
	dupB := filepath.Join(dir, "b-dup")
	writeLeakPluginHelper(t, dupB, "twin", filepath.Join(dir, "b.pid"))
	latePath := filepath.Join(dir, "c-late")
	writeLeakPluginHelper(t, latePath, "late", latePid)

	m := NewManager(nil)
	defer m.Close()

	err := m.LoadPlugins(context.Background(), []PluginSpec{
		{Path: dupA},
		{Path: dupB}, // duplicate name -> registerLoaded errors
		{Path: latePath},
	})
	if err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("expected an already-in-use error, got: %v", err)
	}

	data, readErr := os.ReadFile(latePid)
	if readErr != nil {
		t.Fatalf("late plugin did not start (no pidfile): %v", readErr)
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if convErr != nil {
		t.Fatalf("bad pidfile contents %q: %v", data, convErr)
	}

	if !waitPidGone(pid, 5*time.Second) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("late plugin process %d is still alive after LoadPlugins returned: "+
			"the duplicate-name path leaked a started-but-unregistered client", pid)
	}
}
