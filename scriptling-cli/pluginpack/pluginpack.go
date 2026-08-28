// Package pluginpack bridges fetcher plugins into the pack scheme registry:
// every loaded plugin that advertises the fetch capability gets its schemes
// registered, so knot://libs as a --package value (or as a script source)
// resolves to a bundle whose files are fetched on demand over the plugin
// protocol.
//
// The host never persists what a plugin serves. Content is held in memory for
// the lifetime of the bundle's file system and revalidated with the plugin's
// own validators; nothing reaches the package cache on disk. A plugin that
// wants caching across runs does it behind its fetcher, where the backend's
// credentials and freshness rules already live.
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
		if err := b.registerScheme(client); err != nil {
			rollback()
			return fmt.Errorf("plugin %s: %w", meta.Name, err)
		}
		claimed = append(claimed, client.Scheme())
	}
	return nil
}

// registerScheme routes the plugin's one scheme to it. Every source under the
// scheme opens as a virtual bundle with the standard layout (modules under
// lib/) — the plugin serves paths, never a manifest. The opener signature
// matches FetchBundle for composition; scheme openers ignore the insecure and
// cache-dir parameters, which only apply to the built-in zip/URL path.
func (b *Bridge) registerScheme(client *plugin.Client) error {
	scheme := client.Scheme()
	opener := func(source string, insecure bool, cacheDir string) (*pack.Bundle, error) {
		fsys := newPluginFS(b.ctx, client, source, b.dirTTL)
		return pack.VirtualBundle(declaredName(client), client.Metadata().Version, fsys, source), nil
	}
	if err := b.registry.Register(scheme, opener); err != nil {
		return err
	}
	b.mu.Lock()
	b.clients[scheme] = client
	b.mu.Unlock()
	return nil
}

// declaredName strips the plugin. namespace prefix for use as a bundle name.
func declaredName(client *plugin.Client) string {
	return strings.TrimPrefix(client.Metadata().Name, "plugin.")
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

// Bundles opens the library bundle of every fetch-capable plugin — the
// <scheme>://libs source each plugin attaches automatically, with the
// standard layout. Add the result to a pack.Loader ahead of explicit
// --package bundles so explicit sources shadow plugin libraries.
func (b *Bridge) Bundles() ([]*pack.Bundle, error) {
	if b.manager == nil {
		return nil, nil
	}
	var bundles []*pack.Bundle
	for _, scheme := range b.Schemes() {
		source := scheme + "://libs"
		bundle, err := b.registry.FetchBundle(source, false, "")
		if err != nil {
			return nil, fmt.Errorf("failed to load package %s: %w", source, err)
		}
		bundles = append(bundles, bundle)
	}
	return bundles, nil
}

// FetchScript reads a scheme source that is itself a single script file
// (knot://myscript). Scripts execute immediately, so every call refetches.
// ctx bounds the fetch; pass nil to use the bridge's context.
func (b *Bridge) FetchScript(ctx context.Context, source string) ([]byte, error) {
	client, err := b.clientFor(source)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = b.ctx
	}
	return client.FetchFile(ctx, source, "")
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
func fetchFile(ctx context.Context, client *plugin.Client, source, path string) ([]byte, error) {
	data, err := client.FetchFile(ctx, source, path)
	if err != nil {
		if errors.Is(err, plugin.ErrFetchNotFound) {
			return nil, &fs.PathError{Op: "readfile", Path: path, Err: fs.ErrNotExist}
		}
		return nil, err
	}
	return data, nil
}
