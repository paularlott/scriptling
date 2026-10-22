package fetch

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
)

// fetchJail returns an allowed directory, a directory outside it holding an
// existing file, and a server that counts its requests.
func fetchJail(t *testing.T, body []byte) (allowed, outside, url string, hits *atomic.Int32) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	allowed = filepath.Join(base, "allowed")
	outside = filepath.Join(base, "outside")
	for _, d := range []string{allowed, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "present"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	hits = &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return allowed, outside, srv.URL, hits
}

func jailedFetch(allowed string) *scriptling.Scriptling {
	p := scriptling.New()
	RegisterConfigured(p, fssecurity.Config{AllowedPaths: []string{allowed}}, nil)
	return p
}

// provides used to stat every path with no allowed-paths check, so a script
// could test whether any file on the host exists (CWE-22, strix 4250
// vuln-0004). Every entry is now checked first: a path outside the allowed
// directories raises the same uncatchable PermissionError whether or not it
// exists, and nothing is fetched.
func TestFetchProvidesHonorsAllowedPaths(t *testing.T) {
	allowed, outside, url, hits := fetchJail(t, []byte("data"))
	dest := filepath.Join(allowed, "out.bin")

	var messages []string
	for _, probe := range []string{
		filepath.Join(outside, "present"),
		filepath.Join(outside, "absent"),
		allowed + "/../outside/present",
	} {
		p := jailedFetch(allowed)
		_, err := p.Eval(`import scriptling.provision.fetch as fetch
try:
    fetch.file("` + url + `", "` + dest + `", provides=["` + probe + `"])
except Exception:
    pass`)
		if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
			t.Fatalf("%s: expected an uncatchable access denied error, got %v", probe, err)
		}
		messages = append(messages, strings.Replace(err.Error(), probe, "PROBE", 1))
	}
	// Existing and missing paths are refused identically.
	if messages[0] != messages[1] {
		t.Errorf("existing and missing paths are distinguishable:\n%s\n%s", messages[0], messages[1])
	}
	if hits.Load() != 0 {
		t.Fatalf("fetched %d times for refused provides paths", hits.Load())
	}
}

// Inside the allowed directories provides works as before: all present skips
// the fetch, one missing fetches.
func TestFetchProvidesInsideAllowedPaths(t *testing.T) {
	allowed, _, url, hits := fetchJail(t, []byte("data"))
	dest := filepath.Join(allowed, "out.bin")
	present := filepath.Join(allowed, "present")
	os.WriteFile(present, []byte("x"), 0o644)

	p := jailedFetch(allowed)
	result, err := p.Eval(`import scriptling.provision.fetch as fetch
fetch.file("` + url + `", "` + dest + `", provides=["` + present + `"])["status"]`)
	if err != nil || result.Inspect() != "unchanged" || hits.Load() != 0 {
		t.Fatalf("all present: %v %v (fetches %d)", result, err, hits.Load())
	}
	result, err = p.Eval(`fetch.file("` + url + `", "` + dest + `", provides=["` + present + `", "` + allowed + `/missing"])["status"]`)
	if err != nil || result.Inspect() != "created" || hits.Load() != 1 {
		t.Fatalf("one missing: %v %v (fetches %d)", result, err, hits.Load())
	}
}

// A provides path that cannot be stat'ed (permission denied) does not count as
// present; it used to, because only "not exist" cleared the flag.
func TestFetchProvidesUnreadableIsNotPresent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can stat anything")
	}
	allowed, _, url, hits := fetchJail(t, []byte("data"))
	locked := filepath.Join(allowed, "locked")
	os.MkdirAll(locked, 0o755)
	os.WriteFile(filepath.Join(locked, "f"), []byte("x"), 0o644)
	os.Chmod(locked, 0o000)
	defer os.Chmod(locked, 0o755)

	p := jailedFetch(allowed)
	result, err := p.Eval(`import scriptling.provision.fetch as fetch
fetch.file("` + url + `", "` + allowed + `/out.bin", provides=["` + locked + `/f"])["status"]`)
	if err != nil || result.Inspect() != "created" || hits.Load() != 1 {
		t.Fatalf("unreadable provides path: %v %v (fetches %d)", result, err, hits.Load())
	}
}

// dest is checked before the download, so a refused destination fetches
// nothing; and the refusal cannot be caught.
func TestFetchDestCheckedBeforeDownload(t *testing.T) {
	allowed, outside, url, hits := fetchJail(t, []byte("data"))
	p := jailedFetch(allowed)
	_, err := p.Eval(`import scriptling.provision.fetch as fetch
try:
    fetch.file("` + url + `", "` + outside + `/out.bin")
except Exception:
    pass`)
	if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
		t.Fatalf("expected an uncatchable access denied error, got %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("downloaded %d times for a refused destination", hits.Load())
	}
	if _, statErr := os.Stat(filepath.Join(outside, "out.bin")); statErr == nil {
		t.Fatal("wrote outside the allowed directories")
	}
}

// A zip entry that a symlink inside dest redirects outside the allowed
// directories is refused uncatchably too. (A "../" entry is rejected earlier
// as a malformed archive, with or without allowed paths.)
func TestFetchZipEscapeIsPermissionError(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("evil/pwn.txt")
	w.Write([]byte("pwned"))
	zw.Close()
	allowed, outside, url, _ := fetchJail(t, buf.Bytes())
	extract := filepath.Join(allowed, "extract")
	os.MkdirAll(extract, 0o755)
	if err := os.Symlink(outside, filepath.Join(extract, "evil")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	p := jailedFetch(allowed)
	p.EnableOutputCapture()
	_, err := p.Eval(`import scriptling.provision.fetch as fetch
try:
    fetch.file("` + url + `", "` + extract + `", unpack_zip=True)
except Exception:
    pass
print("continued")`)
	if err == nil || !strings.Contains(err.Error(), "outside allowed directories") || strings.Contains(p.GetOutput(), "continued") {
		t.Fatalf("expected an uncatchable access denied error, got %v (output %q)", err, p.GetOutput())
	}
	if _, statErr := os.Stat(filepath.Join(outside, "pwn.txt")); statErr == nil {
		t.Fatal("zip entry written outside the allowed directories")
	}
}

// Without allowed paths, provides and dest are unrestricted, as before.
func TestFetchProvidesUnrestricted(t *testing.T) {
	_, outside, url, hits := fetchJail(t, []byte("data"))
	p := scriptling.New()
	Register(p)
	result, err := p.Eval(`import scriptling.provision.fetch as fetch
fetch.file("` + url + `", "` + outside + `/out.bin", provides=["` + outside + `/present"])["status"]`)
	if err != nil || result.Inspect() != "unchanged" || hits.Load() != 0 {
		t.Fatalf("unrestricted: %v %v (fetches %d)", result, err, hits.Load())
	}
}
