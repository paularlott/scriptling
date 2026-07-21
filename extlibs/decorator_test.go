package extlibs

import (
	"testing"

	"github.com/paularlott/scriptling"
)

func TestHTTPVerbDecorators(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	p.SetVar("__name__", "handlers")

	script := `
import scriptling.runtime.http as http

@http.get("/health")
def health_check(request):
    return http.json(200, {"status": "ok"})

@http.post("/api/users")
def create_user(request):
    return http.json(201, {"id": 1})

@http.put("/api/users/1")
def update_user(request):
    return http.json(200, {"updated": True})

@http.delete("/api/users/1")
def delete_user(request):
    return http.json(204, {})
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	tests := []struct {
		key     string
		handler string
	}{
		{"GET /health", "handlers.health_check"},
		{"POST /api/users", "handlers.create_user"},
		{"PUT /api/users/1", "handlers.update_user"},
		{"DELETE /api/users/1", "handlers.delete_user"},
	}

	for _, tc := range tests {
		route, ok := RuntimeState.Routes[tc.key]
		if !ok {
			t.Errorf("route %q not registered", tc.key)
			continue
		}
		if route.Handler != tc.handler {
			t.Errorf("route %q: expected handler %q, got %q", tc.key, tc.handler, route.Handler)
		}
	}
}

func TestHTTPRouteDecorator(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	p.SetVar("__name__", "api")

	script := `
import scriptling.runtime.http as http

@http.route("/multi", methods=["GET", "POST"])
def multi_handler(request):
    return http.json(200, {})
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if route, ok := RuntimeState.Routes["GET /multi"]; !ok || route.Handler != "api.multi_handler" {
		t.Errorf("GET /multi not registered correctly: got %v", route)
	}
	if route, ok := RuntimeState.Routes["POST /multi"]; !ok || route.Handler != "api.multi_handler" {
		t.Errorf("POST /multi not registered correctly: got %v", route)
	}
}

func TestHTTPMiddlewareDecorator(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	p.SetVar("__name__", "auth")

	script := `
import scriptling.runtime.http as http

@http.middleware
def check_auth(request):
    return None
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if RuntimeState.Middleware != "auth.check_auth" {
		t.Errorf("expected middleware 'auth.check_auth', got %q", RuntimeState.Middleware)
	}
}

func TestHTTPNotFoundDecorator(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	p.SetVar("__name__", "errors")

	script := `
import scriptling.runtime.http as http

@http.not_found
def handle_404(request):
    return http.html(404, "<h1>Not Found</h1>")
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if RuntimeState.NotFoundHandler != "errors.handle_404" {
		t.Errorf("expected not_found 'errors.handle_404', got %q", RuntimeState.NotFoundHandler)
	}
}

func TestHTTPWebsocketDecorator(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	p.SetVar("__name__", "ws")

	script := `
import scriptling.runtime.http as http

@http.websocket("/chat")
def chat_handler(client):
    client.send("Welcome!")
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	route, ok := RuntimeState.WebSocketRoutes["/chat"]
	if !ok {
		t.Fatal("websocket route /chat not registered")
	}
	if route.Handler != "ws.chat_handler" {
		t.Errorf("expected handler 'ws.chat_handler', got %q", route.Handler)
	}
}

func TestHTTPDecoratorModuleNameFromFile(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	// __name__ is "__main__" by default; SetSourceFile sets __file__
	p.SetSourceFile("myapp.py")

	script := `
import scriptling.runtime.http as http

@http.get("/health")
def health_check(request):
    return http.json(200, {"status": "ok"})
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	route, ok := RuntimeState.Routes["GET /health"]
	if !ok {
		t.Fatal("route GET /health not registered")
	}
	if route.Handler != "myapp.health_check" {
		t.Errorf("expected handler 'myapp.health_check' (derived from __file__), got %q", route.Handler)
	}
}

func TestHTTPImperativeBackwardCompat(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	script := `
import scriptling.runtime as runtime

runtime.http.get("/test", "handler.test")
runtime.http.post("/api", "handler.api")
runtime.http.websocket("/ws", "handler.ws")
runtime.http.middleware("auth.check")
runtime.http.not_found("errors.handle")
runtime.http.route("/multi", "handler.multi", methods=["GET", "POST"])
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	checks := map[string]string{
		"GET /test":          "handler.test",
		"POST /api":          "handler.api",
		"GET /multi":         "handler.multi",
		"POST /multi":        "handler.multi",
	}
	for key, want := range checks {
		if route, ok := RuntimeState.Routes[key]; !ok || route.Handler != want {
			t.Errorf("route %q: expected %q", key, want)
		}
	}
	if RuntimeState.Middleware != "auth.check" {
		t.Errorf("middleware: expected %q, got %q", "auth.check", RuntimeState.Middleware)
	}
	if RuntimeState.NotFoundHandler != "errors.handle" {
		t.Errorf("not_found: expected %q, got %q", "errors.handle", RuntimeState.NotFoundHandler)
	}
	if route, ok := RuntimeState.WebSocketRoutes["/ws"]; !ok || route.Handler != "handler.ws" {
		t.Error("websocket route not registered correctly")
	}
}

func TestJSONRPCMethodDecorator(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	p.SetVar("__name__", "rpc")

	script := `
import scriptling.runtime.jsonrpc as jsonrpc

@jsonrpc.method("echo")
def echo(params):
    return params

@jsonrpc.method("add")
def add(params):
    return params["a"] + params["b"]
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if handler, ok := RuntimeState.JSONRPCMethods["echo"]; !ok || handler != "rpc.echo" {
		t.Errorf("method 'echo': expected 'rpc.echo', got %q", handler)
	}
	if handler, ok := RuntimeState.JSONRPCMethods["add"]; !ok || handler != "rpc.add" {
		t.Errorf("method 'add': expected 'rpc.add', got %q", handler)
	}
}

func TestJSONRPCNotificationDecorator(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	p.SetVar("__name__", "rpc")

	script := `
import scriptling.runtime.jsonrpc as jsonrpc

@jsonrpc.notification("updated")
def on_updated(params):
    pass
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if handler, ok := RuntimeState.JSONRPCNotifications["updated"]; !ok || handler != "rpc.on_updated" {
		t.Errorf("notification 'updated': expected 'rpc.on_updated', got %q", handler)
	}
}

func TestJSONRPCImperativeBackwardCompat(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)

	script := `
import scriptling.runtime as runtime

runtime.jsonrpc.method("echo", "handlers.echo")
runtime.jsonrpc.notification("updated", "handlers.on_updated")
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if handler, ok := RuntimeState.JSONRPCMethods["echo"]; !ok || handler != "handlers.echo" {
		t.Errorf("method 'echo': expected 'handlers.echo', got %q", handler)
	}
	if handler, ok := RuntimeState.JSONRPCNotifications["updated"]; !ok || handler != "handlers.on_updated" {
		t.Errorf("notification 'updated': expected 'handlers.on_updated', got %q", handler)
	}
}
