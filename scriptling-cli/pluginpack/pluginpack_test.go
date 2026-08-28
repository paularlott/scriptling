package pluginpack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

// mutableFetcher serves a virtual package whose contents tests can change
// mid-run, recording every read (and the validators the host sent) so cache
// and staleness behavior is observable.
type mutableFetcher struct {
	mu      sync.Mutex
	source  string
	files   map[string]string
	scripts map[string]string
	reads   map[string]int
	sent    map[string]string // path → validator most recently sent by the host
}

func newMutableFetcher(source string, files, scripts map[string]string) *mutableFetcher {
	return &mutableFetcher{
		source:  source,
		files:   files,
		scripts: scripts,
		reads:   map[string]int{},
		sent:    map[string]string{},
	}
}

func (f *mutableFetcher) validator(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:8])
}

func (f *mutableFetcher) Read(ctx context.Context, source, path, etag, lastModified string) (plugin.FetchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := path
	content, ok := "", false
	if path == "" {
		content, ok = f.scripts[source]
		key = "(script)"
	} else if source == f.source {
		content, ok = f.files[path]
	}
	if !ok {
		return plugin.FetchResult{}, fmt.Errorf("%w: %s in %s", plugin.ErrFetchNotFound, path, source)
	}
	f.reads[key]++
	f.sent[key] = etag
	validator := f.validator(content)
	if etag != "" && etag == validator {
		return plugin.FetchResult{NotModified: true, ETag: validator}, nil
	}
	return plugin.FetchResult{Data: []byte(content), ETag: validator}, nil
}

func (f *mutableFetcher) List(ctx context.Context, source, path string) ([]plugin.FetchEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if source != f.source {
		return nil, fmt.Errorf("%w: %s", plugin.ErrFetchNotFound, source)
	}
	if path == "" {
		path = "."
	}
	prefix := ""
	if path != "." {
		prefix = path + "/"
	}
	seen := map[string]bool{}
	isDir := map[string]bool{}
	for name := range f.files {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := name[len(prefix):]
		base, _, nested := strings.Cut(rest, "/")
		seen[base] = true
		isDir[base] = isDir[base] || nested
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("%w: %s in %s", plugin.ErrFetchNotFound, path, source)
	}
	entries := make([]plugin.FetchEntry, 0, len(seen))
	for base := range seen {
		entries = append(entries, plugin.FetchEntry{Name: base, IsDir: isDir[base]})
	}
	return entries, nil
}

func (f *mutableFetcher) set(path, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[path] = content
}

func (f *mutableFetcher) readsOf(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads[path]
}

func (f *mutableFetcher) sentValidator(path string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sent[path]
}

