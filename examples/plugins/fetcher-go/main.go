// fetcher-go is a fetcher plugin: the one RegisterFetcher("demo", ...) call is
// the whole contract. It serves a virtual library (demo://libs, modules under
// lib/) and script sources (demo://scripts/hello) from memory, on demand. Run
// it with:
//
//	scriptling --plugin /tmp/scriptling-plugins/fetcher-go \
//	           -c 'import greet; print(greet.greeting("World"))'
//	scriptling --plugin /tmp/scriptling-plugins/fetcher-go demo://scripts/hello
//
// The host only asks for the files an import actually touches, and caches
// nothing it fetches, so the example returns content plainly.
package main

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/plugin"
)

// files is the virtual package served at demo://libs.
var files = map[string]string{
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

// memoryFetcher serves the maps above straight from memory. Read just returns
// the bytes; the host caches nothing, so there is no validator to deal with. A
// plugin whose backend is slow enough to want caching does it behind Read.
type memoryFetcher struct{}

func (memoryFetcher) Read(ctx context.Context, source, path string) ([]byte, error) {
	if content, ok := scripts[source]; ok && path == "" {
		return []byte(content), nil
	}
	if !strings.HasPrefix(source, "demo://libs") {
		return nil, fmt.Errorf("%w: %s", plugin.ErrFetchNotFound, source)
	}
	content, ok := files[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s in %s", plugin.ErrFetchNotFound, path, source)
	}
	return []byte(content), nil
}

func (memoryFetcher) Glob(ctx context.Context, source, pattern string) ([]plugin.FetchEntry, error) {
	if !strings.HasPrefix(source, "demo://libs") {
		return nil, fmt.Errorf("%w: %s", plugin.ErrFetchNotFound, source)
	}
	// The tree the pattern matches against: every file plus every directory
	// leading to one (so "<dir>" resolves as a directory entry and "<dir>/*"
	// lists it, even when it holds nothing).
	paths := map[string]bool{}
	for name := range files {
		paths[name] = false
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			paths[dir] = true
		}
	}
	entries := make([]plugin.FetchEntry, 0, len(paths))
	for name, isDir := range paths {
		if plugin.MatchGlob(pattern, name) {
			entries = append(entries, plugin.FetchEntry{Name: name, IsDir: isDir})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func main() {
	server := plugin.NewServer("demo-fetcher", "1.0.0", "Fetcher plugin serving demo:// sources from memory")
	server.RegisterFetcher("demo", memoryFetcher{})
	if err := server.Run(); err != nil {
		panic(err)
	}
}
