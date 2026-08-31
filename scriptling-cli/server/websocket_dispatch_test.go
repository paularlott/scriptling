package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialWS opens a real WebSocket connection against the test server.
func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial %s: %v (status %d)", url, err, status)
	}
	return conn
}

func wsRead(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("read deadline: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	return string(msg)
}

func wsWrite(t *testing.T, conn *websocket.Conn, msg string) {
	t.Helper()
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("write deadline: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		t.Fatalf("write message: %v", err)
	}
}

// TestWebSocketHandlerFlatModule: a WebSocket route registered imperatively
// from a flat module upgrades, dispatches on a fresh evaluator, and echoes.
func TestWebSocketHandlerFlatModule(t *testing.T) {
	dir := t.TempDir()
	wsPy := `import scriptling.runtime as runtime

def echo(client):
    client.send("welcome")
    msg = client.receive(timeout=5)
    if msg:
        client.send("echo:" + msg)
`
	if err := os.WriteFile(filepath.Join(dir, "wsecho.py"), []byte(wsPy), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := writeSetup(t, `
import scriptling.runtime as runtime
runtime.http.websocket("/ws", "wsecho.echo")
`)
	s, err := NewServer(ServerConfig{ScriptFile: setup, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	conn := dialWS(t, "ws"+strings.TrimPrefix(ts.URL, "http")+"/ws")
	defer conn.Close()

	if got := wsRead(t, conn); got != "welcome" {
		t.Errorf("first message = %q, want welcome", got)
	}
	wsWrite(t, conn, "ping")
	if got := wsRead(t, conn); got != "echo:ping" {
		t.Errorf("echo = %q, want echo:ping", got)
	}
}

// TestWebSocketHandlerSubdirectoryModule: a @http.websocket-decorated handler
// in ws/mod.py registers the dotted ref "ws.mod.echo"; the connection-time
// re-import must import the module "ws.mod", not cut the ref at the first dot.
func TestWebSocketHandlerSubdirectoryModule(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "ws")
	if err := os.Mkdir(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mod := `import scriptling.runtime.http as http

@http.websocket("/wsub")
def echo(client):
    client.send("sub-welcome")
    msg = client.receive(timeout=5)
    if msg:
        client.send("sub-echo:" + msg)
`
	if err := os.WriteFile(filepath.Join(wsDir, "mod.py"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := writeSetup(t, "import ws.mod\n")
	s, err := NewServer(ServerConfig{ScriptFile: setup, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	if ref := s.wsHandlers["/wsub"]; ref != "ws.mod.echo" {
		t.Fatalf("ws route ref = %q, want ws.mod.echo", ref)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	conn := dialWS(t, "ws"+strings.TrimPrefix(ts.URL, "http")+"/wsub")
	defer conn.Close()

	if got := wsRead(t, conn); got != "sub-welcome" {
		t.Errorf("first message = %q, want sub-welcome", got)
	}
	wsWrite(t, conn, "hi")
	if got := wsRead(t, conn); got != "sub-echo:hi" {
		t.Errorf("echo = %q, want sub-echo:hi", got)
	}
}

// TestWebSocketUnhappyPaths: a plain GET (no upgrade headers) on a WebSocket
// path is not dispatched as a route; an upgrade to an unregistered path fails
// to connect; regular HTTP routes on the same server keep working.
func TestWebSocketUnhappyPaths(t *testing.T) {
	dir := t.TempDir()
	wsPy := `import scriptling.runtime as runtime

def echo(client):
    client.send("welcome")
    msg = client.receive(timeout=5)
    if msg:
        client.send("echo:" + msg)

def hello(request):
    return runtime.http.json(200, {"ok": True})
`
	if err := os.WriteFile(filepath.Join(dir, "wsecho.py"), []byte(wsPy), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := writeSetup(t, `
import scriptling.runtime as runtime
runtime.http.websocket("/ws", "wsecho.echo")
runtime.http.get("/hello", "wsecho.hello")
`)
	s, err := NewServer(ServerConfig{ScriptFile: setup, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()
	wsBase := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Plain GET on the ws path: no handler is registered for GET /ws, so the
	// request must not be dispatched as a script route.
	resp, err := http.Get(ts.URL + "/ws")
	if err != nil {
		t.Fatalf("GET /ws: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /ws = %d, want 404 (no HTTP handler on a ws path)", resp.StatusCode)
	}

	// Upgrade to an unregistered path must fail at dial time.
	if conn, _, err := websocket.DefaultDialer.Dial(wsBase+"/nowhere", nil); err == nil {
		conn.Close()
		t.Error("dial /nowhere succeeded, want failure")
	}

	// The regular HTTP route still serves.
	resp, err = http.Get(ts.URL + "/hello")
	if err != nil {
		t.Fatalf("GET /hello: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /hello = %d, want 200 alongside ws routes", resp.StatusCode)
	}

	// And the ws route itself still works after the failures above.
	conn := dialWS(t, wsBase+"/ws")
	defer conn.Close()
	if got := wsRead(t, conn); got != "welcome" {
		t.Errorf("welcome after failures = %q, want welcome", got)
	}
	wsWrite(t, conn, "still-there")
	if got := wsRead(t, conn); got != "echo:still-there" {
		t.Errorf("echo after failures = %q, want echo:still-there", got)
	}
}

// TestWebSocketOriginPolicy pins the default same-origin check and the
// allowlist: a browser upgrade from another origin is refused (cross-site
// WebSocket hijacking), the same origin passes, an explicit allowlist entry
// passes, and non-browser clients (no Origin header) always pass. Each case
// gets its own server: the runtime state behind start_server is
// process-global and does not stack.
func TestWebSocketOriginPolicy(t *testing.T) {
	// policy builds one server with the given origin policy and returns the
	// raw host:port to dial. The route must live in the setup script itself:
	// a separate module is only registered if something imports it.
	policy := func(t *testing.T, origins []string) string {
		t.Helper()
		script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

@http.websocket("/ws")
def echo(ws):
    pass

runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
		s, err := NewServer(ServerConfig{
			ScriptFile:       script,
			WebSocketOrigins: origins,
		})
		if err != nil {
			t.Fatalf("NewServer: %v", err)
		}
		ts := httptest.NewServer(s.buildMux())
		t.Cleanup(ts.Close)
		t.Cleanup(func() { signalShutdown(t, s) })
		return strings.TrimPrefix(ts.URL, "http://")
	}

	dial := func(t *testing.T, host, origin string) bool {
		t.Helper()
		header := http.Header{}
		if origin != "" {
			header.Set("Origin", origin)
		}
		conn, _, err := websocket.DefaultDialer.Dial("ws://"+host+"/ws", header)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}

	t.Run("default allows same-origin and origin-less", func(t *testing.T) {
		host := policy(t, nil)
		if !dial(t, host, "http://"+host) {
			t.Fatal("same-origin upgrade refused under the default policy")
		}
		if !dial(t, host, "") {
			t.Fatal("origin-less (non-browser) upgrade refused")
		}
	})
	t.Run("default refuses cross-origin", func(t *testing.T) {
		host := policy(t, nil)
		if dial(t, host, "http://evil.example") {
			t.Fatal("cross-origin upgrade accepted under the default policy")
		}
	})
	t.Run("allowlist gates exactly its entries", func(t *testing.T) {
		host := policy(t, []string{"https://app.example"})
		if !dial(t, host, "https://app.example") {
			t.Fatal("allowlisted origin refused")
		}
		if dial(t, host, "https://other.example") {
			t.Fatal("non-allowlisted origin accepted")
		}
	})
	t.Run("wildcard opts out", func(t *testing.T) {
		host := policy(t, []string{"*"})
		if !dial(t, host, "http://anything.example") {
			t.Fatal("wildcard policy refused an origin")
		}
	})
}

// TestConflictingRoutesFailStartup pins that two wildcard-equivalent routes
// are a configuration error: which one survived used to depend on map
// iteration order, with the loser silently dropped.
func TestConflictingRoutesFailStartup(t *testing.T) {
	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

@http.get("/items/{id}/detail")
def by_id(request):
    return {"status": 200, "body": "id"}

@http.get("/items/{slug}/detail")
def by_slug(request):
    return {"status": 200, "body": "slug"}

runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	_, err := NewServer(ServerConfig{ScriptFile: script})
	if err == nil {
		t.Fatal("expected conflicting routes to fail startup")
	}
	if !strings.Contains(err.Error(), "conflicts with another route") {
		t.Fatalf("expected a conflict error naming the route, got: %v", err)
	}

	// Distinct routes still start cleanly.
	ok := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

@http.get("/items/{id}")
def by_id(request):
    return {"status": 200, "body": "id"}

@http.get("/items/{id}/detail")
def detail(request):
    return {"status": 200, "body": "detail"}

runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	s, err := NewServer(ServerConfig{ScriptFile: ok})
	if err != nil {
		t.Fatalf("distinct routes must not conflict: %v", err)
	}
	signalShutdown(t, s)
}