// servePlugin mounts a plugin server with the fetcher on an in-process HTTP
// endpoint and loads it into a manager, mirroring what the CLI does.
func servePlugin(t *testing.T, scheme string, fetcher plugin.Fetcher) *plugin.Manager {
	t.Helper()
	server := plugin.NewServer(t.Name(), "1.0.0", "pluginpack test plugin")
	server.RegisterFetcher(scheme, fetcher)
	server.DeclarePackage(scheme + "://libs")
	httpSrv := httptest.NewServer(server)
	t.Cleanup(httpSrv.Close)

	manager := plugin.NewManager(nil)
	t.Cleanup(func() { _ = manager.Close() })
	if _, err := manager.LoadURL(context.Background(), "pp"+scheme, httpSrv.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	return manager
}

var testFiles = map[string]string{
	"manifest.toml":          "name = \"ppkg\"\nversion = \"1.0.0\"\nlibs = [\"lib\"]\n",
	"lib/hello.py":           "def value():\n    return 'hello-v1'\n",
	"lib/nested/__init__.py": "nested_value = 'nested'\n",
	"docs/never-imported.md": "# never imported\n",
}

func TestFetchBundleServesPackageOnDemand(t *testing.T) {
	fetcher := newMutableFetcher("ppond://libs", testFiles, nil)
	manager := servePlugin(t, "ppond", fetcher)
	if err := Register(manager); err != nil {
		t.Fatalf("Register: %v", err)
	}

	cacheDir := t.TempDir()
	bundle, err := pack.FetchBundle("ppond://libs", false, cacheDir)
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}
	if bundle.Manifest.Name != "ppkg" {
		t.Fatalf("expected manifest name ppkg, got %s", bundle.Manifest.Name)
	}

	// Import modules from the plugin-served bundle through a real interpreter.
	p := scriptling.New()
	loader := pack.NewLoader()
	if err := loader.AddBundle(bundle); err != nil {
		t.Fatalf("AddBundle: %v", err)
	}
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)

	result, err := p.Eval("import hello\nhello.value()")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "hello-v1" {
		t.Fatalf("expected hello-v1, got %s", result.Inspect())
	}
	result, err = p.Eval("import nested\nnested.nested_value")
	if err != nil {
		t.Fatalf("eval nested: %v", err)
	}
	if result.Inspect() != "nested" {
		t.Fatalf("expected nested, got %s", result.Inspect())
	}

	// On demand: only the manifest and the imported modules were fetched.
	if got := fetcher.readsOf("manifest.toml"); got != 1 {
		t.Errorf("manifest reads = %d, want 1", got)
	}
	if got := fetcher.readsOf("lib/hello.py"); got != 1 {
		t.Errorf("lib/hello.py reads = %d, want 1", got)
	}
	if got := fetcher.readsOf("lib/nested/__init__.py"); got != 1 {
		t.Errorf("lib/nested/__init__.py reads = %d, want 1", got)
	}
	if got := fetcher.readsOf("docs/never-imported.md"); got != 0 {
		t.Errorf("docs/never-imported.md reads = %d, want 0", got)
	}

	// The fs surface works too: stat, readdir, sub.
	if _, err := fs.Stat(bundle.FS(), "lib"); err != nil {
		t.Errorf("stat lib: %v", err)
	}
	entries, err := fs.ReadDir(bundle.FS(), ".")
	if err != nil || len(entries) != 3 {
		t.Errorf("readdir . = %d entries (err %v), want 3 (lib, docs, manifest.toml)", len(entries), err)
	}
	if sub, ok := bundle.Sub("lib"); !ok {
		t.Error("expected Sub(lib) to exist")
	} else if _, err := fs.ReadFile(sub, "hello.py"); err != nil {
		t.Errorf("read via sub: %v", err)
	}
}

func TestCacheRevalidatesAndRefreshes(t *testing.T) {
	fetcher := newMutableFetcher("ppcache://libs", map[string]string{
		"manifest.toml": testFiles["manifest.toml"],
		"lib/data.py":   "def value():\n    return 'data-v1'\n",
	}, nil)
	manager := servePlugin(t, "ppcache", fetcher)
	if err := Register(manager); err != nil {
		t.Fatalf("Register: %v", err)
	}
	cacheDir := t.TempDir()

	readViaBundle := func() string {
		t.Helper()
		bundle, err := pack.FetchBundle("ppcache://libs", false, cacheDir)
		if err != nil {
			t.Fatalf("FetchBundle: %v", err)
		}
		content, err := bundle.ReadFile("lib/data.py")
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		return string(content)
	}

	// First read: unconditional, cached to disk.
	if got := readViaBundle(); !strings.Contains(got, "data-v1") {
		t.Fatalf("first read got %q", got)
	}
	if got := fetcher.readsOf("lib/data.py"); got != 1 {
		t.Fatalf("reads after first = %d, want 1", got)
	}
	if err := filepath.WalkDir(cacheDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".pfile") {
			return nil
		}
		return nil
	}); err != nil {
		t.Fatalf("walk cache: %v", err)
	}
	if !cacheHasPfile(cacheDir) {
		t.Fatal("expected a .pfile cache entry on disk")
	}

	// Second read (fresh bundle + FS, same cache): conditional — the peer
	// receives the stored validator and answers not_modified; bytes come from
	// the local cache.
	if got := readViaBundle(); !strings.Contains(got, "data-v1") {
		t.Fatalf("second read got %q", got)
	}
	if got := fetcher.readsOf("lib/data.py"); got != 2 {
		t.Fatalf("reads after second = %d, want 2 (one conditional RPC)", got)
	}
	if v := fetcher.sentValidator("lib/data.py"); v == "" {
		t.Fatal("expected the peer to receive the cached validator")
	}

	// Change the content: the validator differs, fresh bytes replace the cache.
	fetcher.set("lib/data.py", "def value():\n    return 'data-v2'\n")
	if got := readViaBundle(); !strings.Contains(got, "data-v2") {
		t.Fatalf("third read got %q, want v2", got)
	}
	if got := fetcher.readsOf("lib/data.py"); got != 3 {
		t.Fatalf("reads after third = %d, want 3", got)
	}
}

