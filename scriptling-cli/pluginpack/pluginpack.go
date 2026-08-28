// Package pluginpack bridges fetcher plugins into the pack source registry:
// every loaded plugin that advertises the fetch capability gets its schemes
// registered with pack, so --package knot://libs (and scheme script sources)
// resolve to bundles whose files are fetched on demand over the plugin
// protocol, with a per-file disk cache and staleness checks.
package pluginpack

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"sync"

	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

// DefaultMaxFetchFileSize caps a single file served by a fetcher plugin.
const DefaultMaxFetchFileSize = 32 * 1024 * 1024 // 32MB

var (
	clientsMu sync.RWMutex
	clients   = map[string]*plugin.Client{} // scheme → serving plugin
)

// Register wires every fetch-capable plugin in the manager into the pack
// scheme registry. It must be called after the manager has loaded its plugins
// and before --package sources are opened. Two plugins claiming the same
// scheme is an error.
func Register(manager *plugin.Manager) error {
	for _, meta := range manager.List() {
		client, ok := manager.Get(meta.Name)
		if !ok || !client.SupportsFetch() {
			continue
		}
		for _, scheme := range client.Schemes() {
			if err := registerScheme(client, scheme, meta.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func registerScheme(client *plugin.Client, scheme, pluginName string) error {
	if err := pack.RegisterScheme(scheme, func(source string, insecure bool, cacheDir string) (*pack.Bundle, error) {
		return openBundle(client, source, cacheDir)
	}); err != nil {
		return fmt.Errorf("plugin %s: %w", pluginName, err)
	}
	clientsMu.Lock()
	clients[scheme] = client
	clientsMu.Unlock()
	return nil
}

// DeclaredPackages returns the package sources the manager's fetcher plugins
// declared for automatic attachment, deduplicated and in stable plugin order.
func DeclaredPackages(manager *plugin.Manager) []string {
	var sources []string
	seen := map[string]bool{}
	for _, meta := range manager.List() {
		client, ok := manager.Get(meta.Name)
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

// DeclaredBundles opens the package sources the manager's fetcher plugins
// declared for automatic attachment, skipping any source in skip (typically
// the explicit --package sources, whose PreRun-opened bundles are used
// instead). The bundles come back ready to add to a pack loader ahead of any
// explicit bundles, so explicit sources shadow declared ones.
func DeclaredBundles(manager *plugin.Manager, insecure bool, cacheDir string, skip map[string]bool) ([]*pack.Bundle, error) {
	var bundles []*pack.Bundle
	for _, src := range DeclaredPackages(manager) {
		if skip[src] {
			continue
		}
		b, err := pack.FetchBundle(src, insecure, cacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load package %s: %w", src, err)
		}
		bundles = append(bundles, b)
	}
	return bundles, nil
}

// clientFor returns the plugin serving the source's registered scheme.
func clientFor(source string) (*plugin.Client, error) {
	scheme, ok := pack.SchemeFor(source)
	if !ok {
		return nil, fmt.Errorf("no fetcher plugin registered for source %s", source)
	}
	clientsMu.RLock()
	client := clients[scheme]
	clientsMu.RUnlock()
	if client == nil {
		return nil, fmt.Errorf("no fetcher plugin registered for source %s", source)
	}
	return client, nil
}

// openBundle opens a plugin-served source as a pack bundle. The manifest is
// read through the plugin file system like any other file, so opening costs
// one fetch plus the files later imports actually touch.
func openBundle(client *plugin.Client, source, cacheDir string) (*pack.Bundle, error) {
	return pack.OpenBundle(newPluginFS(client, source, cacheDir), source)
}

// FetchScript reads a scheme source that is itself a single script file
// (knot://scripts/hello). Scripts are executed right away, so they bypass the
// cache entirely — every invocation refetches, and a peer answering with a
// validator still returns full bytes (none are sent).
func FetchScript(source string) ([]byte, error) {
	client, err := clientFor(source)
	if err != nil {
		return nil, err
	}
	res, err := client.FetchFile(context.Background(), source, "", "", "")
	if err != nil {
		return nil, err
	}
	if res.NotModified {
		return nil, fmt.Errorf("plugin %s answered not_modified for an unconditional script fetch", client.Metadata().Name)
	}
	if len(res.Data) > DefaultMaxFetchFileSize {
		return nil, fmt.Errorf("script %s exceeds maximum size of %d bytes", source, DefaultMaxFetchFileSize)
	}
	return res.Data, nil
}

// fetchFile fetches one file through the client, mapping a miss to
// fs.ErrNotExist so loader probing treats it as a plain not-found.
func fetchFile(client *plugin.Client, source, path, etag, lastModified string) ([]byte, plugin.FetchResult, error) {
	res, err := client.FetchFile(context.Background(), source, path, etag, lastModified)
	if err != nil {
		if errors.Is(err, plugin.ErrFetchNotFound) {
			return nil, res, &fs.PathError{Op: "readfile", Path: path, Err: fs.ErrNotExist}
		}
		return nil, res, err
	}
	if len(res.Data) > DefaultMaxFetchFileSize {
		return nil, res, fmt.Errorf("file %s in %s exceeds maximum size of %d bytes", path, source, DefaultMaxFetchFileSize)
	}
	return res.Data, res, nil
}
