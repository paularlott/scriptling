// fetcher-go is a fetcher plugin: it serves a small virtual package under the
// demo:// scheme, on demand, straight from memory. Run it with:
//
//	scriptling --plugin /tmp/scriptling-plugins/fetcher-go --package demo://libs \
//	           -c 'import greet; print(greet.greeting("World"))'
//	scriptling --plugin /tmp/scriptling-plugins/fetcher-go demo://scripts/hello
//
// The host only asks for the files an import actually touches — manifest.toml
// first, then each module as it resolves. Validators are the sha256 of the
// content, so a second run revalidates with the cached etag and the plugin
// answers not_modified instead of resending bytes.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/plugin"
)

// files is the virtual package served at demo://libs.
var files = map[string]string{
	"manifest.toml":           "name = \"demo-libs\"\nversion = \"1.0.0\"\ndescription = \"Virtual package served by the fetcher-go example plugin\"\nlibs = [\"lib\"]\n",
	"lib/greet.py":            "import demo\n\ndef greeting(name):\n    return demo.prefix() + \", \" + name\n",
	"lib/demo/__init__.py":    "def prefix():\n    return \"hello from demo://libs\"\n",
	"lib/calc.py":             "import greet\n\ndef add(params):\n    return params[\"a\"] + params[\"b\"]\n\ndef hello(params):\n    return greet.greeting(params.get(\"name\", \"World\"))\n",
	"docs/getting-started.md": "# demo://libs\n\nServed on demand by the fetcher-go example plugin.\n",
	"docs/configuration.md":   "# Configuration\n\nThere is nothing to configure; the files live in the plugin binary.\n",
}

// scripts are single-file sources: fetching demo://scripts/hello returns the
// script itself (no path).
var scripts = map[string]string{
	"demo://scripts/hello": "#!/usr/bin/env scriptling\nimport greet\nimport sys\nprint(greet.greeting(sys.argv[1] if len(sys.argv) > 1 else \"World\"))\n",
	"demo://scripts/setup": "import scriptling.runtime as runtime\n\nruntime.jsonrpc.method(\"demo.add\", \"calc.add\")\nruntime.jsonrpc.method(\"demo.hello\", \"calc.hello\")\n",
}

// memoryFetcher serves the maps above with content-hash validators.
type memoryFetcher struct{}

func (memoryFetcher) Read(ctx context.Context, source, path, etag, lastModified string) (plugin.FetchResult, error) {
	if content, ok := scripts[source]; ok && path == "" {
		return withValidator(content, etag)
	}
	if !strings.HasPrefix(source, "demo://libs") {
		return plugin.FetchResult{}, fmt.Errorf("%w: %s", plugin.ErrFetchNotFound, source)
	}
	if path == "" {
		path = "manifest.toml"
	}
	content, ok := files[path]
	if !ok {
		return plugin.FetchResult{}, fmt.Errorf("%w: %s in %s", plugin.ErrFetchNotFound, path, source)
	}
	return withValidator(content, etag)
}

func (memoryFetcher) List(ctx context.Context, source, path string) ([]plugin.FetchEntry, error) {
	if !strings.HasPrefix(source, "demo://libs") {
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
	for name := range files {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := name[len(prefix):]
		base, _, nested := strings.Cut(rest, "/")
		seen[base] = true
		isDir[base] = isDir[base] || nested
	}
	entries := make([]plugin.FetchEntry, 0, len(seen))
	for base := range seen {
		entries = append(entries, plugin.FetchEntry{Name: base, IsDir: isDir[base]})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	if len(entries) == 0 {
		return nil, fmt.Errorf("%w: %s in %s", plugin.ErrFetchNotFound, path, source)
	}
	return entries, nil
}

// withValidator returns the content with its sha256 as the etag, answering
// not_modified when the host's cached validator already matches.
func withValidator(content, etag string) (plugin.FetchResult, error) {
	sum := sha256.Sum256([]byte(content))
	validator := "sha256:" + hex.EncodeToString(sum[:])
	if etag != "" && etag == validator {
		return plugin.FetchResult{NotModified: true, ETag: validator}, nil
	}
	return plugin.FetchResult{Data: []byte(content), ETag: validator}, nil
}

func main() {
	server := plugin.NewServer("demo-fetcher", "1.0.0", "Fetcher plugin serving demo:// sources from memory")
	server.RegisterFetcher("demo", memoryFetcher{})
	// Attach demo://libs automatically: importing greet needs no --package.
	server.DeclarePackage("demo://libs")
	if err := server.Run(); err != nil {
		panic(err)
	}
}