func cacheHasPfile(cacheDir string) bool {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pfile") {
			return true
		}
	}
	return false
}

func TestFetchScriptAlwaysRefetches(t *testing.T) {
	fetcher := newMutableFetcher("ppscript://x", map[string]string{
		"manifest.toml": testFiles["manifest.toml"],
	}, map[string]string{
		"ppscript://run/hello": "print('script-v1')\n",
	})
	manager := servePlugin(t, "ppscript", fetcher)
	if err := Register(manager); err != nil {
		t.Fatalf("Register: %v", err)
	}

	content, err := FetchScript("ppscript://run/hello")
	if err != nil {
		t.Fatalf("FetchScript: %v", err)
	}
	if !strings.Contains(string(content), "script-v1") {
		t.Fatalf("got %q", content)
	}

	// Scripts bypass the cache: an edit is visible on the very next fetch.
	fetcher.mu.Lock()
	fetcher.scripts["ppscript://run/hello"] = "print('script-v2')\n"
	fetcher.mu.Unlock()
	content, err = FetchScript("ppscript://run/hello")
	if err != nil {
		t.Fatalf("FetchScript again: %v", err)
	}
	if !strings.Contains(string(content), "script-v2") {
		t.Fatalf("expected the edited script, got %q", content)
	}
	if got := fetcher.readsOf("(script)"); got != 2 {
		t.Fatalf("script reads = %d, want 2", got)
	}
}

func TestModuleMissIsNotAnError(t *testing.T) {
	fetcher := newMutableFetcher("ppmiss://libs", map[string]string{
		"manifest.toml": testFiles["manifest.toml"],
		"lib/here.py":   "x = 1\n",
	}, nil)
	manager := servePlugin(t, "ppmiss", fetcher)
	if err := Register(manager); err != nil {
		t.Fatalf("Register: %v", err)
	}

	bundle, err := pack.FetchBundle("ppmiss://libs", false, t.TempDir())
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}

	// At the FS layer a miss is fs.ErrNotExist.
	if _, err := bundle.ReadFile("lib/absent.py"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got %v", err)
	}

	// At the loader layer a miss is (found=false), never a fatal error.
	loader := pack.NewLoader()
	if err := loader.AddBundle(bundle); err != nil {
		t.Fatalf("AddBundle: %v", err)
	}
	if _, found, err := loader.Load("absent"); err != nil || found {
		t.Fatalf("Load(absent) = found=%v err=%v, want found=false err=nil", found, err)
	}
	if _, found, err := loader.Load("here"); err != nil || !found {
		t.Fatalf("Load(here) = found=%v err=%v, want found=true err=nil", found, err)
	}
}

func TestRegisterRejectsSchemeConflicts(t *testing.T) {
	fetcher := newMutableFetcher("ppconf://libs", map[string]string{
		"manifest.toml": testFiles["manifest.toml"],
	}, nil)
	manager := servePlugin(t, "ppconf", fetcher)
	if err := Register(manager); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// A second plugin claiming the same scheme is a hard error.
	other := servePlugin(t, "ppconf", fetcher)
	if err := Register(other); err == nil {
		t.Fatal("expected a scheme conflict error")
	}
}

