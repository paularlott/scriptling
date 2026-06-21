package fetch

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
)

func TestProvisionFetchRegistration(t *testing.T) {
	p := scriptling.New()
	Register(p)

	if _, err := p.Eval(`import scriptling.provision.fetch as fetch`); err != nil {
		t.Fatalf("Failed to import provision.fetch library: %v", err)
	}
}

func TestFetchFileCreatesAndUnchanged(t *testing.T) {
	response := "hello fetch"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.txt")

	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
first = fetch.file("` + server.URL + `", "` + path + `")
second = fetch.file("` + server.URL + `", "` + path + `")
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fetched file: %v", err)
	}
	if string(content) != "hello fetch" {
		t.Fatalf("unexpected content: %q", string(content))
	}

	firstMap := getResult(t, p, "first")
	if firstMap["status"] != StatusCreated {
		t.Fatalf("first status = %v, want %s", firstMap["status"], StatusCreated)
	}
	if firstMap["bytes"] != int64(len(response)) {
		t.Fatalf("bytes = %v", firstMap["bytes"])
	}
	if firstMap["url"] != server.URL {
		t.Fatalf("url = %v, want %s", firstMap["url"], server.URL)
	}
	if firstMap["path"] != path {
		t.Fatalf("path = %v, want %s", firstMap["path"], path)
	}
	if firstMap["unpacked"] != false {
		t.Fatalf("unpacked = %v, want false", firstMap["unpacked"])
	}
	secondMap := getResult(t, p, "second")
	if secondMap["status"] != StatusUnchanged {
		t.Fatalf("second status = %v, want %s", secondMap["status"], StatusUnchanged)
	}
}

func TestFetchFileUpdates(t *testing.T) {
	body := "first"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	p := scriptling.New()
	Register(p)
	if _, err := p.Eval(`
import scriptling.provision.fetch as fetch
created = fetch.file("` + server.URL + `", "` + path + `")
`); err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}

	body = "second"
	if _, err := p.Eval(`
updated = fetch.file("` + server.URL + `", "` + path + `")
`); err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}

	updated := getResult(t, p, "updated")
	if updated["status"] != StatusUpdated {
		t.Fatalf("status = %v, want %s", updated["status"], StatusUpdated)
	}
	assertFile(t, path, "second")
}

func TestFetchFileAppliesModeAndDirMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("mode\n"))
	}))
	defer server.Close()

	dir := t.TempDir()
	parent := filepath.Join(dir, "nested")
	path := filepath.Join(parent, "file.txt")

	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
result = fetch.file("` + server.URL + `", "` + path + `", mode=0o600, dir_mode=0o700)
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}

	assertPerm(t, path, 0o600)
	assertPerm(t, parent, 0o700)
}

func TestFetchFileHTTPSInsecure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secret-ish"))
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "tls.txt")

	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
caught = False
try:
    fetch.file("` + server.URL + `", "` + path + `")
except Exception as e:
    caught = True
result = fetch.file("` + server.URL + `", "` + path + `", insecure=True)
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	caught, _ := p.GetVar("caught")
	if caught != true {
		t.Fatal("expected HTTPS fetch without insecure=True to fail for test server cert")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fetched file: %v", err)
	}
	if string(content) != "secret-ish" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestFetchUnpackZip(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{
		"app/config.txt": "port=8080\n",
		"README.md":      "hello\n",
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipBytes)
	}))
	defer server.Close()

	dir := t.TempDir()

	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
result = fetch.file("` + server.URL + `", "` + dir + `", unpack_zip=True)
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}

	assertFile(t, filepath.Join(dir, "app", "config.txt"), "port=8080\n")
	assertFile(t, filepath.Join(dir, "README.md"), "hello\n")

	result, _ := p.GetVar("result")
	resultMap := result.(map[string]interface{})
	if resultMap["status"] != StatusCreated {
		t.Fatalf("status = %v, want %s", resultMap["status"], StatusCreated)
	}
	if resultMap["unpacked"] != true {
		t.Fatalf("unpacked = %v, want true", resultMap["unpacked"])
	}
	files := resultMap["files"].([]interface{})
	if len(files) != 2 {
		t.Fatalf("files len = %d, want 2", len(files))
	}
}

