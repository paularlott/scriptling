package pack

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestRegisterSchemeValidation(t *testing.T) {
	opener := func(source string, insecure bool, cacheDir string) (*Bundle, error) {
		return nil, errors.New("not called")
	}
	for _, scheme := range []string{"http", "https", "file", "", "1starts-with-digit", "has/slash", "has space"} {
		if err := RegisterScheme(scheme, opener); err == nil {
			t.Errorf("expected rejection for scheme %q", scheme)
		}
	}
	if err := RegisterScheme("nil-opener-test", nil); err == nil {
		t.Error("expected rejection for nil opener")
	}
	if err := RegisterScheme("dup-scheme-test", opener); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	if err := RegisterScheme("dup-scheme-test", opener); err == nil {
		t.Error("expected duplicate registration to fail")
	}
}

func TestSchemeFor(t *testing.T) {
	if err := RegisterScheme("scheme-for-test", func(source string, insecure bool, cacheDir string) (*Bundle, error) {
		return nil, errors.New("not called")
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cases := []struct {
		source string
		scheme string
		ok     bool
	}{
		{"scheme-for-test://libs", "scheme-for-test", true},
		{"scheme-for-test://", "", false},         // empty rest
		{"scheme-for-test ", "", false},           // no ://
		{"unregistered-scheme://libs", "", false}, // not registered
		{"http://example.com/x.zip", "", false},   // built-in
		{"https://example.com/x.zip", "", false},  // built-in
		{"/local/path/pkg.zip", "", false},        // local path
		{"relative/pkg.zip", "", false},           // local path
		{"1bad://libs", "", false},                // invalid scheme grammar
		{"scheme-for-test://lib s", "", false},    // whitespace in rest
	}
	for _, tc := range cases {
		scheme, ok := SchemeFor(tc.source)
		if scheme != tc.scheme || ok != tc.ok {
			t.Errorf("SchemeFor(%q) = (%q, %v), want (%q, %v)", tc.source, scheme, ok, tc.scheme, tc.ok)
		}
	}
}

func TestFetchBundleRoutesCustomScheme(t *testing.T) {
	mapFS := fstest.MapFS{
		"manifest.toml": &fstest.MapFile{Data: []byte("name = \"schemepkg\"\nversion = \"1.0.0\"\n")},
		"lib/mod.py":    &fstest.MapFile{Data: []byte("def value():\n    return 'from scheme'\n")},
	}
	called := false
	if err := RegisterScheme("fetch-route-test", func(source string, insecure bool, cacheDir string) (*Bundle, error) {
		called = true
		if source != "fetch-route-test://libs" {
			t.Errorf("opener received source %q", source)
		}
		return OpenBundle(mapFS, source)
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	bundle, err := FetchBundle("fetch-route-test://libs", false, t.TempDir())
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}
	if !called {
		t.Fatal("expected the scheme opener to be called")
	}
	if bundle.Manifest.Name != "schemepkg" {
		t.Fatalf("expected manifest name schemepkg, got %s", bundle.Manifest.Name)
	}
	if bundle.Source() != "fetch-route-test://libs" {
		t.Fatalf("unexpected source %s", bundle.Source())
	}
	content, err := fs.ReadFile(bundle.FS(), "lib/mod.py")
	if err != nil || string(content) != "def value():\n    return 'from scheme'\n" {
		t.Fatalf("unexpected module content: %q err=%v", content, err)
	}

	// Unregistered custom schemes still fail cleanly.
	if _, err := FetchBundle("not-registered-anywhere://libs", false, ""); err == nil {
		t.Fatal("expected an error for an unregistered scheme source")
	}

	// Built-in sources keep working: a directory bundle still opens.
	dir := writeBundleDir(t)
	if _, err := FetchBundle(dir, false, ""); err != nil {
		t.Fatalf("directory FetchBundle: %v", err)
	}
}

func TestPruneCacheRemovesPfilePairs(t *testing.T) {
	cacheDir := t.TempDir()
	old := time.Now().Add(-8 * 24 * time.Hour) // beyond the 7-day TTL

	writePair := func(base string, mtime time.Time) {
		data := filepath.Join(cacheDir, base+".pfile")
		meta := filepath.Join(cacheDir, base+".meta")
		if err := os.WriteFile(data, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(meta, []byte("v\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(data, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}

	writePair("stale", old)
	writePair("fresh", time.Now())
	staleZip := filepath.Join(cacheDir, "old-zip.zip")
	if err := os.WriteFile(staleZip, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(staleZip, old, old); err != nil {
		t.Fatal(err)
	}

	if err := PruneCache(cacheDir, 0); err != nil {
		t.Fatalf("PruneCache: %v", err)
	}

	for _, gone := range []string{"stale.pfile", "stale.meta", "old-zip.zip", "old-zip.meta"} {
		if _, err := os.Stat(filepath.Join(cacheDir, gone)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("expected %s to be pruned", gone)
		}
	}
	for _, kept := range []string{"fresh.pfile", "fresh.meta"} {
		if _, err := os.Stat(filepath.Join(cacheDir, kept)); err != nil {
			t.Errorf("expected %s to survive: %v", kept, err)
		}
	}
}

// erroringFS serves one file and fails every other read with a storage
// error that is not fs.ErrNotExist.
type erroringFS struct {
	serve map[string]string
}

func (e erroringFS) Open(name string) (fs.File, error) {
	data, err := e.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return &openStringFile{name: name, data: data}, nil
}

func (e erroringFS) ReadFile(name string) ([]byte, error) {
	if content, ok := e.serve[name]; ok {
		return []byte(content), nil
	}
	return nil, errors.New("storage backend unavailable")
}

type openStringFile struct {
	name string
	data []byte
}

func (f *openStringFile) Stat() (fs.FileInfo, error) { return nil, errors.New("unsupported") }
func (f *openStringFile) Read([]byte) (int, error)   { return 0, errors.New("unsupported") }
func (f *openStringFile) Close() error               { return nil }

func TestLoaderDistinguishesFetchErrorsFromMisses(t *testing.T) {
	healthy, err := OpenBundle(fstest.MapFS{
		"manifest.toml": &fstest.MapFile{Data: []byte("name = \"okpkg\"\nversion = \"1.0.0\"\n")},
		"lib/here.py":   &fstest.MapFile{Data: []byte("x = 1\n")},
	}, "okpkg://libs")
	if err != nil {
		t.Fatalf("OpenBundle healthy: %v", err)
	}
	erroring, err := OpenBundle(erroringFS{serve: map[string]string{
		"manifest.toml": "name = \"errpkg\"\nversion = \"1.0.0\"\n",
	}}, "errpkg://libs")
	if err != nil {
		t.Fatalf("OpenBundle erroring: %v", err)
	}

	// On healthy bundles a miss stays a quiet not-found.
	quiet := NewLoader()
	if err := quiet.AddBundle(healthy); err != nil {
		t.Fatalf("AddBundle: %v", err)
	}
	if _, found, err := quiet.Load("absent"); err != nil || found {
		t.Fatalf("Load(absent) = found=%v err=%v, want found=false err=nil", found, err)
	}
	if _, found, err := quiet.Load("here"); err != nil || !found {
		t.Fatalf("Load(here) = found=%v err=%v, want found=true err=nil", found, err)
	}

	// A storage error aborts the search and names the source, instead of
	// masquerading as "module does not exist".
	loader := NewLoader()
	if err := loader.AddBundle(erroring); err != nil {
		t.Fatalf("AddBundle erroring: %v", err)
	}
	if err := loader.AddBundle(healthy); err != nil {
		t.Fatalf("AddBundle healthy: %v", err)
	}
	_, found, err := loader.Load("absent")
	if err == nil || found {
		t.Fatalf("Load(absent) = found=%v err=%v, want a loud error", found, err)
	}
	if !strings.Contains(err.Error(), "errpkg://libs") {
		t.Fatalf("expected the error to name the package source, got %v", err)
	}

	// A higher-priority healthy bundle still answers before the erroring one
	// is consulted.
	if _, found, err := loader.Load("here"); err != nil || !found {
		t.Fatalf("Load(here) = found=%v err=%v, want the healthy bundle to shield it", found, err)
	}
}
