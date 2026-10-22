package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// registerControl registers scriptling.plugin on a fresh interpreter through
// RegisterLibrariesConfigured and returns it.
func registerControl(t *testing.T, manager *Manager, pol *Policy, allowProcesses bool) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	RegisterLibrariesConfigured(p, manager, pol, allowProcesses)
	return p
}

// A script that starts an executable plugin runs a subprocess; where the host
// disables subprocesses, plugin.load() of an executable must be refused before
// the process starts (CWE-863, vuln-0015), whatever the filesystem/network
// policy says.
func TestPluginLoadExecRefusedWhenProcessesDisallowed(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	launcher := filepath.Join(t.TempDir(), "launch.sh")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, pol := range []*Policy{nil, {}, {AllowedPaths: []string{filepath.Dir(launcher)}}} {
		p := registerControl(t, NewManager(nil), pol, false)
		_, err := p.Eval(`import scriptling.plugin as P
P.load("s", "` + launcher + `", scriptling=False)`)
		if err == nil || !strings.Contains(err.Error(), "not permitted") {
			t.Fatalf("policy=%+v: expected a transport refusal, got: %v", pol, err)
		}
		if _, statErr := os.Stat(marker); statErr == nil {
			t.Fatalf("policy=%+v: the launcher ran despite processes being disabled", pol)
		}
		// An absolute system shell is refused too (by transport, or by the
		// exec-path jail when one is set — either way it cannot run).
		_, shErr := p.Eval(`import scriptling.plugin as P
P.load("s", "/bin/sh", scriptling=False, args=["-c", "true"])`)
		if shErr == nil {
			t.Fatalf("policy=%+v: /bin/sh was not refused", pol)
		}
	}
}

// Even inside the allowed paths, an executable plugin is refused when
// processes are disabled: staging a launcher in the jail must not be a way
// back in (the report's combined bypass).
func TestPluginLoadExecRefusedInsideJail(t *testing.T) {
	jail := t.TempDir()
	marker := filepath.Join(jail, "ran")
	launcher := filepath.Join(jail, "launch.sh")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := registerControl(t, NewManager(nil), &Policy{AllowedPaths: []string{jail}}, false)
	if _, err := p.Eval(`import scriptling.plugin as P
P.load("s", "` + launcher + `", scriptling=False)`); err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("a launcher staged inside the jail should still be refused, got: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("the staged launcher ran")
	}
}

// HTTP(S) plugins are not subprocesses, so they are still loadable when
// processes are disabled: a real HTTP plugin server loads and its function
// runs.
func TestPluginLoadHTTPAllowedWhenProcessesDisallowed(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	srv := httptestServer(t, NewServer("httpplug", "1.0.0", "http plugin").RegisterFunc("echo", echo))
	waitHTTPReady(t, srv.URL, 5*time.Second)

	p := registerControl(t, NewManager(nil), nil, false)
	// The load must not be refused by the transport restriction: an HTTP
	// plugin is not a subprocess. It handshakes and registers.
	result, err := p.Eval(`import scriptling.plugin as P
P.load("h", "` + srv.URL + `", scriptling=False)
"plugin.h" in [m["name"] for m in P.list()]`)
	if err != nil {
		t.Fatalf("HTTP plugin load with processes disabled was refused: %v", err)
	}
	if result.Inspect() != "True" {
		t.Fatalf("HTTP plugin did not register: %s", result.Inspect())
	}
}

// With processes allowed (the default and RegisterLibraries), an executable
// plugin loads and runs as before.
func TestPluginLoadExecAllowedWhenProcessesAllowed(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	launcher := filepath.Join(t.TempDir(), "launch.sh")
	// The launcher is not an MCP/plugin server, so the handshake fails after
	// it runs; that is enough to prove the process was started.
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\ntouch "+marker+"\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, allow := range []func(*scriptling.Scriptling){
		func(p *scriptling.Scriptling) { RegisterLibraries(p, NewManager(nil)) },
		func(p *scriptling.Scriptling) { RegisterLibrariesConfigured(p, NewManager(nil), nil, true) },
	} {
		p := scriptling.New()
		allow(p)
		_, _ = p.Eval(`import scriptling.plugin as P
try:
    P.load("s", "` + launcher + `", scriptling=False)
except Exception:
    pass`)
		// The subprocess touches the marker as it starts; give it a moment.
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if _, statErr := os.Stat(marker); statErr == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if _, statErr := os.Stat(marker); statErr != nil {
			t.Fatal("with processes allowed the launcher should have started")
		}
		os.Remove(marker)
	}
}

// Plugins the operator preloaded on the manager (as --plugin / --plugin-dir
// do) stay visible and callable even when script-driven process execution is
// disabled: the operator is trusted. Only the ability to load new executables
// from a script is removed.
func TestOperatorPreloadedPluginsCallableWhenProcessesDisallowed(t *testing.T) {
	fb := object.NewFunctionBuilder()
	fb.Function(func() string { return "operator" })
	server := NewServer("preloaded", "1.0.0", "operator plugin")
	server.RegisterFunc("who", fb)
	client := policyPipeServer(t, server, nil)
	defer client.Close()

	manager := NewManager(nil)
	manager.mu.Lock()
	manager.clients["plugin.preloaded"] = client
	manager.mu.Unlock()

	// Processes disabled: the control scope is HTTP-only, but the preloaded
	// plugin is on the parent manager and must remain fully usable.
	p := registerControl(t, manager, nil, false)
	result, err := p.Eval(`import scriptling.plugin as P
listed = "plugin.preloaded" in [m["name"] for m in P.list()]
called = P.call_function("plugin.preloaded", "who")
[listed, called]`)
	if err != nil {
		t.Fatalf("operator plugin should be usable with processes disabled: %v", err)
	}
	if got := result.Inspect(); got != "[True, 'operator']" {
		t.Fatalf("operator plugin not usable: got %s", got)
	}
}