func TestFetchUnpackZipMixedStatusUpdatedWins(t *testing.T) {
	firstZip := makeZip(t, map[string]string{
		"existing.txt": "old\n",
	})
	secondZip := makeZip(t, map[string]string{
		"existing.txt": "new\n",
		"new.txt":      "created\n",
	})
	body := firstZip
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer server.Close()

	dir := t.TempDir()
	p := scriptling.New()
	Register(p)
	if _, err := p.Eval(`
import scriptling.provision.fetch as fetch
first = fetch.file("` + server.URL + `", "` + dir + `", unpack_zip=True)
`); err != nil {
		t.Fatalf("first unpack failed: %v", err)
	}
	body = secondZip
	if _, err := p.Eval(`
second = fetch.file("` + server.URL + `", "` + dir + `", unpack_zip=True)
`); err != nil {
		t.Fatalf("second unpack failed: %v", err)
	}

	second := getResult(t, p, "second")
	if second["status"] != StatusUpdated {
		t.Fatalf("status = %v, want %s", second["status"], StatusUpdated)
	}
	assertFile(t, filepath.Join(dir, "existing.txt"), "new\n")
	assertFile(t, filepath.Join(dir, "new.txt"), "created\n")
}

func TestFetchUnpackZipPreservesExecutableBits(t *testing.T) {
	zipBytes := makeZipWithModes(t, map[string]zipEntry{
		"bin/tool": {content: "#!/bin/sh\n", mode: 0o755},
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipBytes)
	}))
	defer server.Close()

	dir := t.TempDir()
	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
result = fetch.file("` + server.URL + `", "` + dir + `", unpack_zip=True)
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "bin", "tool"))
	if err != nil {
		t.Fatalf("stat extracted tool: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("expected executable bits to be preserved, mode=%#o", info.Mode().Perm())
	}
}

func TestFetchUnpackZipAppliesModeAndDirMode(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{
		"app/config.txt": "zip mode\n",
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipBytes)
	}))
	defer server.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "unpack")
	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
result = fetch.file("` + server.URL + `", "` + dest + `", unpack_zip=True, mode=0o600, dir_mode=0o700)
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}

	assertPerm(t, dest, 0o700)
	assertPerm(t, filepath.Join(dest, "app"), 0o700)
	assertPerm(t, filepath.Join(dest, "app", "config.txt"), 0o600)
}

func TestFetchUnpackZipRejectsTraversal(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{
		"../escape.txt": "nope",
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipBytes)
	}))
	defer server.Close()

	dir := t.TempDir()

	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
caught = False
try:
    fetch.file("` + server.URL + `", "` + dir + `", unpack_zip=True)
except Exception as e:
    caught = "escapes destination" in str(e)
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	caught, _ := p.GetVar("caught")
	if caught != true {
		t.Fatal("expected zip traversal error to be caught")
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("escape file should not exist, stat err=%v", err)
	}
}

func TestFetchUnpackZipRejectsAbsolutePath(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{
		"/escape.txt": "nope",
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipBytes)
	}))
	defer server.Close()

	dir := t.TempDir()
	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
caught = False
try:
    fetch.file("` + server.URL + `", "` + dir + `", unpack_zip=True)
except Exception as e:
    caught = "escapes destination" in str(e)
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	caught, _ := p.GetVar("caught")
	if caught != true {
		t.Fatal("expected absolute zip path error to be caught")
	}
}

func TestFetchRejectsUnsupportedScheme(t *testing.T) {
	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
caught = False
try:
    fetch.file("file:///tmp/nope", "/tmp/nope")
except Exception as e:
    caught = "unsupported URL scheme" in str(e)
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	caught, _ := p.GetVar("caught")
	if caught != true {
		t.Fatal("expected unsupported scheme error to be caught")
	}
}

func TestFetchRejectsHTTPErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer server.Close()

	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
caught = False
try:
    fetch.file("` + server.URL + `", "/tmp/nope")
except Exception as e:
    caught = "HTTP 500" in str(e)
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	caught, _ := p.GetVar("caught")
	if caught != true {
		t.Fatal("expected HTTP status error to be caught")
	}
}

func TestFetchRejectsEmptyHost(t *testing.T) {
	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
caught = False
try:
    fetch.file("https:///missing-host", "/tmp/nope")
except Exception as e:
    caught = "URL host is required" in str(e)
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	caught, _ := p.GetVar("caught")
	if caught != true {
		t.Fatal("expected empty host error to be caught")
	}
}

