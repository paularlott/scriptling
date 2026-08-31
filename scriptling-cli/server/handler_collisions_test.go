package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// TestSameFunctionNameAcrossModules: two modules may define the same handler
// function name; refs are fully qualified ("alpha.mod.detail" vs
// "beta.mod.detail") so each route dispatches to its own module's function.
// The decorated handler also takes a named parameter with a default, which
// the request-time call must leave at its default.
func TestSameFunctionNameAcrossModules(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "alpha", "mod.py"), []byte(`import scriptling.runtime.http as http

@http.get("/alpha/{id}")
def detail(request, version="v1"):
    return http.json(200, {"src": "alpha", "version": version})
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "beta", "mod.py"), []byte(`import scriptling.runtime.http as http

@http.get("/beta/{id}")
def detail(request):
    return http.json(200, {"src": "beta"})
`), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := writeSetup(t, "import alpha.mod\nimport beta.mod\n")
	s, err := NewServer(ServerConfig{ScriptFile: setup, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	get := func(path string) (int, string) {
		t.Helper()
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode, string(body)
	}

	code, body := get("/alpha/7")
	if code != 200 || body != `{"src":"alpha","version":"v1"}` {
		t.Errorf("GET /alpha/7 = %d %s, want alpha with default version", code, body)
	}
	code, body = get("/beta/7")
	if code != 200 || body != `{"src":"beta"}` {
		t.Errorf("GET /beta/7 = %d %s, want beta", code, body)
	}
}

// TestDuplicateFunctionNameWithinModule: redefining a function name in one
// module follows Python shadowing — the later definition wins, and both
// routes registered under that name dispatch to it.
func TestDuplicateFunctionNameWithinModule(t *testing.T) {
	dir := t.TempDir()
	mod := `import scriptling.runtime.http as http

@http.get("/first")
def same(request):
    return http.json(200, {"which": "first"})

@http.get("/second")
def same(request):
    return http.json(200, {"which": "second"})
`
	if err := os.WriteFile(filepath.Join(dir, "mod.py"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := writeSetup(t, "import mod\n")
	s, err := NewServer(ServerConfig{ScriptFile: setup, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	for _, path := range []string{"/first", "/second"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || string(body) != `{"which":"second"}` {
			t.Errorf("GET %s = %d %s, want the later definition (second)", path, resp.StatusCode, body)
		}
	}
}

// TestConflictingWildcardPatternsSkipped: two routes whose wildcard patterns
// match the same requests (/items/{name}/detail vs /items/{slug}/detail) are
// a configuration error at startup: which one survived used to depend on map
// iteration order, with the loser silently dropped mid-serve.
func TestConflictingWildcardPatternsSkipped(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "alpha", "mod.py"), []byte(`import scriptling.runtime.http as http

@http.get("/items/{name}/detail")
def detail(request):
    return http.json(200, {"src": "alpha"})

@http.get("/alpha/ok")
def ok(request):
    return http.json(200, {"src": "alpha"})
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "beta", "mod.py"), []byte(`import scriptling.runtime.http as http

@http.get("/items/{slug}/detail")
def detail(request):
    return http.json(200, {"src": "beta"})

@http.get("/beta/ok")
def ok(request):
    return http.json(200, {"src": "beta"})
`), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := writeSetup(t, "import alpha.mod\nimport beta.mod\n")
	_, err := NewServer(ServerConfig{ScriptFile: setup, LibDirs: []string{dir}})
	if err == nil || !strings.Contains(err.Error(), "conflicts with another route") {
		t.Fatalf("expected the conflicting wildcard routes to fail startup, got: %v", err)
	}
}

// TestBareRootAndDollarRouteBothRegistered: "GET /" and "GET /{$}" map to the
// same mux pattern; the duplicate registration must be skipped, not panic.
func TestBareRootAndDollarRouteBothRegistered(t *testing.T) {
	s := &Server{handlers: map[string]string{
		"GET /":    "a.index",
		"GET /{$}": "b.index",
	}}
	mux := s.buildMux() // must not panic

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code == http.StatusNotFound {
		t.Error("GET / must still be served after skipping the duplicate pattern")
	}
}