func TestFetchScriptUnknownScheme(t *testing.T) {
	if _, err := FetchScript("nowhere://scripts/foo"); err == nil {
		t.Fatal("expected an error for a source with no fetcher plugin")
	}
}

func TestDeclaredPackages(t *testing.T) {
	fetcher := newMutableFetcher("ppdecl://libs", testFiles, nil)
	manager := servePlugin(t, "ppdecl", fetcher)
	if err := Register(manager); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got := DeclaredPackages(manager); len(got) != 1 || got[0] != "ppdecl://libs" {
		t.Fatalf("expected [ppdecl://libs], got %v", got)
	}

	// A second plugin that declares its package (and a duplicate of another's).
	declServer := plugin.NewServer("declarer", "1.0.0", "declares a package")
	declServer.RegisterFetcher("ppdecl2", fetcher)
	declServer.DeclarePackage("ppdecl2://libs")
	declServer.DeclarePackage("ppdecl://libs") // duplicate of another plugin's
	httpSrv := httptest.NewServer(declServer)
	t.Cleanup(httpSrv.Close)
	if _, err := manager.LoadURL(context.Background(), "declarer", httpSrv.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}

	got := DeclaredPackages(manager)
	if len(got) != 2 || got[0] != "ppdecl2://libs" || got[1] != "ppdecl://libs" {
		t.Fatalf("expected sorted [ppdecl2://libs ppdecl://libs] deduplicated, got %v", got)
	}
}

func TestAutoAttachedPackageImportsWithoutExplicitPackage(t *testing.T) {
	fetcher := newMutableFetcher("ppauto://libs", testFiles, nil)
	manager := servePlugin(t, "ppauto", fetcher)
	if err := Register(manager); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// The CLI's auto-attach flow: open every declared package and add it to
	// the loader before any explicit --package bundles.
	loader := pack.NewLoader()
	for _, src := range DeclaredPackages(manager) {
		b, err := pack.FetchBundle(src, false, t.TempDir())
		if err != nil {
			t.Fatalf("FetchBundle(%s): %v", src, err)
		}
		if err := loader.AddBundle(b); err != nil {
			t.Fatalf("AddBundle: %v", err)
		}
	}

	p := scriptling.New()
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)

	result, err := p.Eval("import hello\nhello.value()")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "hello-v1" {
		t.Fatalf("expected hello-v1, got %s", result.Inspect())
	}
	if got := fetcher.readsOf("docs/never-imported.md"); got != 0 {
		t.Fatalf("auto-attached packages stay on demand, docs read %d times", got)
	}
}

func TestExplicitPackageShadowsAutoAttached(t *testing.T) {
	autoFetcher := newMutableFetcher("ppshadow-auto://libs", map[string]string{
		"manifest.toml": "name = \"shadow-auto\"\nversion = \"1.0.0\"\nlibs = [\"lib\"]\n",
		"lib/which.py":  "def value():\n    return 'from auto'\n",
	}, nil)
	autoManager := servePlugin(t, "ppshadow-auto", autoFetcher)
	if err := Register(autoManager); err != nil {
		t.Fatalf("Register: %v", err)
	}

	explicitFetcher := newMutableFetcher("ppshadow-explicit://libs", map[string]string{
		"manifest.toml": "name = \"shadow-explicit\"\nversion = \"1.0.0\"\nlibs = [\"lib\"]\n",
		"lib/which.py":  "def value():\n    return 'from explicit'\n",
	}, nil)
	explicitManager := servePlugin(t, "ppshadow-explicit", explicitFetcher)
	if err := Register(explicitManager); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Mirror the CLI ordering: auto bundles first, explicit --package bundles
	// after (the loader searches last-added first).
	loader := pack.NewLoader()
	for _, src := range DeclaredPackages(autoManager) {
		b, err := pack.FetchBundle(src, false, t.TempDir())
		if err != nil {
			t.Fatalf("FetchBundle(%s): %v", src, err)
		}
		if err := loader.AddBundle(b); err != nil {
			t.Fatalf("AddBundle: %v", err)
		}
	}
	b, err := pack.FetchBundle("ppshadow-explicit://libs", false, t.TempDir())
	if err != nil {
		t.Fatalf("FetchBundle explicit: %v", err)
	}
	if err := loader.AddBundle(b); err != nil {
		t.Fatalf("AddBundle explicit: %v", err)
	}

	p := scriptling.New()
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)

	result, err := p.Eval("import which\nwhich.value()")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "from explicit" {
		t.Fatalf("expected the explicit package to shadow the auto-attached one, got %s", result.Inspect())
	}
}

