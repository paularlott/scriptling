package extlibs

import (
	"testing"
	"time"

	"github.com/paularlott/scriptling"
)

// initServerChannels wires RuntimeState into "server mode" for tests. In script
// mode these channels are nil and start_server/server_running are no-ops.
func initServerChannels() {
	RuntimeState.Lock()
	RuntimeState.ServerStartCh = make(chan struct{})
	RuntimeState.ServerRunningCh = make(chan struct{})
	RuntimeState.ServerStarted = false
	RuntimeState.Unlock()
}

// start_server(wait=False) closes the start channel, marks the server started,
// and returns immediately.
func TestStartServerWaitFalse(t *testing.T) {
	ResetRuntime()
	initServerChannels()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	if _, err := p.Eval(`import scriptling.runtime as runtime
runtime.start_server(wait=False)`); err != nil {
		t.Fatalf("start_server: %v", err)
	}

	RuntimeState.RLock()
	started := RuntimeState.ServerStarted
	startCh := RuntimeState.ServerStartCh
	RuntimeState.RUnlock()
	if !started {
		t.Fatal("ServerStarted should be true after start_server")
	}
	select {
	case <-startCh:
	default:
		t.Fatal("ServerStartCh should be closed after start_server")
	}
}

// server_running() reflects the running-channel state: true while open, false
// once the server signals shutdown (closes the channel).
func TestServerRunning(t *testing.T) {
	ResetRuntime()
	initServerChannels()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	out, err := p.Eval(`import scriptling.runtime as runtime
runtime.server_running()`)
	if err != nil {
		t.Fatalf("server_running: %v", err)
	}
	if out.Inspect() != "true" {
		t.Fatalf("server_running() before shutdown = %s, want true", out.Inspect())
	}

	// Signal shutdown.
	RuntimeState.Lock()
	close(RuntimeState.ServerRunningCh)
	RuntimeState.Unlock()

	out, err = p.Eval(`import scriptling.runtime as runtime
runtime.server_running()`)
	if err != nil {
		t.Fatalf("server_running: %v", err)
	}
	if out.Inspect() != "false" {
		t.Fatalf("server_running() after shutdown = %s, want false", out.Inspect())
	}
}

// start_server() with the default wait=True blocks the script on the running
// channel (GIL released via RunBlocking so handlers/threads can fire) and
// returns once shutdown is signaled.
func TestStartServerWaitTrue(t *testing.T) {
	ResetRuntime()
	initServerChannels()
	defer ResetRuntime()

	RuntimeState.RLock()
	startCh := RuntimeState.ServerStartCh
	RuntimeState.RUnlock()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	done := make(chan error, 1)
	go func() {
		_, err := p.Eval(`import scriptling.runtime as runtime
runtime.start_server()`)
		done <- err
	}()

	// Wait for the script to reach start_server (it closes the start channel
	// before blocking on the running channel).
	select {
	case <-startCh:
	case <-time.After(2 * time.Second):
		t.Fatal("start_server never signaled start")
	}
	// It must now be blocking.
	select {
	case <-done:
		t.Fatal("start_server(wait=True) returned before shutdown")
	default:
	}

	// Trigger shutdown → start_server should return promptly.
	RuntimeState.Lock()
	close(RuntimeState.ServerRunningCh)
	RuntimeState.Unlock()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("start_server returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("start_server(wait=True) did not return after shutdown")
	}
}

// Calling start_server twice must not panic from a double close of the start
// channel (guarded by the ServerStarted flag).
func TestStartServerDoubleCallSafe(t *testing.T) {
	ResetRuntime()
	initServerChannels()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	for i := 0; i < 2; i++ {
		if _, err := p.Eval(`import scriptling.runtime as runtime
runtime.start_server(wait=False)`); err != nil {
			t.Fatalf("start_server call %d: %v", i, err)
		}
	}
}

// In script mode (channels nil) start_server is a no-op and server_running()
// returns false — never panics on close(nil).
func TestStartServerScriptModeNoop(t *testing.T) {
	ResetRuntime()
	defer ResetRuntime()
	// Channels left nil (script mode).

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	if _, err := p.Eval(`import scriptling.runtime as runtime
runtime.start_server(wait=False)`); err != nil {
		t.Fatalf("start_server in script mode: %v", err)
	}
	out, err := p.Eval(`import scriptling.runtime as runtime
runtime.server_running()`)
	if err != nil {
		t.Fatalf("server_running in script mode: %v", err)
	}
	if out.Inspect() != "false" {
		t.Fatalf("server_running() in script mode = %s, want false", out.Inspect())
	}
}