// TestHandlerFailures: a raising handler returns a JSON 500, and a handler
// reference without a module part returns a plain 500 — neither crashes the
// request path.
func TestHandlerFailures(t *testing.T) {
	dir := t.TempDir()
	mod := `import scriptling.runtime as runtime

def boom(request):
    raise Exception("kaboom")
`
	if err := os.WriteFile(filepath.Join(dir, "mod.py"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := writeSetup(t, `
import scriptling.runtime as runtime
runtime.http.get("/boom", "mod.boom")
runtime.http.get("/bad-ref", "nodot")
`)
	s, err := NewServer(ServerConfig{ScriptFile: setup, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/boom")
	if err != nil {
		t.Fatalf("GET /boom: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 500 || !strings.Contains(string(body), "kaboom") {
		t.Errorf("GET /boom = %d %s, want 500 with the error message", resp.StatusCode, body)
	}

	resp, err = http.Get(ts.URL + "/bad-ref")
	if err != nil {
		t.Fatalf("GET /bad-ref: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Errorf("GET /bad-ref = %d, want 500 for a reference with no module part", resp.StatusCode)
	}
}

// TestLambdaDecoratorRejected: decorating an unnamed function (a lambda)
// returns a script-level error instead of registering a broken route.
func TestLambdaDecoratorRejected(t *testing.T) {
	s := &Server{}
	p := scriptling.New()
	s.setupScriptling(p)

	result, err := p.Eval("import scriptling.runtime.http as http\nhttp.get(\"/x\")(lambda r: 1)")
	if err == nil {
		if result == nil {
			t.Fatal("expected an error result for decorating a lambda")
		}
		if _, isErr := result.(*object.Error); !isErr {
			t.Fatalf("expected error result, got %T (%s)", result, result.Inspect())
		}
	}
}

// TestNestedAndFlatModuleNamespaces: nothing is discarded from a handler
// reference — the full dotted module path is imported. A flat module
// (handlers/instant.py -> "handlers.instant.create") and a nested one
// (handlers/sales/instant.py -> "handlers.sales.instant.create") are distinct
// module names even when every function shares the same name, so all four
// combinations dispatch to their own module.
func TestNestedAndFlatModuleNamespaces(t *testing.T) {
	dir := t.TempDir()
	salesDir := filepath.Join(dir, "handlers", "sales")
	if err := os.MkdirAll(salesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mod := func(route string) string {
		return `import scriptling.runtime.http as http

@http.get("` + route + `")
def create(request):
    return http.json(200, {"route": "` + route + `"})
`
	}
	files := map[string]string{
		filepath.Join(dir, "handlers", "instant.py"): mod("/orders/instant"),
		filepath.Join(dir, "handlers", "credit.py"):  mod("/orders/credit"),
		filepath.Join(salesDir, "instant.py"):        mod("/orders/sales/instant"),
		filepath.Join(salesDir, "credit.py"):         mod("/orders/sales/credit"),
	}
	for path, src := range files {
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	setup := writeSetup(t, `
import handlers.instant
import handlers.credit
import handlers.sales.instant
import handlers.sales.credit
`)
	s, err := NewServer(ServerConfig{ScriptFile: setup, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	wantRefs := map[string]string{
		"/orders/instant":       "handlers.instant.create",
		"/orders/credit":        "handlers.credit.create",
		"/orders/sales/instant": "handlers.sales.instant.create",
		"/orders/sales/credit":  "handlers.sales.credit.create",
	}
	for path, want := range wantRefs {
		if got := s.handlers["GET "+path]; got != want {
			t.Fatalf("route %s ref = %q, want %q", path, got, want)
		}
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	for path := range wantRefs {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("GET %s = %d (%s), want 200", path, resp.StatusCode, body)
		} else if !strings.Contains(string(body), `"route":"`+path+`"`) {
			t.Errorf("GET %s body = %s, want it served by its own module", path, body)
		}
	}
}

// TestStackedRouteDecorators: applying several route decorators to one
// function registers a route per decorator, all dispatching to that function.
func TestStackedRouteDecorators(t *testing.T) {
	dir := t.TempDir()
	mod := `import scriptling.runtime.http as http

@http.get("/multi")
@http.post("/multi")
def multi(request):
    return http.json(200, {"fn": "multi"})
`
	if err := os.WriteFile(filepath.Join(dir, "mod.py"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := writeSetup(t, "import mod\n")
	s, err := NewServer(ServerConfig{ScriptFile: setup, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	for _, key := range []string{"GET /multi", "POST /multi"} {
		if ref := s.handlers[key]; ref != "mod.multi" {
			t.Fatalf("route %s ref = %q, want mod.multi", key, ref)
		}
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		req, err := http.NewRequest(method, ts.URL+"/multi", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s /multi: %v", method, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.Contains(string(body), `"fn":"multi"`) {
			t.Errorf("%s /multi = %d %s, want the stacked handler", method, resp.StatusCode, body)
		}
	}
}

// TestInvalidRoutePatternSkipped: a malformed pattern (unclosed wildcard) is
// rejected by ServeMux; the route is skipped with an error log and the rest
// of the server keeps serving instead of panicking at startup.
func TestInvalidRoutePatternSkipped(t *testing.T) {
	dir := t.TempDir()
	mod := `import scriptling.runtime.http as http

@http.get("/bad/{")
def bad(request):
    return http.json(200, {"fn": "bad"})

@http.get("/good")
def good(request):
    return http.json(200, {"fn": "good"})
`
	if err := os.WriteFile(filepath.Join(dir, "mod.py"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := writeSetup(t, "import mod\n")
	_, err := NewServer(ServerConfig{ScriptFile: setup, LibDirs: []string{dir}})
	if err == nil {
		t.Fatal("expected the malformed route pattern to fail startup")
	}
}

// TestPluginHandlerDottedModule: plugin functions registered from a module in
// a subdirectory dispatch through the same last-dot ref split.
func TestPluginHandlerDottedModule(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, "plug")
	if err := os.Mkdir(plugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugDir, "mod.py"), []byte("def add(a, b):\n    return a + b\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := writeSetup(t, "x = 1\n")
	s, err := NewServer(ServerConfig{ScriptFile: setup, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	result := s.runPluginHandler(context.Background(), "plug.mod.add",
		[]object.Object{object.NewInteger(3), object.NewInteger(4)}, nil)
	if result == nil {
		t.Fatal("runPluginHandler returned nil")
	}
	if e, isErr := result.(*object.Error); isErr {
		t.Fatalf("runPluginHandler error: %s", e.Message)
	}
	sum, convErr := result.AsInt()
	if convErr != nil || sum != 7 {
		t.Errorf("plug.mod.add(3, 4) = %v, want 7", result.Inspect())
	}
}
