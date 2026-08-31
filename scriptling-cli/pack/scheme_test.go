package pack

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

// testRegistry keeps scheme tests isolated from the process-wide default.
var testRegistry = NewSchemeRegistry()

func TestRegisterSchemeValidation(t *testing.T) {
	opener := func(source string, insecure bool, cacheDir string) (*Bundle, error) {
		return nil, errors.New("not called")
	}
	for _, scheme := range []string{"http", "https", "file", "", "1starts-with-digit", "has/slash", "has space"} {
		if err := testRegistry.Register(scheme, opener); err == nil {
			t.Errorf("expected rejection for scheme %q", scheme)
		}
	}
	if err := testRegistry.Register("nil-opener-test", nil); err == nil {
		t.Error("expected rejection for nil opener")
	}
	if err := testRegistry.Register("dup-scheme-test", opener); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	if err := testRegistry.Register("dup-scheme-test", opener); err == nil {
		t.Error("expected duplicate registration to fail")
	}
}

func TestSchemeSyntaxAndRouting(t *testing.T) {
	if err := testRegistry.Register("scheme-for-test", func(source string, insecure bool, cacheDir string) (*Bundle, error) {
		return nil, errors.New("not called")
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cases := []struct {
		source string
		scheme string
		syntax bool
	}{
		{"scheme-for-test://libs", "scheme-for-test", true},
		{"scheme-for-test://", "", false}, // empty rest
		{"scheme-for-test ", "", false},   // no ://
		{"unregistered-scheme://libs", "unregistered-scheme", true},
		{"http://example.com/x.zip", "", false},  // built-in
		{"https://example.com/x.zip", "", false}, // built-in
		{"/local/path/pkg.zip", "", false},       // local path
		{"relative/pkg.zip", "", false},          // local path
		{"1bad://libs", "", false},               // invalid scheme grammar
		{"scheme-for-test://lib s", "", false},   // whitespace in rest
	}
	for _, tc := range cases {
		scheme, ok := SchemeSyntax(tc.source)
		if scheme != tc.scheme || ok != tc.syntax {
			t.Errorf("SchemeSyntax(%q) = (%q, %v), want (%q, %v)", tc.source, scheme, ok, tc.scheme, tc.syntax)
		}
	}

	// Routing is a registry question: the opener runs for a registered
	// scheme, and an unknown scheme-shaped source fails with the sentinel
	// naming the scheme.
	if _, err := testRegistry.FetchBundle("scheme-for-test://libs", false, t.TempDir()); err == nil || err.Error() != "not called" {
		t.Errorf("expected the opener to run, got %v", err)
	}
	if _, err := testRegistry.FetchBundle("unregistered-scheme://libs", false, ""); !errors.Is(err, ErrUnknownScheme) {
		t.Errorf("expected ErrUnknownScheme, got %v", err)
	}
}

func TestFetchBundleRoutesCustomScheme(t *testing.T) {
	mapFS := fstest.MapFS{
		"manifest.toml": &fstest.MapFile{Data: []byte("name = \"schemepkg\"\nversion = \"1.0.0\"\n")},
		"lib/mod.py":    &fstest.MapFile{Data: []byte("def value():\n    return 'from scheme'\n")},
	}
	called := false
	if err := testRegistry.Register("fetch-route-test", func(source string, insecure bool, cacheDir string) (*Bundle, error) {
		called = true
		if source != "fetch-route-test://libs" {
			t.Errorf("opener received source %q", source)
		}
		return OpenBundle(mapFS, source)
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	bundle, err := testRegistry.FetchBundle("fetch-route-test://libs", false, t.TempDir())
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
	if _, err := testRegistry.FetchBundle("not-registered-anywhere://libs", false, ""); err == nil {
		t.Fatal("expected an error for an unregistered scheme source")
	}

	// Built-in sources keep working: a directory bundle still opens.
	dir := writeBundleDir(t)
	if _, err := FetchBundle(dir, false, ""); err != nil {
		t.Fatalf("directory FetchBundle: %v", err)
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

func TestSchemeSyntaxIgnoresRegistration(t *testing.T) {
	// SchemeSyntax is what lets callers tell "not a scheme" apart from
	// "scheme with no plugin loaded", so it must not consult the registry.
	cases := []struct {
		source string
		scheme string
		ok     bool
	}{
		{"knot://libs", "knot", true},
		{"never-registered://scripts/x", "never-registered", true},
		{"a+b-c.d://x", "a+b-c.d", true},
		{"knot://", "", false},
		{"knot", "", false},
		{"http://example.com/x.zip", "", false},
		{"https://example.com/x.zip", "", false},
		{"file://x", "", false},
		{"/local/path.zip", "", false},
		{"relative/path.zip", "", false},
		{"1bad://libs", "", false},
		{"knot://lib s", "", false},
		{"knot://lib\tx", "", false},
	}
	for _, tc := range cases {
		scheme, ok := SchemeSyntax(tc.source)
		if scheme != tc.scheme || ok != tc.ok {
			t.Errorf("SchemeSyntax(%q) = (%q, %v), want (%q, %v)", tc.source, scheme, ok, tc.scheme, tc.ok)
		}
	}
}

func TestUnregisterSchemeAllowsReclaim(t *testing.T) {
	opener := func(source string, insecure bool, cacheDir string) (*Bundle, error) {
		return nil, errors.New("not called")
	}
	if err := testRegistry.Register("reclaim-test", opener); err != nil {
		t.Fatalf("register: %v", err)
	}
	if testRegistry.Lookup("reclaim-test") == nil {
		t.Fatal("expected the scheme to resolve after registration")
	}
	if err := testRegistry.Register("reclaim-test", opener); err == nil {
		t.Fatal("expected a duplicate registration to fail")
	}

	if !testRegistry.Unregister("reclaim-test") {
		t.Fatal("expected Unregister to report the scheme was registered")
	}
	if testRegistry.Unregister("reclaim-test") {
		t.Fatal("expected a second Unregister to report nothing was registered")
	}
	if testRegistry.Lookup("reclaim-test") != nil {
		t.Fatal("expected the scheme to stop resolving after Unregister")
	}

	// Reclaimable: this is what lets a host swap plugins at runtime.
	if err := testRegistry.Register("reclaim-test", opener); err != nil {
		t.Fatalf("re-register after Unregister: %v", err)
	}
	testRegistry.Unregister("reclaim-test")
}

func TestUnknownSchemeErrorIsActionable(t *testing.T) {
	_, err := FetchBundle("no-such-plugin-scheme://libs", false, t.TempDir())
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()

	// Callers detect the cause without matching on message text.
	if !errors.Is(err, ErrUnknownScheme) {
		t.Errorf("expected the error to wrap ErrUnknownScheme, got: %v", err)
	}
	// It must name the scheme and say what to do, rather than falling through
	// to a missing-file error.
	if !strings.Contains(msg, "no-such-plugin-scheme") {
		t.Errorf("expected the scheme named, got: %v", err)
	}
	if !strings.Contains(msg, "load the plugin that serves it") {
		t.Errorf("expected the fix described, got: %v", err)
	}
	if strings.Contains(msg, "no such file") {
		t.Errorf("expected a plugin error, not a file error: %v", err)
	}
	// This package is shared by the CLI and by embedding hosts, so it must not
	// hand out CLI-specific advice. The CLI adds the flag names itself.
	for _, cliOnly := range []string{"--plugin", "--plugin-dir", "CLI"} {
		if strings.Contains(msg, cliOnly) {
			t.Errorf("library error mentions %q, which is wrong advice for an embedding host: %v", cliOnly, err)
		}
	}
}

func TestIsolatedRegistriesAreIndependent(t *testing.T) {
	mapFS := fstest.MapFS{
		"manifest.toml": &fstest.MapFile{Data: []byte("name = \"isopkg\"\nversion = \"1.0.0\"\n")},
	}
	first := NewSchemeRegistry()
	second := NewSchemeRegistry()
	opener := func(source string, insecure bool, cacheDir string) (*Bundle, error) {
		return OpenBundle(mapFS, source)
	}

	// The same scheme in two registries is fine: they are separate tables.
	if err := first.Register("iso-test", opener); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := second.Register("iso-test", opener); err != nil {
		t.Fatalf("second register: %v", err)
	}

	if _, err := first.FetchBundle("iso-test://libs", false, ""); err != nil {
		t.Fatalf("first FetchBundle: %v", err)
	}
	// Neither leaks into the process-wide registry.
	if DefaultSchemeRegistry().Lookup("iso-test") != nil {
		t.Fatal("an isolated registry must not affect the default one")
	}
	if _, err := FetchBundle("iso-test://libs", false, ""); err == nil {
		t.Fatal("expected the default registry not to resolve an isolated scheme")
	}

	if got := first.Registered(); len(got) != 1 || got[0] != "iso-test" {
		t.Fatalf("Registered() = %v, want [iso-test]", got)
	}
	if first.Lookup("iso-test") == nil {
		t.Fatal("expected Lookup to return the opener")
	}
	if first.Lookup("absent") != nil {
		t.Fatal("expected Lookup to return nil for an unregistered scheme")
	}

	// Built-in sources still work through an isolated registry.
	dir := writeBundleDir(t)
	if _, err := first.FetchBundle(dir, false, ""); err != nil {
		t.Fatalf("directory FetchBundle through an isolated registry: %v", err)
	}

	first.Unregister("iso-test")
	if _, err := first.FetchBundle("iso-test://libs", false, ""); err == nil {
		t.Fatal("expected the unregistered scheme to fail")
	}
	// The second registry is untouched by the first's Unregister.
	if _, err := second.FetchBundle("iso-test://libs", false, ""); err != nil {
		t.Fatalf("second registry should be unaffected: %v", err)
	}
	second.Unregister("iso-test")
}

func TestRegisteredSchemesSorted(t *testing.T) {
	r := NewSchemeRegistry()
	opener := func(source string, insecure bool, cacheDir string) (*Bundle, error) {
		return nil, errors.New("not called")
	}
	for _, s := range []string{"zeta", "alpha", "mid"} {
		if err := r.Register(s, opener); err != nil {
			t.Fatalf("register %s: %v", s, err)
		}
	}
	got := r.Registered()
	want := []string{"alpha", "mid", "zeta"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Registered() = %v, want %v", got, want)
	}
}
