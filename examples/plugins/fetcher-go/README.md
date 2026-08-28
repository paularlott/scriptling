# Fetcher Plugin (Go)

`fetcher-go` is a fetcher plugin: it registers the `demo` scheme and serves a
small virtual package (`demo://libs`) plus a script source
(`demo://scripts/hello`) from memory, on demand. The host asks for
`manifest.toml` first and then for each module as an import actually resolves —
files nothing imports (the `docs/` here) are never transferred.

```bash
go build -o /tmp/scriptling-plugins/fetcher-go ./examples/plugins/fetcher-go

# the plugin declares demo://libs, so imports need no --package at all
scriptling --plugin /tmp/scriptling-plugins/fetcher-go -c 'import greet; print(greet.greeting("Ada"))'

# run a script served by the plugin (scripts are refetched on every run)
scriptling --plugin /tmp/scriptling-plugins/fetcher-go demo://scripts/hello Ada
```

It serves:

- `demo://libs` — package: `manifest.toml`, `lib/greet.py`, `lib/calc.py`, `lib/demo/__init__.py`, `docs/`
- `demo://scripts/hello` — a single-file script source
- `demo://scripts/setup` — a JSON-RPC setup script for server modes

The plugin returns the sha256 of each file as its etag. The host does not cache
what a plugin serves, so it never sends those validators back and every read
reaches `Read` — the etag is returned anyway because it costs nothing and a host
that does cache will use it. Nothing is written to the package cache:

```bash
scriptling --cache-dir /tmp/demo-cache --plugin /tmp/scriptling-plugins/fetcher-go \
           -c 'import greet'
ls /tmp/demo-cache   # empty: plugin content is never persisted
```

Setup scripts work in the server modes, with handler modules arriving from
the declared package:

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"demo.add","params":{"a":2,"b":3}}\n' |
  scriptling --plugin /tmp/scriptling-plugins/fetcher-go --json-rpc demo://scripts/setup
```

See the website's plugin fetchers documentation for the `Fetcher` interface
(`Read` and `List`) and the wire protocol (`fetch.read` / `fetch.list`).
