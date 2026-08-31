// fetcher-go is the full-example plugin: it registers a function and a class
// like any Go plugin (importable as plugin.demo), and additionally serves a
// fetcher — the one RegisterFetcher("demo", ...) call — so the same plugin
// owns the demo:// scheme. It serves a virtual library (demo://libs, modules
// under lib/, any depth of nesting), static assets (markdown, json) and
// single-file script sources (demo://scripts/hello), all from memory, on
// demand. Run it with:
//
//	scriptling --plugin /tmp/scriptling-plugins/fetcher-go \
//	           -c 'import greet; print(greet.greeting("World"))'
//	scriptling --plugin /tmp/scriptling-plugins/fetcher-go demo://scripts/tour
//
// The host only asks for the files an import or read actually touches, and
// caches nothing it fetches, so the example returns content plainly.
package main

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
)

// files is the virtual package served at demo://libs: code under lib/
// (nested packages become dotted imports — blah/blah/__init__.py is
// import blah.blah) and static assets anywhere else.
var files = map[string]string{
	"lib/greet.py":              "import hub\n\ndef greeting(name):\n    return hub.prefix() + \", \" + name\n",
	"lib/hub/__init__.py":       "def prefix():\n    return \"hello from demo://libs\"\n",
	"lib/fred/__init__.py":      "def value():\n    return \"fred, a one-level package\"\n",
	"lib/blah/__init__.py":      "label = \"blah\"\n",
	"lib/blah/blah/__init__.py": "def value():\n    return \"blah.blah, a two-level package\"\n",
	"lib/calc.py":               "import greet\n\ndef add(params):\n    return params[\"a\"] + params[\"b\"]\n\ndef hello(params):\n    return greet.greeting(params.get(\"name\", \"World\"))\n",
	"docs/getting-started.md":   "# demo://libs\n\nServed on demand by the fetcher-go example plugin.\n",
	"docs/configuration.md":     "# Configuration\n\nThere is nothing to configure; the files live in the plugin binary.\n",
	"data/config.json":          "{\n  \"greeting\": \"hello from data/config.json\",\n  \"version\": 1\n}\n",
}

// scripts are single-file sources: fetching demo://scripts/hello returns the
// script itself (no path).
var scripts = map[string]string{
	"demo://scripts/hello": "#!/usr/bin/env scriptling\nimport greet\nimport sys\nprint(greet.greeting(sys.argv[1] if len(sys.argv) > 1 else \"World\"))\n",
	"demo://scripts/setup": "import scriptling.runtime as runtime\n\nruntime.jsonrpc.method(\"demo.add\", \"calc.add\")\nruntime.jsonrpc.method(\"demo.hello\", \"calc.hello\")\n",
	// The tour exercises everything the plugin serves: namespaced modules,
	// static assets via scriptling.package, the registered function and class.
	"demo://scripts/tour": `import greet
import fred
import blah.blah
import json
import plugin.demo
import scriptling.package as package

print(greet.greeting("tour"))
print(fred.value())
print(blah.blah.value())

config = json.loads(package.read_file("demo", "data/config.json"))
print(config["greeting"])

print(plugin.demo.asset("docs/getting-started.md").splitlines()[0])
doc = plugin.demo.Doc("docs/configuration.md")
print(doc.title())
print(package.glob("demo", "**/*.md"))
`,
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
	server := plugin.NewServer("demo", "1.0.0", "Fetcher plugin serving demo:// sources from memory")

	// The function surface: scripts call plugin.demo.asset(...). It reads the
	// same map the fetcher serves, so one source of truth backs both halves.
	assetBuilder := object.NewFunctionBuilder()
	assetBuilder.Function(func(name string) (string, error) {
		content, ok := files[name]
		if !ok {
			return "", fmt.Errorf("no such file: %s", name)
		}
		return content, nil
	})
	server.RegisterFunc("asset", assetBuilder)

	// The class surface: plugin.demo.Doc(path) wraps a served document. The
	// constructor resolves the content once and stores it on the instance.
	docBuilder := object.NewClassBuilder("Doc").
		Method("__init__", func(self *object.Instance, p string) error {
			content, ok := files[p]
			if !ok {
				return fmt.Errorf("no such document: %s", p)
			}
			self.SetField("path", object.NewString(p))
			self.SetField("content", object.NewString(content))
			return nil
		}).
		Method("title", func(self *object.Instance) string {
			for _, line := range strings.Split(self.Field("content").(*object.String).StringValue(), "\n") {
				if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "# ") {
					return strings.TrimPrefix(trimmed, "# ")
				}
			}
			return ""
		}).
		Method("content", func(self *object.Instance) string {
			return self.Field("content").(*object.String).StringValue()
		}).
		Method("lines", func(self *object.Instance) int {
			return len(strings.Split(self.Field("content").(*object.String).StringValue(), "\n"))
		})
	server.RegisterClass(docBuilder)

	server.RegisterFetcher("demo", memoryFetcher{})
	if err := server.Run(); err != nil {
		panic(err)
	}
}
