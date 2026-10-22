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

// staticFixture lays out a served directory inside an allowed area, plus a
// file outside it that a symlink in the served directory points to.
func staticFixture(t *testing.T) (allowed, served string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	allowed = filepath.Join(base, "allowed")
	served = filepath.Join(allowed, "public")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{filepath.Join(served, "sub"), filepath.Join(served, "withindex"), filepath.Join(served, ".git"), outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, c string) {
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(served, "hello.txt"), "hello")
	write(filepath.Join(served, "sub", "nested.txt"), "nested")
	write(filepath.Join(served, "withindex", "index.html"), "index page")
	write(filepath.Join(served, ".env"), "SECRET=1")
	write(filepath.Join(served, ".git", "config"), "gitconfig")
	write(filepath.Join(outside, "secret.txt"), "OUTSIDE-SECRET")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(served, "leak.txt")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(served, "leakdir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(served, "hello.txt"), filepath.Join(served, "alias.txt")); err != nil {
		t.Fatal(err)
	}
	return allowed, served
}

func get(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	body, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, string(body)
}

// Under --allowed-paths the static route serves only files that resolve
// inside the allowed paths, and hides dotfiles and listings (vuln-0016).
func TestStaticRouteRestricted(t *testing.T) {
	allowed, served := staticFixture(t)
	h := staticRoute{dir: served, allowed: []string{allowed}}.handler()

	for path, want := range map[string]string{
		"/hello.txt":      "hello",
		"/sub/nested.txt": "nested",
		"/alias.txt":      "hello", // a link that stays inside is fine
		"/withindex/":     "index page",
	} {
		code, body := get(t, h, path)
		if code != http.StatusOK || body != want {
			t.Errorf("GET %s = %d %q, want 200 %q", path, code, body, want)
		}
	}
	for _, path := range []string{
		"/leak.txt",           // symlink out of the allowed paths
		"/leakdir/secret.txt", // through a directory symlink out
		"/.env",               // dotfile
		"/.git/config",        // inside a dot directory
		"/sub/",               // directory without index: no listing
		"/",                   // root listing
	} {
		code, body := get(t, h, path)
		if code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, code)
		}
		if strings.Contains(body, "OUTSIDE-SECRET") || strings.Contains(body, "SECRET=1") || strings.Contains(body, "gitconfig") {
			t.Errorf("GET %s leaked content: %q", path, body)
		}
	}
}

// With deny-all allowed paths nothing is served, even inside the directory.
func TestStaticRouteDenyAll(t *testing.T) {
	_, served := staticFixture(t)
	h := staticRoute{dir: served, allowed: []string{}}.handler()
	if code, _ := get(t, h, "/hello.txt"); code != http.StatusNotFound {
		t.Errorf("deny-all served a file: %d", code)
	}
}

// Without --allowed-paths behaviour is unchanged: a plain file server, which
// serves dotfiles, follows symlinks and lists directories.
func TestStaticRouteUnrestrictedUnchanged(t *testing.T) {
	_, served := staticFixture(t)
	h := staticRoute{dir: served}.handler()
	for path, want := range map[string]string{
		"/hello.txt": "hello",
		"/leak.txt":  "OUTSIDE-SECRET",
		"/.env":      "SECRET=1",
	} {
		if code, body := get(t, h, path); code != http.StatusOK || body != want {
			t.Errorf("GET %s = %d %q, want 200 %q", path, code, body, want)
		}
	}
	if code, body := get(t, h, "/sub/"); code != http.StatusOK || !strings.Contains(body, "nested.txt") {
		t.Errorf("unrestricted directory listing changed: %d %q", code, body)
	}
}
