# Fetcher Plugin (Go)

`fetcher-go` is the full-example plugin. One binary serves everything a
plugin can:

- a **fetcher** — the one `RegisterFetcher("demo", ...)` call — owning the
  `demo://` scheme: a virtual library at `demo://libs` (modules under `lib/`,
  any depth of nesting) plus single-file script sources (`demo://scripts/...`)
- a **function** and a **class** under `plugin.demo`, like any Go plugin

The host synthesizes the package layout itself and asks for each file only
when an import or read actually touches it, so files nothing reads (most of
`docs/` and `data/`) are never transferred.

```bash
go build -o /tmp/scriptling-plugins/fetcher-go ./examples/plugins/fetcher-go

# the plugin's library attaches automatically, so no --package is needed
scriptling --plugin /tmp/scriptling-plugins/fetcher-go -c 'import greet; print(greet.greeting("Ada"))'

# run a script served by the plugin (scripts are refetched on every run)
scriptling --plugin /tmp/scriptling-plugins/fetcher-go demo://scripts/hello Ada

# the tour: namespaced modules, static assets, the function and the class
scriptling --plugin /tmp/scriptling-plugins/fetcher-go demo://scripts/tour
```

It serves:

- `demo://libs` — the library: `lib/greet.py`, `lib/calc.py`, packages
  `lib/hub/`, `lib/fred/` (one level, `import fred`) and `lib/blah/blah/`
  (two levels, `import blah.blah`), plus static assets `docs/` and `data/`
- `demo://scripts/hello`, `demo://scripts/tour` — single-file script sources
- `demo://scripts/setup` — a JSON-RPC setup script for server modes

Static assets are read from scripts through `scriptling.package`, using the
plugin's name (`demo`) as the package name:

```python
import scriptling.package as package

text = package.read_file("demo", "docs/getting-started.md")
docs = package.glob("demo", "**/*.md")
```

The `Read` handler just returns the bytes. The host caches nothing it fetches,
so every read reaches the plugin and nothing is written to the package cache:

```bash
scriptling --cache-dir /tmp/demo-cache --plugin /tmp/scriptling-plugins/fetcher-go \
           -c 'import greet'
ls /tmp/demo-cache   # empty: plugin content is never persisted
```

A fetcher whose backend is slow enough to want caching does it inside `Read`;
there is no host-side cache and no conditional-read protocol to hook into.

Setup scripts work in the server modes, with handler modules arriving from
the plugin's library:

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"demo.add","params":{"a":2,"b":3}}\n' |
  scriptling --plugin /tmp/scriptling-plugins/fetcher-go --json-rpc demo://scripts/setup
```

See the website's plugin fetchers documentation for the `Fetcher` interface
(`Read` and `Glob`) and the wire protocol (`fetch.read` / `fetch.glob`), and
the fetcher plugin tutorial for a walkthrough of this example.
