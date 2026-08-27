package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestHTTPPathParameters proves the full chain for wildcard routes:
// setup.py registers GET /api/users/{id} → buildMux serves it → a request to
// /api/users/42 dispatches to the handler (not a 404) → the captured value is
// exposed via request.path_param() along with the other request accessors.
func TestHTTPPathParameters(t *testing.T) {
	dir := t.TempDir()

	handlersPy := `import scriptling.runtime.http as http

@http.get("/api/users/{id}")
def get_user(request):
    return http.json(200, {
        "user_id": request.path_param("id"),
        "missing": request.path_param("nope", "fallback"),
        "query_page": request.query_param("page", "1"),
        "auth": request.header("X-Api-Key"),
        "remote": request.remote_addr,
    })

@http.get("/api/users/new")
def new_user(request):
    return http.json(200, {"route": "literal"})

@http.get("/files/{path...}")
def get_file(request):
    return http.json(200, {"path": request.path_param("path")})
`
	if err := os.WriteFile(filepath.Join(dir, "handlers.py"), []byte(handlersPy), 0o644); err != nil {
		t.Fatal(err)
	}

	setupPy := `import handlers
`
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte(setupPy), 0o644); err != nil {
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

	getJSON := func(t *testing.T, url string, hdr map[string]string) (int, map[string]any) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var out map[string]any
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatalf("GET %s: invalid JSON body %q: %v", url, body, err)
		}
		return resp.StatusCode, out
	}

	// Wildcard route dispatches and exposes the captured parameter.
	code, body := getJSON(t, ts.URL+"/api/users/42", map[string]string{"X-Api-Key": "sekret"})
	if code != 200 {
		t.Fatalf("GET /api/users/42 status = %d, want 200", code)
	}
	if body["user_id"] != "42" {
		t.Errorf("path_param(id) = %v, want \"42\"", body["user_id"])
	}
	if body["missing"] != "fallback" {
		t.Errorf("path_param(nope, default) = %v, want \"fallback\"", body["missing"])
	}
	if body["auth"] != "sekret" {
		t.Errorf("header(X-Api-Key) = %v, want \"sekret\"", body["auth"])
	}
	if r, _ := body["remote"].(string); r == "" {
		t.Errorf("remote_addr = %v, want non-empty", body["remote"])
	}

	// Query accessor with and without a default.
	_, body = getJSON(t, ts.URL+"/api/users/42?page=3", nil)
	if body["query_page"] != "3" {
		t.Errorf("query_param(page) = %v, want \"3\"", body["query_page"])
	}
	_, body = getJSON(t, ts.URL+"/api/users/42", nil)
	if body["query_page"] != "1" {
		t.Errorf("query_param(page, default) = %v, want \"1\"", body["query_page"])
	}

	// Percent-encoded path parameters arrive unescaped.
	_, body = getJSON(t, ts.URL+"/api/users/john%20doe", nil)
	if body["user_id"] != "john doe" {
		t.Errorf("path_param(id) for escaped segment = %v, want \"john doe\"", body["user_id"])
	}

	// A literal route beats the wildcard for the same shape.
	code, body = getJSON(t, ts.URL+"/api/users/new", nil)
	if code != 200 || body["route"] != "literal" {
		t.Errorf("GET /api/users/new = %d %v, want literal route", code, body)
	}

	// Rest wildcard captures the remaining segments.
	_, body = getJSON(t, ts.URL+"/files/docs/readme.txt", nil)
	if body["path"] != "docs/readme.txt" {
		t.Errorf("path_param(path...) = %v, want \"docs/readme.txt\"", body["path"])
	}

	// A path with no matching route still 404s.
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/nothing/here", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/nothing/here: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("GET /api/nothing/here status = %d, want 404", resp.StatusCode)
	}
}

