package http

// Late-registration warning test for routes, moved here from
// extlibs/runtime/runtime_server_test.go with the library.

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/runtime"
)

// captureStderr runs fn and returns what it wrote to stderr (copied from the
// runtime package's tests). The registration checks write their warnings to
// os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	readDone := make(chan string, 1)
	go func() { b, _ := io.ReadAll(r); readDone <- string(b) }()
	fn()
	w.Close()
	os.Stderr = old
	return <-readDone
}

// Registering an HTTP route after start_server warns on stderr that the route
// will not be served (the mux is already built).
func TestHTTPRouteLateRegistrationWarns(t *testing.T) {
	runtime.ResetRuntime()
	defer runtime.ResetRuntime()

	p := scriptling.New()
	registerAll(p, nil)

	// Simulate the server already having started.
	runtime.RuntimeState.Lock()
	runtime.RuntimeState.ServerStarted = true
	runtime.RuntimeState.Unlock()

	got := captureStderr(t, func() {
		if _, err := p.Eval(`import scriptling.runtime as runtime
runtime.http.get("/late", "h")`); err != nil {
			t.Errorf("Eval: %v", err)
		}
	})
	if !strings.Contains(got, "registered after start_server") || !strings.Contains(got, "/late") {
		t.Fatalf("expected late-registration warning for /late on stderr, got: %q", got)
	}

	// A route registered while NOT started must not warn.
	runtime.RuntimeState.Lock()
	runtime.RuntimeState.ServerStarted = false
	runtime.RuntimeState.Unlock()
	got = captureStderr(t, func() {
		if _, err := p.Eval(`import scriptling.runtime as runtime
runtime.http.get("/early", "h")`); err != nil {
			t.Errorf("Eval: %v", err)
		}
	})
	if strings.Contains(got, "registered after start_server") {
		t.Fatalf("unexpected warning for /early (not started), got: %q", got)
	}
}
