package pluginpack

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
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

func (f *mutableFetcher) Read(ctx context.Context, source, path string) ([]byte, error) {
	f.mu.Lock()
	block := f.blockCh
	f.mu.Unlock()
	if block != nil {
		// Hold the call open so a caller can cancel its context mid-fetch.
		select {
		case <-block:
		case <-ctx.Done():
			return nil, ctx.Err()
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
		return nil, fmt.Errorf("%w: %s in %s", plugin.ErrFetchNotFound, path, source)
	}
	f.reads[key]++
	return []byte(content), nil
}

func (f *mutableFetcher) Glob(ctx context.Context, source, pattern string) ([]plugin.FetchEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if source != f.source {
		return nil, fmt.Errorf("%w: %s", plugin.ErrFetchNotFound, source)
	}
	f.lists[pattern]++
	// The tree: every file plus every directory leading to one, so exact-path
	// probes resolve directories (empty ones included) and "<dir>/*" lists.
	paths := map[string]bool{}
	for name := range f.files {
		paths[name] = false
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			paths[dir] = true
		}
	}
	entries := []plugin.FetchEntry{}
	for name, isDir := range paths {
		if plugin.MatchGlob(pattern, name) {
			entries = append(entries, plugin.FetchEntry{Name: name, IsDir: isDir})
		}
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
	httpSrv := httptest.NewServer(server)
	t.Cleanup(httpSrv.Close)

	manager := plugin.NewManager(nil)
	t.Cleanup(func() { _ = manager.Close() })
	if _, err := manager.LoadURL(context.Background(), scheme, httpSrv.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	return manager
}

// bridgeFor builds a registered Bridge over a manager and releases its schemes
// when the test ends, so schemes never leak between tests.
func bridgeFor(t *testing.T, manager *plugin.Manager) *Bridge {
	t.Helper()
	bridge := New(Options{Manager: manager})
	if err := bridge.Register(); err != nil {
		t.Fatalf("Bridge.Register: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })
	return bridge
}

var testFiles = map[string]string{
	"lib/hello.py":           "def value():\n    return 'hello-v1'\n",
	"lib/nested/__init__.py": "nested_value = 'nested'\n",
	"docs/never-imported.md": "# never imported\n",
}

func TestFetchBundleServesPackageOnDemand(t *testing.T) {
	fetcher := newMutableFetcher("ppond://libs", testFiles, nil)
	manager := servePlugin(t, "ppond", fetcher)
	cacheDir := t.TempDir()
	bridge := bridgeFor(t, manager)

	bundle, err := pack.FetchBundle("ppond://libs", false, cacheDir)
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}
	if bundle.Manifest.Name != "ppond" {
		t.Fatalf("expected manifest name from the plugin name (ppond), got %s", bundle.Manifest.Name)
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

	// On demand: only the imported modules were fetched — no manifest read,
	// the layout is synthesized host-side.
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
	if err != nil || len(entries) != 2 {
		t.Errorf("readdir . = %d entries (err %v), want 2 (lib, docs)", len(entries), err)
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
	bridgeFor(t, manager)

	bundle, err := pack.FetchBundle("ppnodisk://libs", false, cacheDir)
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}
	for _, name := range []string{"lib/hello.py", "lib/nested/__init__.py"} {
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
		"lib/data.py": "def value():\n    return 'data-v1'\n",
	}, nil)
	manager := servePlugin(t, "ppmem", fetcher)
	bridgeFor(t, manager)

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
	fetcher := newMutableFetcher("ppscript://x", map[string]string{}, map[string]string{
		"ppscript://run/hello": "print('script-v1')\n",
	})
	manager := servePlugin(t, "ppscript", fetcher)
	bridge := bridgeFor(t, manager)

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
		"lib/here.py": "x = 1\n",
	}, nil)
	manager := servePlugin(t, "ppmiss", fetcher)
	bridgeFor(t, manager)

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
	fetcher := newMutableFetcher("ppconf://libs", map[string]string{}, nil)
	manager := servePlugin(t, "ppconf", fetcher)
	bridgeFor(t, manager)

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
		"lib/v.py": "v = 'first'\n",
	}, nil)
	manager := servePlugin(t, "ppreload", fetcher)

	first := New(Options{Manager: manager})
	if err := first.Register(); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if pack.DefaultSchemeRegistry().Lookup("ppreload") == nil {
		t.Fatal("expected ppreload to be registered")
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if pack.DefaultSchemeRegistry().Lookup("ppreload") != nil {
		t.Fatal("expected Close to release the scheme")
	}
	// Close is idempotent.
	if err := first.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	// A fresh bridge over the same plugins claims the scheme again.
	second := New(Options{Manager: manager})
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
	bridge := New(Options{Manager: manager, Registry: registry})
	if err := bridge.Register(); err != nil {
		t.Fatalf("Register: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })

	// The private registry resolves the source; the default one does not.
	if _, err := registry.FetchBundle("ppiso://libs", false, t.TempDir()); err != nil {
		t.Fatalf("private registry FetchBundle: %v", err)
	}
	if pack.DefaultSchemeRegistry().Lookup("ppiso") != nil {
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

func TestBundlesPerFetcherPlugin(t *testing.T) {
	fetcher := newMutableFetcher("ppdecl://libs", testFiles, nil)
	manager := servePlugin(t, "ppdecl", fetcher)

	// A second plugin brings its own fetcher and its own scheme — one per
	// plugin, no declarations, no overlaps. Both load before the bridge
	// registers: Register wires whatever the manager holds at that moment.
	declServer := plugin.NewServer("declarer", "2.0.0", "second fetcher plugin")
	declServer.RegisterFetcher("ppdecl2", fetcher)
	httpSrv := httptest.NewServer(declServer)
	t.Cleanup(httpSrv.Close)
	if _, err := manager.LoadURL(context.Background(), "declarer", httpSrv.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}

	bridge := bridgeFor(t, manager)
	got := bridge.Schemes()
	if len(got) != 2 || got[0] != "ppdecl" || got[1] != "ppdecl2" {
		t.Fatalf("expected sorted [ppdecl ppdecl2], got %v", got)
	}

	bundles, err := bridge.Bundles()
	if err != nil {
		t.Fatalf("Bundles: %v", err)
	}
	if len(bundles) != 2 {
		t.Fatalf("expected one bundle per fetcher plugin, got %d", len(bundles))
	}
	byName := map[string]*pack.Bundle{}
	for _, b := range bundles {
		byName[b.Manifest.Name] = b
	}
	// Synthesized manifests take name and version from each plugin's
	// handshake; the layout is the hardcoded lib/ dir.
	if b := byName["ppdecl"]; b == nil || b.Manifest.Version != "1.0.0" || b.Manifest.Libs[0] != "lib" {
		t.Fatalf("expected ppdecl v1.0.0 with libs=[lib], got %+v", b)
	}
	if b := byName["declarer"]; b == nil || b.Manifest.Version != "2.0.0" {
		t.Fatalf("expected declarer v2.0.0 from the handshake, got %+v", b)
	}
}

func TestAutoAttachedPackageImportsWithoutExplicitPackage(t *testing.T) {
	fetcher := newMutableFetcher("ppauto://libs", testFiles, nil)
	manager := servePlugin(t, "ppauto", fetcher)
	bridge := bridgeFor(t, manager)

	// The host flow: open every plugin's library bundle and add it to the
	// loader before any explicit --package bundles.
	loader := pack.NewLoader()
	bundles, err := bridge.Bundles()
	if err != nil {
		t.Fatalf("Bundles: %v", err)
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

// TestExplicitPackageShadowsAutoAttached: an explicit --package (a local dir or
// zip) takes precedence over a plugin's auto-attached library, because the CLI
// adds it to the loader after the plugin bundles and the loader searches
// last-added first. --package never carries a scheme source — plugin libraries
// attach on their own — so the explicit side here is an ordinary local package,
// which is the only kind --package accepts.
func TestExplicitPackageShadowsAutoAttached(t *testing.T) {
	autoFetcher := newMutableFetcher("ppshadow-auto://libs", map[string]string{
		"lib/which.py": "def value():\n    return 'from auto'\n",
	}, nil)
	autoManager := servePlugin(t, "ppshadow-auto", autoFetcher)
	autoBridge := bridgeFor(t, autoManager)

	// The explicit package is a local directory, exactly what --package takes.
	explicitDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(explicitDir, "manifest.toml"),
		[]byte("name = \"shadow-explicit\"\nversion = \"1.0.0\"\nlibs = [\"lib\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(explicitDir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(explicitDir, "lib", "which.py"),
		[]byte("def value():\n    return 'from explicit'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mirror the CLI ordering: auto bundles first, explicit --package bundle
	// after (the loader searches last-added first).
	loader := pack.NewLoader()
	autoBundles, err := autoBridge.Bundles()
	if err != nil {
		t.Fatalf("Bundles: %v", err)
	}
	for _, b := range autoBundles {
		if err := loader.AddBundle(b); err != nil {
			t.Fatalf("AddBundle: %v", err)
		}
	}
	b, err := pack.FetchBundle(explicitDir, false, t.TempDir())
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

func TestBundles(t *testing.T) {
	fetcher := newMutableFetcher("ppbundles://libs", testFiles, nil)
	manager := servePlugin(t, "ppbundles", fetcher)
	bridge := bridgeFor(t, manager)

	bundles, err := bridge.Bundles()
	if err != nil {
		t.Fatalf("Bundles: %v", err)
	}
	if len(bundles) != 1 || bundles[0].Manifest.Name != "ppbundles" {
		t.Fatalf("expected the bundle named after the plugin, got %+v", bundles)
	}
	if got := fetcher.readsOf("manifest.toml"); got != 0 {
		t.Fatalf("no manifest is ever fetched, got %d reads", got)
	}
}

// failingFetcher fails every read with a plain error (not ErrFetchNotFound),
// simulating an unreachable backend.
type failingFetcher struct {
	inner *mutableFetcher
}

func (f failingFetcher) Read(ctx context.Context, source, path string) ([]byte, error) {
	return nil, errors.New("knot server unreachable: connection refused")
}

func (f failingFetcher) Glob(ctx context.Context, source, pattern string) ([]plugin.FetchEntry, error) {
	return f.inner.Glob(ctx, source, pattern)
}

// TestPluginFailureAbortsImportLoudly proves the not-found vs cannot-reach
// split across the whole protocol: a plugin whose fetcher errors (rather
// than answering not-found) makes the import fail with the source named,
// while a plain miss stays a quiet unknown-library.
func TestPluginFailureAbortsImportLoudly(t *testing.T) {
	base := newMutableFetcher("ppfail://libs", map[string]string{}, nil)
	manager := servePlugin(t, "ppfail", failingFetcher{inner: base})
	bridge := bridgeFor(t, manager)

	bundles, err := bridge.Bundles()
	if err != nil {
		t.Fatalf("Bundles: %v", err)
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
		"tools/first.py": "x = 1\n",
	}, nil)
	manager := servePlugin(t, "ppdirttl", fetcher)

	// A negative TTL disables reuse entirely, so every listing is fresh.
	bridge := New(Options{Manager: manager, DirTTL: -1})
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
	if got := fetcher.listsOf("tools/*"); got != 2 {
		t.Fatalf("expected 2 fetch.glob listings with reuse disabled, got %d", got)
	}
}

// TestDirListingsReusedWithinTTL is the other half: within the TTL a listing is
// served from memory, so probe-heavy work does not re-list on every call.
func TestDirListingsReusedWithinTTL(t *testing.T) {
	fetcher := newMutableFetcher("ppdirreuse://libs", map[string]string{
		"tools/first.py": "x = 1\n",
	}, nil)
	manager := servePlugin(t, "ppdirreuse", fetcher)
	bridge := New(Options{Manager: manager, DirTTL: time.Hour})
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
	if got := fetcher.listsOf("tools/*"); got != 1 {
		t.Fatalf("expected 1 fetch.glob listing within the TTL, got %d", got)
	}
}

// TestContextCancellationAbortsFetch is the ctx-propagation guarantee: a host
// that cancels its context stops in-flight fetches instead of waiting out the
// protocol timeout.
func TestContextCancellationAbortsFetch(t *testing.T) {
	fetcher := newMutableFetcher("ppcancel://libs", map[string]string{
		"lib/slow.py": "x = 1\n",
	}, nil)
	manager := servePlugin(t, "ppcancel", fetcher)

	ctx, cancel := context.WithCancel(context.Background())
	bridge := New(Options{Manager: manager, Context: ctx})
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

// TestFetcherServesNamespacesAndAssets proves the whole serving story from one
// fetcher: packages nest to any depth (lib/blah/blah/__init__.py imports as
// blah.blah), a module beside a package's __init__ is reachable
// (blah.extra), and static files at the root or in subdirectories are read
// from scripts through scriptling.package under the plugin's name.
func TestFetcherServesNamespacesAndAssets(t *testing.T) {
	fetcher := newMutableFetcher("ppns://libs", map[string]string{
		"lib/fred/__init__.py":      "def value():\n    return 'fred'\n",
		"lib/blah/__init__.py":      "label = 'blah'\n",
		"lib/blah/blah/__init__.py": "def value():\n    return 'blah.blah'\n",
		"lib/blah/extra.py":         "def value():\n    return 'blah.extra'\n",
		"something.md":              "# root asset\n",
		"other/something.md":        "# nested asset\n",
	}, nil)
	manager := servePlugin(t, "ppns", fetcher)
	bridge := bridgeFor(t, manager)

	loader := pack.NewLoader()
	bundles, err := bridge.Bundles()
	if err != nil {
		t.Fatalf("Bundles: %v", err)
	}
	for _, b := range bundles {
		if err := loader.AddBundle(b); err != nil {
			t.Fatalf("AddBundle: %v", err)
		}
	}

	p := scriptling.New()
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)
	pack.RegisterPackageLibrary(p, loader)

	script := `
import blah
import blah.blah
import blah.extra
import fred
import scriptling.package as package

[
    fred.value(),
    blah.label,
    blah.blah.value(),
    blah.extra.value(),
    package.read_file("ppns", "something.md"),
    package.read_file("ppns", "other/something.md"),
    package.file_exists("ppns", "other/something.md"),
    package.glob("ppns", "**/*.md"),
    package.names(),
]
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected a list result, got %T", result)
	}
	want := []string{
		"fred",
		"blah",
		"blah.blah",
		"blah.extra",
		"# root asset\n",
		"# nested asset\n",
		"True",
		"[other/something.md, something.md]",
		"[ppns]",
	}
	if len(list.Elements) != len(want) {
		t.Fatalf("result has %d elements (%v), want %d", len(list.Elements), result.Inspect(), len(want))
	}
	for i, expected := range want {
		if got := list.Elements[i].Inspect(); got != expected {
			t.Errorf("element %d = %s, want %s", i, got, expected)
		}
	}
}

// dirAwareFetcher serves an explicit tree with directories of its own,
// including an empty one, so directory resolution can be probed without
// files forcing the directories into existence.
type dirAwareFetcher struct {
	source string
	dirs   map[string]bool
	files  map[string]string
	globs  map[string]int
}

func (f *dirAwareFetcher) Read(ctx context.Context, source, path string) ([]byte, error) {
	if content, ok := f.files[path]; ok {
		return []byte(content), nil
	}
	return nil, fmt.Errorf("%w: %s", plugin.ErrFetchNotFound, path)
}

func (f *dirAwareFetcher) Glob(ctx context.Context, source, pattern string) ([]plugin.FetchEntry, error) {
	f.globs[pattern]++
	tree := map[string]bool{}
	for name := range f.files {
		tree[name] = false
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			tree[dir] = true
		}
	}
	for dir := range f.dirs {
		tree[dir] = true
	}
	entries := []plugin.FetchEntry{}
	for name, isDir := range tree {
		if plugin.MatchGlob(pattern, name) {
			entries = append(entries, plugin.FetchEntry{Name: name, IsDir: isDir})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

// TestPluginFSGlobOneShot proves pluginFS implements GlobFS: a whole-subtree
// glob is one fetch.glob call, not a listing per level.
func TestPluginFSGlobOneShot(t *testing.T) {
	fetcher := &dirAwareFetcher{
		source: "ppglob://libs",
		dirs:   map[string]bool{"empty": true},
		files: map[string]string{
			"lib/a/one.py":   "x = 1\n",
			"lib/a/two.py":   "y = 2\n",
			"lib/b/three.py": "z = 3\n",
		},
		globs: map[string]int{},
	}
	manager := servePlugin(t, "ppglob", fetcher)
	bridge := bridgeFor(t, manager)

	bundles, err := bridge.Bundles()
	if err != nil {
		t.Fatalf("Bundles: %v", err)
	}
	bundle := bundles[0]

	matches, err := fs.Glob(bundle.FS(), "lib/**/*.py")
	if err != nil {
		t.Fatalf("fs.Glob: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %v", matches)
	}
	if got := fetcher.globs["lib/**/*.py"]; got != 1 {
		t.Fatalf("a subtree glob must be one fetch.glob call, got %d", got)
	}

	// Cosmetic spellings are accepted; traversal still is not.
	for _, spelling := range []string{"./lib/**/*.py", "lib/./**/*.py"} {
		matches, err := fs.Glob(bundle.FS(), spelling)
		if err != nil {
			t.Fatalf("fs.Glob(%q): %v", spelling, err)
		}
		if len(matches) != 3 {
			t.Fatalf("fs.Glob(%q) = %v, want 3 matches", spelling, matches)
		}
	}
	if _, err := fs.Glob(bundle.FS(), "../x/*.py"); err == nil {
		t.Fatal("expected ../ traversal to be rejected")
	}

	// An empty directory resolves as a directory and lists as empty, where a
	// listing-based scheme cannot tell empty from missing.
	info, err := fs.Stat(bundle.FS(), "empty")
	if err != nil || !info.IsDir() {
		t.Fatalf("Stat(empty) = %v, %v; want a directory", info, err)
	}
	entries, err := fs.ReadDir(bundle.FS(), "empty")
	if err != nil || len(entries) != 0 {
		t.Fatalf("ReadDir(empty) = %v, %v; want an empty listing", entries, err)
	}
}
