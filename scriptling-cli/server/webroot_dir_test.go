package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestServeFromDir covers the directory WebRoot path: it serves files,
// resolves index.html at the root, serves nested files, and rejects path
// traversal. It mirrors the guarantees already covered for the bundle path.
func TestServeFromDir(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"index.html":    "<h1>dir webroot</h1>",
		"app.js":        "console.log(\"dir\")",
		"sub/page.html": "<p>sub</p>",
		"secret.key":    "topsecret",
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := &Server{config: ServerConfig{WebRoot: dir}}
	ts := httptest.NewServer(http.HandlerFunc(s.serveFromDir))
	defer ts.Close()

	// index.html served at the root.
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "dir webroot") {
		t.Errorf("/ = %d %q", resp.StatusCode, body)
	}

	// A top-level asset is served.
	resp, err = http.Get(ts.URL + "/app.js")
	if err != nil {
		t.Fatalf("GET /app.js: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "dir") {
		t.Errorf("/app.js = %d %q", resp.StatusCode, body)
	}

	// A nested file is served.
	resp, err = http.Get(ts.URL + "/sub/page.html")
	if err != nil {
		t.Fatalf("GET /sub/page.html: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "sub") {
		t.Errorf("/sub/page.html = %d %q", resp.StatusCode, body)
	}

	// Server-side traversal containment: force ".." into the request path
	// (the HTTP client would otherwise collapse it before sending) and
	// confirm serveFromDir never serves content outside the root.
	for _, raw := range []string{
		"/../" + filepath.Base(dir) + "/secret.key",
		"/../../../etc/passwd",
		"/sub/../../secret.key",
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL.Path = raw
		rec := httptest.NewRecorder()
		s.serveFromDir(rec, req)
		if rec.Code == 200 {
			t.Errorf("traversal %q returned 200 (%q)", raw, rec.Body.String())
		}
	}
}