// TestHTTPPathParametersImperative mirrors the website docs' imperative
// registration form: runtime.http.get("/api/users/{id}", "handlers.get_user").
func TestHTTPPathParametersImperative(t *testing.T) {
	dir := t.TempDir()

	handlersPy := `import scriptling.runtime as runtime

def get_user(request):
    user_id = request.path_param("id")
    return runtime.http.json(200, {"user_id": user_id})
`
	if err := os.WriteFile(filepath.Join(dir, "handlers.py"), []byte(handlersPy), 0o644); err != nil {
		t.Fatal(err)
	}

	setupPy := `import scriptling.runtime as runtime
runtime.http.get("/api/users/{id}", "handlers.get_user")
`
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte(setupPy), 0o644); err != nil {
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

	resp, err := http.Get(ts.URL + "/api/users/7")
	if err != nil {
		t.Fatalf("GET /api/users/7: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /api/users/7 status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("invalid JSON %q: %v", body, err)
	}
	if out["user_id"] != "7" {
		t.Errorf("user_id = %v, want \"7\"", out["user_id"])
	}
}

// TestHTTPPathParametersEdgeCases covers the wildcard-route shapes beyond the
// basic {id}: PATCH verb, multiple wildcards, trailing-slash subtree routes,
// route(methods=[...]) on a wildcard path, middleware seeing path_params, the
// default= keyword-argument form, and %2F staying within one segment.
func TestHTTPPathParametersEdgeCases(t *testing.T) {
	dir := t.TempDir()

	handlersPy := `import scriptling.runtime.http as http

@http.patch("/api/users/{id}")
def patch_user(request):
    return http.json(200, {"patched": request.path_param("id")})

@http.get("/api/orgs/{org}/repos/{repo}")
def repo(request):
    return http.json(200, {"org": request.path_param("org"), "repo": request.path_param("repo")})

@http.route("/api/items/{id}", methods=["PUT", "DELETE"])
def item(request):
    return http.json(200, {"method": request.method, "id": request.path_param("id")})

@http.get("/docs/")
def docs(request):
    return http.json(200, {"file": request.path_param("file", default="index.html")})

@http.middleware
def guard(request):
    if request.path_param("id") == "blocked":
        return http.json(403, {"middleware_saw": request.path_param("id")})
    return None  # continue to the route handler

@http.get("/api/users/{id}/notes")
def notes(request):
    return http.json(200, {"id": request.path_param("id")})
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

	getJSON := func(t *testing.T, method, path string) (int, map[string]any) {
		t.Helper()
		req, err := http.NewRequest(method, ts.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var out map[string]any
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatalf("%s %s: invalid JSON %q: %v", method, path, body, err)
		}
		return resp.StatusCode, out
	}

	// PATCH verb dispatches with the captured parameter.
	code, body := getJSON(t, http.MethodPatch, "/api/users/42")
	if code != 200 || body["patched"] != "42" {
		t.Errorf("PATCH /api/users/42 = %d %v", code, body)
	}

	// Multiple wildcards in one pattern.
	code, body = getJSON(t, http.MethodGet, "/api/orgs/acme/repos/gateway")
	if code != 200 || body["org"] != "acme" || body["repo"] != "gateway" {
		t.Errorf("GET /api/orgs/acme/repos/gateway = %d %v", code, body)
	}

	// route(methods=[...]) on a wildcard path; other methods get the mux's 405.
	code, body = getJSON(t, http.MethodPut, "/api/items/7")
	if code != 200 || body["id"] != "7" {
		t.Errorf("PUT /api/items/7 = %d %v", code, body)
	}
	resp, err := http.Get(ts.URL + "/api/items/7")
	if err != nil {
		t.Fatalf("GET /api/items/7: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/items/7 = %d, want 405 (route registered PUT/DELETE only)", resp.StatusCode)
	}

	// Trailing-slash subtree route: the bare path and subpaths dispatch to
	// the same handler; subtree patterns capture no path parameters, so the
	// kwargs default= form is returned.
	code, body = getJSON(t, http.MethodGet, "/docs/")
	if code != 200 || body["file"] != "index.html" {
		t.Errorf("GET /docs/ = %d %v, want default via kwargs form", code, body)
	}
	code, body = getJSON(t, http.MethodGet, "/docs/deep/page")
	if code != 200 || body["file"] != "index.html" {
		t.Errorf("GET /docs/deep/page = %d %v, want subtree dispatch with default", code, body)
	}

	// Wildcard in the middle of a pattern.
	code, body = getJSON(t, http.MethodGet, "/api/users/42/notes")
	if code != 200 || body["id"] != "42" {
		t.Errorf("GET /api/users/42/notes = %d %v", code, body)
	}

	// Middleware sees the same path_params: it short-circuits when the
	// captured id is "blocked" and lets everything else through.
	code, body = getJSON(t, http.MethodGet, "/api/users/blocked/notes")
	if code != 403 || body["middleware_saw"] != "blocked" {
		t.Errorf("GET /api/users/blocked/notes = %d %v, want middleware 403 with captured id", code, body)
	}

	// %2F stays within one path segment: /api/users/a%2Fb/notes captures
	// id="a/b" rather than splitting into more segments.
	code, body = getJSON(t, http.MethodGet, "/api/users/a%2Fb/notes")
	if code != 200 || body["id"] != "a/b" {
		t.Errorf("GET /api/users/a%%2Fb/notes = %d %v, want id=\"a/b\"", code, body)
	}
}

// HEAD requests are dispatched to GET handlers, matching ServeMux.
func TestHTTPHeadDispatchesToGET(t *testing.T) {
	dir := t.TempDir()

	handlersPy := `import scriptling.runtime.http as http

@http.get("/health")
def health(request):
    return http.json(200, {"status": "ok"})
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

	resp, err := http.Head(ts.URL + "/health")
	if err != nil {
		t.Fatalf("HEAD /health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("HEAD /health status = %d, want 200", resp.StatusCode)
	}
}
