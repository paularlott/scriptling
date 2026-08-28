package pluginpack

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

// mutableFetcher serves a virtual package whose contents tests can change
// mid-run, counting reads and lists per path so on-demand and re-read behavior
// is observable.
type mutableFetcher struct {
	mu      sync.Mutex
	source  string
	files   map[string]string
	scripts map[string]string
	reads   map[string]int
	lists   map[string]int
	blockCh chan struct{} // when non-nil, Read waits on it (or ctx) before answering
}

func newMutableFetcher(source string, files, scripts map[string]string) *mutableFetcher {
	return &mutableFetcher{
		source:  source,
		files:   files,
		scripts: scripts,
		reads:   map[string]int{},
		lists:   map[string]int{},
	}
}

func (f *mutableFetcher) Read(ctx context.Context, source, path string) (plugin.FetchResult, error) {
	f.mu.Lock()
	block := f.blockCh
	f.mu.Unlock()
	if block != nil {
		// Hold the call open so a caller can cancel its context mid-fetch.
		select {
		case <-block:
		case <-ctx.Done():
			return plugin.FetchResult{}, ctx.Err()
		}
	}

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
	return plugin.FetchResult{Data: []byte(content)}, nil
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
	f.lists[path]++
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

func (f *mutableFetcher) listsOf(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lists[path]
}

func (f *mutableFetcher) block() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blockCh = make(chan struct{})
}

// servePlugin mounts a plugin server with the fetcher on an in-process HTTP
// endpoint and loads it into a manager, mirroring what a host does.
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

