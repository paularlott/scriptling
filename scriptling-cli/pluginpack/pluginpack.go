// Package pluginpack bridges fetcher plugins into the pack scheme registry:
// every loaded plugin that advertises the fetch capability gets its schemes
// registered, so knot://libs as a --package value (or as a script source)
// resolves to a bundle whose files are fetched on demand over the plugin
// protocol, with a per-file disk cache and conditional revalidation.
//
// Host applications that embed scriptling use the same path the CLI does:
//
//	bridge := pluginpack.New(pluginpack.Options{
//	    Context: ctx,      // cancels in-flight fetches
//	    Manager: manager,  // a plugin.Manager with its plugins already loaded
//	})
//	if err := bridge.Register(); err != nil { ... }
//	defer bridge.Close()   // releases the schemes for the next set of plugins
//
//	bundles, err := bridge.DeclaredBundles(nil)  // packages plugins declare
//	loader := pack.NewLoader()
//	for _, b := range bundles { loader.AddBundle(b) }
//	bootstrap.ApplyPackLoader(p, loader)
//
// A Bridge owns the schemes it registered and releases them on Close, so a
// host can reload its plugins without restarting the process.
package pluginpack

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

// DefaultDirTTL is how long a fetcher's directory listing is reused before it
// is fetched again. Long-lived hosts pick up files that appear in a served
// directory without a restart; short runs never notice the difference.
const DefaultDirTTL = 30 * time.Second

// Options configures a Bridge.
type Options struct {
	// Manager holds the loaded plugins to bridge. Required.
	Manager *plugin.Manager

	// Context bounds every fetch this bridge performs. Cancelling it aborts
	// in-flight reads and listings. Defaults to context.Background().
	Context context.Context

	// Registry is the scheme registry to register into. Defaults to the
	// process-wide pack.DefaultSchemeRegistry().
	Registry *pack.SchemeRegistry

	// CacheDir overrides the package cache directory used for fetched file
	// content. Empty means the OS default (pack.DefaultCacheDir).
	CacheDir string

	// Insecure is passed through when opening declared package sources.
	Insecure bool

	// DirTTL overrides how long directory listings are reused.
	// Zero means DefaultDirTTL; negative disables listing reuse entirely.
	DirTTL time.Duration
}

// Bridge connects a plugin.Manager's fetcher plugins to a pack.SchemeRegistry.
// It is safe for concurrent use.
type Bridge struct {
	manager  *plugin.Manager
	ctx      context.Context
	registry *pack.SchemeRegistry
	cacheDir string
	insecure bool
	dirTTL   time.Duration

	mu      sync.RWMutex
	clients map[string]*plugin.Client // scheme → serving plugin
}

// New creates a Bridge. It does not touch the scheme registry; call Register.
func New(opts Options) *Bridge {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	registry := opts.Registry
	if registry == nil {
		registry = pack.DefaultSchemeRegistry()
	}
	ttl := opts.DirTTL
	if ttl == 0 {
		ttl = DefaultDirTTL
	}
	return &Bridge{
		manager:  opts.Manager,
		ctx:      ctx,
		registry: registry,
		cacheDir: opts.CacheDir,
		insecure: opts.Insecure,
		dirTTL:   ttl,
		clients:  map[string]*plugin.Client{},
	}
}

// Register wires every fetch-capable plugin in the manager into the scheme
// registry. Call it after the manager has loaded its plugins and before any
// scheme source is opened. Two plugins claiming the same scheme is an error,
// and a partial registration is rolled back so the bridge is left clean.
func (b *Bridge) Register() error {
	if b.manager == nil {
		return errors.New("pluginpack: Options.Manager is required")
	}
	var claimed []string
	rollback := func() {
		for _, scheme := range claimed {
			b.registry.Unregister(scheme)
			b.mu.Lock()
			delete(b.clients, scheme)
			b.mu.Unlock()
		}
	}
	for _, meta := range b.manager.List() {
		client, ok := b.manager.Get(meta.Name)
		if !ok || !client.SupportsFetch() {
			continue
		}
		for _, scheme := range client.Schemes() {
			if err := b.registerScheme(client, scheme); err != nil {
				rollback()
				return fmt.Errorf("plugin %s: %w", meta.Name, err)
			}
			claimed = append(claimed, scheme)
		}
	}
	return nil
}

func (b *Bridge) registerScheme(client *plugin.Client, scheme string) error {
	opener := func(source string, insecure bool, cacheDir string) (*pack.Bundle, error) {
		if cacheDir == "" {
			cacheDir = b.cacheDir
		}
		return pack.OpenBundle(newPluginFS(b.ctx, client, source, cacheDir, b.dirTTL), source)
	}
	if err := b.registry.Register(scheme, opener); err != nil {
		return err
	}
	b.mu.Lock()
	b.clients[scheme] = client
	b.mu.Unlock()
	return nil
}

