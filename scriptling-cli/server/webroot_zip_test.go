package server

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

type zipMember struct{ name, body string }

// buildZip writes members in order (so duplicates are kept) and returns the
// archive bytes.
func buildZip(t *testing.T, members []zipMember) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, m := range members {
		w, err := zw.Create(m.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(m.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipServer(t *testing.T, members []zipMember) (*Server, []string) {
	t.Helper()
	data := buildZip(t, members)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	index, skipped := newZipWebRoot(zr)
	return &Server{webRootZipFS: index}, skipped
}

// serveRaw sends rawPath to h unchanged, as a decoded request path can reach
// the handler (e.g. "/..%2fx" arrives as "/../x").
func serveRaw(h http.HandlerFunc, rawPath string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.URL.Path = rawPath
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

var hostileZipMembers = []zipMember{
	{"/etc/passwd", "ABS-NOT-REAL"},
	{"../outside/creds.txt", "TRAVERSAL-ENTRY"},
	{"a/../../b.txt", "DOTDOT-MID"},
	{"./dot.txt", "DOT-ENTRY"},
	{"weird//double.txt", "DOUBLE-SLASH"},
}

// A zip web root served archive members whose names are not valid relative
// paths, which the directory and bundle web roots refuse (CWE-22,
// vuln-0019). Those members are now never indexed, so no request reaches
// them, and they are reported when the archive is opened.
func TestZipWebRootRefusesInvalidMemberNames(t *testing.T) {
	members := append([]zipMember{{"index.html", "IDX"}}, hostileZipMembers...)
	s, skipped := zipServer(t, members)

	for _, raw := range []string{
		"//etc/passwd",
		"/etc/passwd",
		"/../outside/creds.txt",
		"/outside/creds.txt",
		"/a/../../b.txt",
		"/../b.txt",
		"/./dot.txt",
		"/dot.txt",
		"/weird//double.txt",
		"/weird/double.txt",
	} {
		rec := serveRaw(s.serveFromZip, raw)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %q = %d %q, want 404", raw, rec.Code, rec.Body.String())
		}
	}

	var want []string
	for _, m := range hostileZipMembers {
		want = append(want, m.name)
	}
	sort.Strings(skipped)
	sort.Strings(want)
	if !reflect.DeepEqual(skipped, want) {
		t.Errorf("skipped = %q, want %q", skipped, want)
	}
}

// Ordinary archives serve as before: the root index, assets with their
// content type, nested files, a directory's index.html with or without the
// trailing slash, and the first of two members with the same name.
func TestZipWebRootServesValidMembers(t *testing.T) {
	s, skipped := zipServer(t, []zipMember{
		{"index.html", "<h1>zip</h1>"},
		{"app.js", "console.log(1)"},
		{"sub/", ""}, // explicit directory entry
		{"sub/page.html", "<p>sub</p>"},
		{"docs/index.html", "DOCS"},
		{"dup.txt", "FIRST"},
		{"dup.txt", "SECOND"},
	})
	if len(skipped) != 0 {
		t.Fatalf("valid members skipped: %q", skipped)
	}
	for path, want := range map[string]struct{ body, ctype string }{
		"/":              {"<h1>zip</h1>", "text/html; charset=utf-8"},
		"/index.html":    {"<h1>zip</h1>", "text/html; charset=utf-8"},
		"/app.js":        {"console.log(1)", "text/javascript; charset=utf-8"},
		"/sub/page.html": {"<p>sub</p>", "text/html; charset=utf-8"},
		"/docs/":         {"DOCS", ""},
		"/docs":          {"DOCS", ""},
		"/dup.txt":       {"FIRST", ""},
	} {
		rec := serveRaw(s.serveFromZip, path)
		if rec.Code != http.StatusOK || rec.Body.String() != want.body {
			t.Errorf("GET %s = %d %q, want 200 %q", path, rec.Code, rec.Body.String(), want.body)
		}
		if want.ctype != "" && rec.Header().Get("Content-Type") != want.ctype {
			t.Errorf("GET %s Content-Type = %q, want %q", path, rec.Header().Get("Content-Type"), want.ctype)
		}
	}
	for _, path := range []string{"/sub", "/sub/", "/missing.txt", "/sub/missing.html"} {
		if rec := serveRaw(s.serveFromZip, path); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
	}
}

// The three web roots share one candidate rule, so the same content answers
// the same requests the same way whether it is a directory, a bundle or a zip.
func TestWebRootsAgree(t *testing.T) {
	files := map[string]string{
		"index.html":      "ROOT",
		"app.css":         "body{}",
		"sub/page.html":   "PAGE",
		"docs/index.html": "DOCS",
		"big.bin":         strings.Repeat("0123456789abcdef", (webRootBufferedMax*3)/16), // streamed
		"edge.bin":        strings.Repeat("e", webRootBufferedMax),                       // largest buffered
		"empty.txt":       "",
	}

	dir := t.TempDir()
	mapFS := fstest.MapFS{}
	var members []zipMember
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		mapFS[name] = &fstest.MapFile{Data: []byte(body)}
		members = append(members, zipMember{name, body})
	}
	zs, _ := zipServer(t, members)
	roots := map[string]http.HandlerFunc{
		"dir":    (&Server{config: ServerConfig{WebRoot: dir}}).serveFromDir,
		"bundle": (&Server{webRootFS: mapFS}).serveFromBundle,
		"zip":    zs.serveFromZip,
	}

	for _, raw := range []string{
		"/", "/index.html", "/app.css", "/sub/page.html", "/docs", "/docs/",
		"/sub", "/missing", "//index.html", "/../index.html", "/sub/../../index.html",
		"/./app.css", "/sub/./page.html", "/big.bin", "/edge.bin", "/empty.txt",
	} {
		type answer struct {
			code        int
			body, ctype string
		}
		var first answer
		for i, name := range []string{"dir", "bundle", "zip"} {
			rec := serveRaw(roots[name], raw)
			got := answer{rec.Code, rec.Body.String(), rec.Header().Get("Content-Type")}
			if want, ok := files[strings.TrimPrefix(path.Clean(raw), "/")]; ok && !strings.Contains(raw, "..") && !strings.HasPrefix(raw, "//") && got.body != want {
				t.Errorf("GET %q from %s: body of %d bytes, want %d", raw, name, len(got.body), len(want))
			}
			if i == 0 {
				first = got
				continue
			}
			if got != first {
				t.Errorf("GET %q: %s answered %+v, dir answered %+v", raw, name, got, first)
			}
		}
	}
}

// NewServer indexes a --web-root zip when it opens it, and the fallback
// route serves from that index.
func TestNewServerZipWebRoot(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "site.zip")
	members := append([]zipMember{{"index.html", "FROM-ZIP"}}, hostileZipMembers...)
	if err := os.WriteFile(zipPath, buildZip(t, members), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewServer(ServerConfig{ScriptFile: writeSetup(t, "x = 1\n"), WebRoot: zipPath})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer s.webRootZip.Close()

	if rec := serveRaw(s.handleFallback, "/"); rec.Code != http.StatusOK || rec.Body.String() != "FROM-ZIP" {
		t.Fatalf("GET / = %d %q", rec.Code, rec.Body.String())
	}
	for _, raw := range []string{"//etc/passwd", "/../outside/creds.txt"} {
		if rec := serveRaw(s.handleFallback, raw); rec.Code != http.StatusNotFound {
			t.Errorf("GET %q = %d %q, want 404", raw, rec.Code, rec.Body.String())
		}
	}
}