// bridgeFor builds a registered Bridge over a manager and releases its schemes
// when the test ends, so schemes never leak between tests.
func bridgeFor(t *testing.T, manager *plugin.Manager, cacheDir string) *Bridge {
	t.Helper()
	bridge := New(Options{Manager: manager, CacheDir: cacheDir})
	if err := bridge.Register(); err != nil {
		t.Fatalf("Bridge.Register: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })
	return bridge
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
	cacheDir := t.TempDir()
	bridge := bridgeFor(t, manager, cacheDir)

	bundle, err := pack.FetchBundle("ppond://libs", false, cacheDir)
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}
	if bundle.Manifest.Name != "ppkg" {
		t.Fatalf("expected manifest name ppkg, got %s", bundle.Manifest.Name)
	}
	if got := bridge.Schemes(); len(got) != 1 || got[0] != "ppond" {
		t.Fatalf("expected the bridge to own [ppond], got %v", got)
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

// TestContentIsNeverWrittenToDisk is the storage contract: the host keeps
// plugin-served bytes in memory for the lifetime of the bundle's file system
// and writes nothing to the package cache. Persisting what a plugin serves is
// the plugin's business — it owns the backend, credentials and freshness rules.
func TestContentIsNeverWrittenToDisk(t *testing.T) {
	fetcher := newMutableFetcher("ppnodisk://libs", testFiles, nil)
	manager := servePlugin(t, "ppnodisk", fetcher)
	cacheDir := t.TempDir()
	bridgeFor(t, manager, cacheDir)

	bundle, err := pack.FetchBundle("ppnodisk://libs", false, cacheDir)
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}
	for _, name := range []string{"manifest.toml", "lib/hello.py", "lib/nested/__init__.py"} {
		if _, err := bundle.ReadFile(name); err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
	}
	if _, err := fs.ReadDir(bundle.FS(), "."); err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	// The cache directory must be untouched — no .pfile, no .meta, nothing.
	entries, err := os.ReadDir(cacheDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read cache dir: %v", err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("plugin content was persisted to disk: %v", names)
	}
}

// TestContentIsNotRetained: content is never held, so every read is a fetch and
// an edit is always visible on the next read.
func TestContentIsNotRetained(t *testing.T) {
	fetcher := newMutableFetcher("ppmem://libs", map[string]string{
		"manifest.toml": testFiles["manifest.toml"],
		"lib/data.py":   "def value():\n    return 'data-v1'\n",
	}, nil)
	manager := servePlugin(t, "ppmem", fetcher)
	bridgeFor(t, manager, t.TempDir())

	bundle, err := pack.FetchBundle("ppmem://libs", false, t.TempDir())
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}
	read := func() string {
		t.Helper()
		content, err := bundle.ReadFile("lib/data.py")
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		return string(content)
	}

	// Every read is a fetch — nothing is served from a cache.
	for i := 1; i <= 3; i++ {
		if got := read(); !strings.Contains(got, "data-v1") {
			t.Fatalf("read %d got %q", i, got)
		}
		if got := fetcher.readsOf("lib/data.py"); got != i {
			t.Fatalf("reads after %d = %d, want %d", i, got, i)
		}
	}

	// An edit is visible immediately, with no invalidation step.
	fetcher.set("lib/data.py", "def value():\n    return 'data-v2'\n")
	if got := read(); !strings.Contains(got, "data-v2") {
		t.Fatalf("expected the edit to be visible at once, got %q", got)
	}
}

func TestFetchScriptAlwaysRefetches(t *testing.T) {
	fetcher := newMutableFetcher("ppscript://x", map[string]string{
		"manifest.toml": testFiles["manifest.toml"],
	}, map[string]string{
		"ppscript://run/hello": "print('script-v1')\n",
	})
	manager := servePlugin(t, "ppscript", fetcher)
	bridge := bridgeFor(t, manager, t.TempDir())

	content, err := bridge.FetchScript(context.Background(), "ppscript://run/hello")
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
	content, err = bridge.FetchScript(context.Background(), "ppscript://run/hello")
	if err != nil {
		t.Fatalf("FetchScript again: %v", err)
	}
	if !strings.Contains(string(content), "script-v2") {
		t.Fatalf("expected the edited script, got %q", content)
	}
	if got := fetcher.readsOf("(script)"); got != 2 {
		t.Fatalf("script reads = %d, want 2", got)
	}

	// A nil context falls back to the bridge's context.
	if _, err := bridge.FetchScript(nil, "ppscript://run/hello"); err != nil {
		t.Fatalf("FetchScript with nil ctx: %v", err)
	}
}

func TestModuleMissIsNotAnError(t *testing.T) {
	fetcher := newMutableFetcher("ppmiss://libs", map[string]string{
		"manifest.toml": testFiles["manifest.toml"],
		"lib/here.py":   "x = 1\n",
	}, nil)
	manager := servePlugin(t, "ppmiss", fetcher)
	bridgeFor(t, manager, t.TempDir())

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
	bridgeFor(t, manager, t.TempDir())

	// A second plugin claiming the same scheme is a hard error.
	other := servePlugin(t, "ppconf", fetcher)
	conflicting := New(Options{Manager: other})
	if err := conflicting.Register(); err == nil {
		_ = conflicting.Close()
		t.Fatal("expected a scheme conflict error")
	}
	// The failed Register claimed nothing, so it owns no schemes to release.
	if got := conflicting.Schemes(); len(got) != 0 {
		t.Fatalf("expected a failed Register to roll back, bridge owns %v", got)
	}
}

// TestCloseReleasesSchemesForReload is the embedding requirement: a host must
// be able to swap its plugins at runtime. Closing a bridge frees its schemes so
// the next bridge can claim them.
func TestCloseReleasesSchemesForReload(t *testing.T) {
	fetcher := newMutableFetcher("ppreload://libs", map[string]string{
		"manifest.toml": testFiles["manifest.toml"],
		"lib/v.py":      "v = 'first'\n",
	}, nil)
	manager := servePlugin(t, "ppreload", fetcher)

	first := New(Options{Manager: manager, CacheDir: t.TempDir()})
	if err := first.Register(); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if _, ok := pack.SchemeFor("ppreload://libs"); !ok {
		t.Fatal("expected ppreload to be registered")
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, ok := pack.SchemeFor("ppreload://libs"); ok {
		t.Fatal("expected Close to release the scheme")
	}
	// Close is idempotent.
	if err := first.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	// A fresh bridge over the same plugins claims the scheme again.
	second := New(Options{Manager: manager, CacheDir: t.TempDir()})
	if err := second.Register(); err != nil {
		t.Fatalf("re-Register after Close: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if _, err := pack.FetchBundle("ppreload://libs", false, t.TempDir()); err != nil {
		t.Fatalf("FetchBundle after reload: %v", err)
	}
}

// TestIsolatedRegistryDoesNotTouchDefault covers a host that wants its own
// routing table rather than the process-wide one.
func TestIsolatedRegistryDoesNotTouchDefault(t *testing.T) {
	fetcher := newMutableFetcher("ppiso://libs", testFiles, nil)
	manager := servePlugin(t, "ppiso", fetcher)

	registry := pack.NewSchemeRegistry()
	bridge := New(Options{Manager: manager, Registry: registry, CacheDir: t.TempDir()})
	if err := bridge.Register(); err != nil {
		t.Fatalf("Register: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })

	// The private registry resolves the source; the default one does not.
	if _, err := registry.FetchBundle("ppiso://libs", false, t.TempDir()); err != nil {
		t.Fatalf("private registry FetchBundle: %v", err)
	}
	if _, ok := pack.SchemeFor("ppiso://libs"); ok {
		t.Fatal("an isolated registry must not register on the default one")
	}
	if _, err := pack.FetchBundle("ppiso://libs", false, t.TempDir()); err == nil {
		t.Fatal("expected the default registry not to resolve an isolated scheme")
	}
}

func TestFetchScriptUnknownScheme(t *testing.T) {
	bridge := New(Options{Manager: plugin.NewManager(nil)})
	_, err := bridge.FetchScript(context.Background(), "nowhere://scripts/foo")
	if err == nil {
		t.Fatal("expected an error for a source with no fetcher plugin")
	}
	msg := err.Error()
	// Detectable by type, so callers need not match on text.
	if !errors.Is(err, pack.ErrUnknownScheme) {
		t.Errorf("expected the error to wrap pack.ErrUnknownScheme, got: %v", err)
	}
	// The message must point at the missing plugin, not a missing file.
	if !strings.Contains(msg, "nowhere") || !strings.Contains(msg, "load the plugin that serves it") {
		t.Fatalf("expected an actionable message naming the scheme, got: %v", err)
	}
	// pluginpack is used by embedding hosts too, so no CLI flag advice here.
	for _, cliOnly := range []string{"--plugin", "--plugin-dir", "CLI"} {
		if strings.Contains(msg, cliOnly) {
			t.Errorf("library error mentions %q, which is wrong advice for an embedding host: %v", cliOnly, err)
		}
	}
}

func TestDeclaredPackages(t *testing.T) {
	fetcher := newMutableFetcher("ppdecl://libs", testFiles, nil)
	manager := servePlugin(t, "ppdecl", fetcher)
	bridge := bridgeFor(t, manager, t.TempDir())
	if got := bridge.DeclaredPackages(); len(got) != 1 || got[0] != "ppdecl://libs" {
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

	got := bridge.DeclaredPackages()
	if len(got) != 2 || got[0] != "ppdecl2://libs" || got[1] != "ppdecl://libs" {
		t.Fatalf("expected sorted [ppdecl2://libs ppdecl://libs] deduplicated, got %v", got)
	}
}

func TestAutoAttachedPackageImportsWithoutExplicitPackage(t *testing.T) {
	fetcher := newMutableFetcher("ppauto://libs", testFiles, nil)
	manager := servePlugin(t, "ppauto", fetcher)
	bridge := bridgeFor(t, manager, t.TempDir())

	// The host flow: open every declared package and add it to the loader
	// before any explicit --package bundles.
	loader := pack.NewLoader()
	bundles, err := bridge.DeclaredBundles(nil)
	if err != nil {
		t.Fatalf("DeclaredBundles: %v", err)
	}
	for _, b := range bundles {
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
	autoBridge := bridgeFor(t, autoManager, t.TempDir())

	explicitFetcher := newMutableFetcher("ppshadow-explicit://libs", map[string]string{
		"manifest.toml": "name = \"shadow-explicit\"\nversion = \"1.0.0\"\nlibs = [\"lib\"]\n",
		"lib/which.py":  "def value():\n    return 'from explicit'\n",
	}, nil)
	explicitManager := servePlugin(t, "ppshadow-explicit", explicitFetcher)
	bridgeFor(t, explicitManager, t.TempDir())

	// Mirror the CLI ordering: auto bundles first, explicit --package bundles
	// after (the loader searches last-added first).
	loader := pack.NewLoader()
	autoBundles, err := autoBridge.DeclaredBundles(nil)
	if err != nil {
		t.Fatalf("DeclaredBundles: %v", err)
	}
	for _, b := range autoBundles {
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
	bridge := bridgeFor(t, manager, t.TempDir())

	bundles, err := bridge.DeclaredBundles(nil)
	if err != nil {
		t.Fatalf("DeclaredBundles: %v", err)
	}
	if len(bundles) != 1 || bundles[0].Manifest.Name != "ppkg" {
		t.Fatalf("expected the declared package as a bundle, got %+v", bundles)
	}

	// Explicitly passed sources are skipped: their already-opened bundle is
	// used instead of opening a duplicate.
	bundles, err = bridge.DeclaredBundles(map[string]bool{"ppbundles://libs": true})
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

func (f failingFetcher) Read(ctx context.Context, source, path string) (plugin.FetchResult, error) {
	if path == "manifest.toml" {
		return f.inner.Read(ctx, source, path)
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
	bridge := bridgeFor(t, manager, t.TempDir())

	bundles, err := bridge.DeclaredBundles(nil)
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

// TestDirListingsRevalidate covers the directory TTL: a long-lived host must
// see files that appear in a served directory after it first listed it.
func TestDirListingsRevalidate(t *testing.T) {
	fetcher := newMutableFetcher("ppdirttl://libs", map[string]string{
		"manifest.toml":  testFiles["manifest.toml"],
		"tools/first.py": "x = 1\n",
	}, nil)
	manager := servePlugin(t, "ppdirttl", fetcher)

	// A negative TTL disables reuse entirely, so every listing is fresh.
	bridge := New(Options{Manager: manager, CacheDir: t.TempDir(), DirTTL: -1})
	if err := bridge.Register(); err != nil {
		t.Fatalf("Register: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })

	bundle, err := pack.FetchBundle("ppdirttl://libs", false, t.TempDir())
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}
	entries, err := fs.ReadDir(bundle.FS(), "tools")
	if err != nil {
		t.Fatalf("ReadDir(tools): %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// A file appears after the first listing.
	fetcher.set("tools/second.py", "y = 2\n")
	entries, err = fs.ReadDir(bundle.FS(), "tools")
	if err != nil {
		t.Fatalf("ReadDir(tools) again: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected the new file to appear, got %d entries", len(entries))
	}
	if got := fetcher.listsOf("tools"); got != 2 {
		t.Fatalf("expected 2 fetch.list calls with reuse disabled, got %d", got)
	}
}

// TestDirListingsReusedWithinTTL is the other half: within the TTL a listing is
// served from memory, so probe-heavy work does not re-list on every call.
func TestDirListingsReusedWithinTTL(t *testing.T) {
	fetcher := newMutableFetcher("ppdirreuse://libs", map[string]string{
		"manifest.toml":  testFiles["manifest.toml"],
		"tools/first.py": "x = 1\n",
	}, nil)
	manager := servePlugin(t, "ppdirreuse", fetcher)
	bridge := New(Options{Manager: manager, CacheDir: t.TempDir(), DirTTL: time.Hour})
	if err := bridge.Register(); err != nil {
		t.Fatalf("Register: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })

	bundle, err := pack.FetchBundle("ppdirreuse://libs", false, t.TempDir())
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := fs.ReadDir(bundle.FS(), "tools"); err != nil {
			t.Fatalf("ReadDir(tools) #%d: %v", i, err)
		}
	}
	if got := fetcher.listsOf("tools"); got != 1 {
		t.Fatalf("expected 1 fetch.list within the TTL, got %d", got)
	}
}

// TestContextCancellationAbortsFetch is the ctx-propagation guarantee: a host
// that cancels its context stops in-flight fetches instead of waiting out the
// protocol timeout.
func TestContextCancellationAbortsFetch(t *testing.T) {
	fetcher := newMutableFetcher("ppcancel://libs", map[string]string{
		"manifest.toml": testFiles["manifest.toml"],
		"lib/slow.py":   "x = 1\n",
	}, nil)
	manager := servePlugin(t, "ppcancel", fetcher)

	ctx, cancel := context.WithCancel(context.Background())
	bridge := New(Options{Manager: manager, Context: ctx, CacheDir: t.TempDir()})
	if err := bridge.Register(); err != nil {
		t.Fatalf("Register: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })

	bundle, err := pack.FetchBundle("ppcancel://libs", false, t.TempDir())
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}

	// Hold the next read open, then cancel the host's context.
	fetcher.block()
	errCh := make(chan error, 1)
	go func() {
		_, err := bundle.ReadFile("lib/slow.py")
		errCh <- err
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected the cancelled fetch to fail")
		}
		t.Logf("cancelled fetch returned: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("cancelling the host context did not abort the fetch")
	}
}

func TestBridgeRequiresManager(t *testing.T) {
	if err := New(Options{}).Register(); err == nil {
		t.Fatal("expected Register to require a manager")
	}
}