func TestDeclaredBundles(t *testing.T) {
	fetcher := newMutableFetcher("ppbundles://libs", testFiles, nil)
	manager := servePlugin(t, "ppbundles", fetcher)
	if err := Register(manager); err != nil {
		t.Fatalf("Register: %v", err)
	}

	bundles, err := DeclaredBundles(manager, false, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("DeclaredBundles: %v", err)
	}
	if len(bundles) != 1 || bundles[0].Manifest.Name != "ppkg" {
		t.Fatalf("expected the declared package as a bundle, got %+v", bundles)
	}

	// Explicitly passed sources are skipped: their PreRun-opened bundle is
	// used instead of opening a duplicate.
	bundles, err = DeclaredBundles(manager, false, t.TempDir(), map[string]bool{"ppbundles://libs": true})
	if err != nil {
		t.Fatalf("DeclaredBundles with skip: %v", err)
	}
	if len(bundles) != 0 {
		t.Fatalf("expected the skipped source to be omitted, got %+v", bundles)
	}
}

// failingFetcher serves its manifest but fails every other read with a
// plain error (not ErrFetchNotFound), simulating an unreachable backend.
type failingFetcher struct {
	inner *mutableFetcher
}

func (f failingFetcher) Read(ctx context.Context, source, path, etag, lastModified string) (plugin.FetchResult, error) {
	if path == "manifest.toml" {
		return f.inner.Read(ctx, source, path, etag, lastModified)
	}
	return plugin.FetchResult{}, errors.New("knot server unreachable: connection refused")
}

func (f failingFetcher) List(ctx context.Context, source, path string) ([]plugin.FetchEntry, error) {
	return f.inner.List(ctx, source, path)
}

// TestPluginFailureAbortsImportLoudly proves the not-found vs cannot-reach
// split across the whole protocol: a plugin whose fetcher errors (rather
// than answering not-found) makes the import fail with the source named,
// while a plain miss stays a quiet unknown-library.
func TestPluginFailureAbortsImportLoudly(t *testing.T) {
	base := newMutableFetcher("ppfail://libs", map[string]string{
		"manifest.toml": testFiles["manifest.toml"],
	}, nil)
	manager := servePlugin(t, "ppfail", failingFetcher{inner: base})
	if err := Register(manager); err != nil {
		t.Fatalf("Register: %v", err)
	}

	bundles, err := DeclaredBundles(manager, false, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("DeclaredBundles: %v", err)
	}
	loader := pack.NewLoader()
	for _, b := range bundles {
		if err := loader.AddBundle(b); err != nil {
			t.Fatalf("AddBundle: %v", err)
		}
	}
	p := scriptling.New()
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)

	_, err = p.Eval("import anything")
	if err == nil {
		t.Fatal("expected the import to fail when the fetcher backend errors")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ppfail://libs") {
		t.Fatalf("expected the error to name the package source, got: %s", msg)
	}
	if !strings.Contains(msg, "unreachable") {
		t.Fatalf("expected the underlying failure to surface, got: %s", msg)
	}
}