func TestFetchRejectsInvalidArguments(t *testing.T) {
	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
errors = []
for fn in [
    lambda: fetch.file("http://example.test"),
    lambda: fetch.file(1, "/tmp/out"),
    lambda: fetch.file("http://example.test", 2),
    lambda: fetch.file("http://example.test", "/tmp/out", timeout=0),
    lambda: fetch.file("http://example.test", "/tmp/out", max_bytes=-1),
    lambda: fetch.file("http://example.test", "/tmp/out", mode=-1),
    lambda: fetch.file("http://example.test", "/tmp/out", dir_mode=-1),
]:
    try:
        fn()
        errors.append("missing")
    except Exception as e:
        errors.append("ok")
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	errors, _ := p.GetVar("errors")
	for i, value := range errors.([]interface{}) {
		if value != "ok" {
			t.Fatalf("case %d = %v, want ok", i, value)
		}
	}
}

func TestFetchExpandsHomePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("home\n"))
	}))
	defer server.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)

	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
result = fetch.file("` + server.URL + `", "~/fetched.txt")
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	assertFile(t, filepath.Join(home, "fetched.txt"), "home\n")
	result := getResult(t, p, "result")
	if result["path"] != filepath.Join(home, "fetched.txt") {
		t.Fatalf("path = %v, want expanded home path", result["path"])
	}
}

func TestFetchMaxBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("too large"))
	}))
	defer server.Close()

	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
caught = False
try:
    fetch.file("` + server.URL + `", "/tmp/nope", max_bytes=3)
except Exception as e:
    caught = "max_bytes" in str(e)
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	caught, _ := p.GetVar("caught")
	if caught != true {
		t.Fatal("expected max_bytes error to be caught")
	}
}

func TestFetchURLWithCustomDefaultTransportDoesNotPanic(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Body:          io.NopCloser(strings.NewReader("custom")),
			ContentLength: int64(len("custom")),
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})
	defer func() { http.DefaultTransport = oldTransport }()

	data, err := fetchURL(context.Background(), "http://example.test/file", false, time.Second, 0)
	if err != nil {
		t.Fatalf("fetchURL failed: %v", err)
	}
	if string(data) != "custom" {
		t.Fatalf("data = %q, want custom", string(data))
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

type zipEntry struct {
	content string
	mode    os.FileMode
}

func makeZipWithModes(t *testing.T, files map[string]zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, entry := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(entry.content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func getResult(t *testing.T, p *scriptling.Scriptling, name string) map[string]interface{} {
	t.Helper()
	result, objErr := p.GetVar(name)
	if objErr != nil {
		t.Fatalf("get %s: %s", name, objErr.Inspect())
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("%s = %T, want map[string]interface{}", name, result)
	}
	return resultMap
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.TrimPrefix(string(raw), "\ufeff") != want {
		t.Fatalf("%s = %q, want %q", path, string(raw), want)
	}
}

func assertPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %#o, want %#o", path, got, want)
	}
}


func TestFetchProvidesSkipWhenAllExist(t *testing.T) {
	tmp := t.TempDir()
	file1 := filepath.Join(tmp, "file1.txt")
	file2 := filepath.Join(tmp, "file2.txt")
	if err := os.WriteFile(file1, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
result = fetch.file(
    "https://example.com/test.zip",
    "` + tmp + `/dest",
    unpack_zip=True,
    provides=["` + file1 + `", "` + file2 + `"],
)
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	// Verify no dest directory was created (fetch was skipped)
	destDir := filepath.Join(tmp, "dest")
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		t.Fatal("expected dest directory to not exist when provides are all present")
	}
}

func TestFetchProvidesSkipsWhenOneMissing(t *testing.T) {
	tmp := t.TempDir()
	file1 := filepath.Join(tmp, "file1.txt")
	if err := os.WriteFile(file1, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(tmp, "missing.txt")

	// Create a valid zip archive
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("test.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("test content"))
	zw.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer server.Close()

	p := scriptling.New()
	Register(p)
	_, err = p.Eval(`
import scriptling.provision.fetch as fetch
result = fetch.file(
    "` + server.URL + `",
    "` + tmp + `/dest",
    unpack_zip=True,
    provides=["` + file1 + `", "` + missing + `"],
)
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	// Verify dest directory was created (fetch proceeded because provides had missing file)
	destDir := filepath.Join(tmp, "dest")
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		t.Fatal("expected dest directory to exist when provides has missing files")
	}
}

func TestFetchProvidesEmptyList(t *testing.T) {
	tmp := t.TempDir()
	response := "hello provides"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	}))
	defer server.Close()

	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.provision.fetch as fetch
result = fetch.file(
    "` + server.URL + `",
    "` + tmp + `/file.txt",
    provides=[],
)
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	path := filepath.Join(tmp, "file.txt")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fetched file: %v", err)
	}
	if string(content) != "hello provides" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}
