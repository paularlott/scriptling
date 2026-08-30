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
)

// Launch guard: a batch load whose context is already cancelled, or is
// cancelled while starts are in flight, must neither hang nor leak subprocesses.
// startBatch spawns every spec before results are inspected; each startClient
// runs its handshake under the caller's context, so a cancelled context must
// fail the handshake fast and self-close the process it just spawned. This
// verifies the no-leak, no-deadlock contract the parallel loader depends on.
//
// Reuses the leak helper's re-exec machinery via a helper that dispatches to
// this test's own name.

func writeCancelPluginHelper(t *testing.T, path, library, pidFile string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("cancel helper uses a POSIX shell script")
	}
	script := "#!/bin/sh\n" +
		"SCRIPTLING_LEAK_PLUGIN_HELPER=1 " +
		"SCRIPTLING_LEAK_PLUGIN_NAME=" + library + " " +
		"SCRIPTLING_LEAK_PLUGIN_PIDFILE=" + pidFile + " " +
		"exec \"" + exe + "\" -test.run=TestLoadPluginsCancelledContextNoLeak --\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

// TestLoadPluginsCancelledContextNoLeak loads a batch with a pre-cancelled
// context. The call must return promptly with an error and leave no live
// subprocess behind: every helper that managed to write a pidfile must be dead.
func TestLoadPluginsCancelledContextNoLeak(t *testing.T) {
	if os.Getenv("SCRIPTLING_LEAK_PLUGIN_HELPER") == "1" {
		runLeakPluginTestHelper()
		return
	}

	dir := t.TempDir()
	var specs []PluginSpec
	pidFiles := make([]string, 0, 6)
	for i, lib := range []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"} {
		pidFile := filepath.Join(dir, lib+".pid")
		pidFiles = append(pidFiles, pidFile)
		path := filepath.Join(dir, "p"+strconv.Itoa(i)+"-"+lib)
		writeCancelPluginHelper(t, path, lib, pidFile)
		specs = append(specs, PluginSpec{Path: path})
	}

	m := NewManager(nil)
	defer m.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the batch starts

	done := make(chan error, 1)
	go func() { done <- m.LoadPlugins(ctx, specs) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error loading with a cancelled context")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("LoadPlugins hung on a cancelled context (possible deadlock)")
	}

	// Any subprocess that started must have been reaped. A helper may or may
	// not have written its pidfile (it depends whether the process got far
	// enough), but whatever pid we can read must be dead.
	deadline := time.Now().Add(5 * time.Second)
	for _, pf := range pidFiles {
		data, readErr := os.ReadFile(pf)
		if readErr != nil {
			continue // never started far enough to record a pid — nothing to leak
		}
		pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if convErr != nil {
			continue
		}
		alive := true
		for time.Now().Before(deadline) {
			if !pidAlive(pid) {
				alive = false
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if alive {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("subprocess %d (from %s) survived a cancelled-context load: leaked", pid, pf)
		}
	}
}
