package fetch

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello fetch"))
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

	first, _ := p.GetVar("first")
	firstMap := first.(map[string]interface{})
	if firstMap["status"] != StatusCreated {
		t.Fatalf("first status = %v, want %s", firstMap["status"], StatusCreated)
	}
	if firstMap["bytes"] != int64(len("hello fetch")) {
		t.Fatalf("bytes = %v", firstMap["bytes"])
	}
	second, _ := p.GetVar("second")
	secondMap := second.(map[string]interface{})
	if secondMap["status"] != StatusUnchanged {
		t.Fatalf("second status = %v, want %s", secondMap["status"], StatusUnchanged)
	}
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
	files := resultMap["files"].([]interface{})
	if len(files) != 2 {
		t.Fatalf("files len = %d, want 2", len(files))
	}
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