// Close releases every scheme this bridge registered. It does not close the
// plugin manager — the host owns that. Close is safe to call more than once,
// and bundles opened through the bridge stop working once it returns.
func (b *Bridge) Close() error {
	b.mu.Lock()
	schemes := make([]string, 0, len(b.clients))
	for scheme := range b.clients {
		schemes = append(schemes, scheme)
	}
	b.clients = map[string]*plugin.Client{}
	b.mu.Unlock()
	for _, scheme := range schemes {
		b.registry.Unregister(scheme)
	}
	return nil
}

// Schemes returns the schemes this bridge registered, in sorted order.
func (b *Bridge) Schemes() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	schemes := make([]string, 0, len(b.clients))
	for scheme := range b.clients {
		schemes = append(schemes, scheme)
	}
	sort.Strings(schemes)
	return schemes
}

// SchemeFor reports the scheme a source carries when this bridge serves it.
func (b *Bridge) SchemeFor(source string) (string, bool) {
	scheme, ok := pack.SchemeSyntax(source)
	if !ok {
		return "", false
	}
	b.mu.RLock()
	_, served := b.clients[scheme]
	b.mu.RUnlock()
	if !served {
		return "", false
	}
	return scheme, true
}

// DeclaredPackages returns the package sources this bridge's fetcher plugins
// declared for automatic attachment, deduplicated and in stable order.
func (b *Bridge) DeclaredPackages() []string {
	if b.manager == nil {
		return nil
	}
	var sources []string
	seen := map[string]bool{}
	for _, meta := range b.manager.List() {
		client, ok := b.manager.Get(meta.Name)
		if !ok || !client.SupportsFetch() {
			continue
		}
		for _, source := range client.Metadata().Packages {
			if !seen[source] {
				seen[source] = true
				sources = append(sources, source)
			}
		}
	}
	sort.Strings(sources) // manager listing order is not deterministic
	return sources
}

// DeclaredBundles opens the declared package sources, skipping any source in
// skip (typically the explicit --package sources, whose already-opened bundles
// are used instead). Add the result to a pack.Loader ahead of explicit
// bundles so explicit sources shadow declared ones.
func (b *Bridge) DeclaredBundles(skip map[string]bool) ([]*pack.Bundle, error) {
	var bundles []*pack.Bundle
	for _, src := range b.DeclaredPackages() {
		if skip[src] {
			continue
		}
		bundle, err := b.registry.FetchBundle(src, b.insecure, b.cacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load package %s: %w", src, err)
		}
		bundles = append(bundles, bundle)
	}
	return bundles, nil
}

// FetchScript reads a scheme source that is itself a single script file
// (knot://scripts/hello). Scripts execute immediately, so they bypass the
// cache: every call refetches. ctx bounds the fetch; pass nil to use the
// bridge's context.
func (b *Bridge) FetchScript(ctx context.Context, source string) ([]byte, error) {
	client, err := b.clientFor(source)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = b.ctx
	}
	res, err := client.FetchFile(ctx, source, "", "", "")
	if err != nil {
		return nil, err
	}
	if res.NotModified {
		return nil, fmt.Errorf("plugin %s answered not_modified for an unconditional script fetch of %s", client.Metadata().Name, source)
	}
	return res.Data, nil
}

// clientFor returns the plugin serving the source's scheme.
func (b *Bridge) clientFor(source string) (*plugin.Client, error) {
	scheme, ok := pack.SchemeSyntax(source)
	if !ok {
		return nil, fmt.Errorf("%s is not a <scheme>:// source", source)
	}
	b.mu.RLock()
	client := b.clients[scheme]
	b.mu.RUnlock()
	if client == nil {
		// Audience-neutral, and wrapping pack.ErrUnknownScheme so a caller that
		// knows how plugins are loaded in its context can extend the advice.
		// The CLI appends its flag names; an embedding host appends whatever
		// fits. Served schemes come last before the advice, for the same reason.
		if served := b.Schemes(); len(served) > 0 {
			return nil, fmt.Errorf("%w %q for %s (available schemes: %s): load the plugin that serves it",
				pack.ErrUnknownScheme, scheme, source, strings.Join(served, ", "))
		}
		return nil, fmt.Errorf("%w %q for %s: load the plugin that serves it", pack.ErrUnknownScheme, scheme, source)
	}
	return client, nil
}

// fetchFile fetches one file through the client, mapping a miss to
// fs.ErrNotExist so loader probing treats it as a plain not-found.
func fetchFile(ctx context.Context, client *plugin.Client, source, path, etag, lastModified string) ([]byte, plugin.FetchResult, error) {
	res, err := client.FetchFile(ctx, source, path, etag, lastModified)
	if err != nil {
		if errors.Is(err, plugin.ErrFetchNotFound) {
			return nil, res, &fs.PathError{Op: "readfile", Path: path, Err: fs.ErrNotExist}
		}
		return nil, res, err
	}
	return res.Data, res, nil
}
