package server

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestBearerTokenGuardsUnguardedRoutesWithMiddleware pins the auth model: a
// configured static token wraps the whole mux even when a script middleware
// is registered. The middleware never runs for /health, static routes, the
// webroot fallback or custom not-found handling, so disabling the token when
// middleware existed left those endpoints unauthenticated.
func TestBearerTokenGuardsUnguardedRoutesWithMiddleware(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte(`
def check(request):
    return None   # let everything through; the middleware is not the subject here
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{
		ScriptFile:  script,
		LibDirs:     []string{libDir},
		BearerToken: "seekrit",
		JSONRPC:     true, // the protocol endpoint must exist for the assertions below to mean anything
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	// /health is exactly the endpoint the middleware never sees: without the
	// token it must refuse, with the token it must pass.
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("get /health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /health without token = %d, want 401 (middleware does not guard it)", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	req.Header.Set("Authorization", "Bearer seekrit")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get /health with token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /health with token = %d, want 200", resp.StatusCode)
	}

	// The protocol endpoint requires the token too, and the middleware still
	// runs behind it (a bad middleware answer is a 500 either way).
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/json-rpc", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post /json-rpc: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST /json-rpc without token = %d, want 401", resp.StatusCode)
	}
}

// TestRequestBodyCap pins the per-request body limit: a body past the cap is
// refused by the outermost middleware no matter which endpoint it targets,
// instead of being buffered whole.
func TestRequestBodyCap(t *testing.T) {
	script := writeSetup(t, `
import scriptling.runtime as runtime
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	s, err := NewServer(ServerConfig{
		ScriptFile:          script,
		MaxRequestBodyBytes: 1024,
		BearerToken:         "seekrit",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/json-rpc", nil)
	req.Header.Set("Authorization", "Bearer seekrit")
	req.Header.Set("Content-Type", "application/json")
	chunk := make([]byte, 4096)
	for i := range chunk {
		chunk[i] = 'a'
	}
	var body []byte
	for len(body) <= 4096 {
		body = append(body, chunk...)
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post oversized body: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode < 400 {
		t.Fatalf("oversized body accepted: status %d", resp.StatusCode)
	}
}

// TestSSEStreamSurvivesWriteTimeout proves the SSE endpoint clears the
// server's write deadline: with a hostile 300ms server-level WriteTimeout,
// a GET /mcp stream must still deliver bytes after the deadline would have
// cut it off.
func TestSSEStreamSurvivesWriteTimeout(t *testing.T) {
	toolsDir := t.TempDir()
	writeFile(t, toolsDir+"/probe.toml", []byte("description = \"Probe\"\n"))
	writeFile(t, toolsDir+"/probe.py", []byte("import scriptling.mcp.tool as tool\ntool.return_string(\"ok\")\n"))

	script := writeSetup(t, `
import scriptling.runtime as runtime
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	s, err := NewServer(ServerConfig{
		ScriptFile:  script,
		MCPToolsDir: toolsDir,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewUnstartedServer(s.buildMux())
	ts.Config.WriteTimeout = 300 * time.Millisecond
	ts.Start()
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/mcp", nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open SSE stream: %v", err)
	}
	defer resp.Body.Close()

	// Outlive the write deadline, then prove the stream is still open: a
	// killed stream reads EOF immediately; a live one stays blocked waiting
	// for the next event.
	time.Sleep(600 * time.Millisecond)
	buf := make([]byte, 512)
	readErr := make(chan error, 1)
	go func() {
		_, err := resp.Body.Read(buf)
		readErr <- err
	}()
	select {
	case err := <-readErr:
		t.Fatalf("SSE stream died inside the server write deadline: %v", err)
	case <-time.After(1500 * time.Millisecond):
		// No EOF within the window: the connection survived the deadline.
	}
}

// TestStartReportsBindFailure pins that a listener or certificate failure
// fails Start itself: it used to return nil and surface the error
// asynchronously from the serving goroutine.
func TestStartReportsBindFailure(t *testing.T) {
	// Occupy a port so the server cannot bind it.
	occupier, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	t.Cleanup(func() { _ = occupier.Close() })
	addr := occupier.Addr().String()

	script := writeSetup(t, `
import scriptling.runtime as runtime
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	s, err := NewServer(ServerConfig{ScriptFile: script, Address: addr})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	if err := s.Start(); err == nil {
		t.Fatal("expected Start to fail on a taken address")
	}
}
