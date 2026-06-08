# Plugin Examples

These examples demonstrate Scriptling plugins loaded from executable files with
`--plugin-dir`.

## Go Plugin

`hello-go` demonstrates all registration styles in one plugin:

```bash
go build -o /tmp/scriptling-plugins/hello-go ./examples/plugins/hello-go
scriptling --plugin-dir /tmp/scriptling-plugins -c 'import plugin.hello; print(plugin.hello.greet("Ada"))'
```

It exposes:

- `plugin.hello.greet(name)` — function via `RegisterFunc`
- `plugin.hello.label(name)` — function via `RegisterFunc`
- `plugin.hello.Config(name).get(key)` — class via `RegisterClass`
- `plugin.hello.Counter(start).inc(amount)` — class via `RegisterClass`
- `plugin.hello.default_name` — constant

## Mixed Wrapper Plugin

`mixed-wrapper` shows generated proxies and custom Scriptling wrappers in the
same plugin:

```bash
go build -o /tmp/scriptling-plugins/wrap ./examples/plugins/mixed-wrapper
scriptling --plugin-dir /tmp/scriptling-plugins -c 'import plugin.wrap; print(plugin.wrap.greet("Ada"))'
```

It exposes:

- `plugin.wrap.generated(name)` — auto-generated function proxy
- `plugin.wrap.greet(name)` — custom function wrapper
- `plugin.wrap.Settings(name)` — auto-generated class proxy
- `plugin.wrap.Config(name)` — custom class wrapper with a defaulted `get`

## Bash Plugin

`bash/hello-plugin.sh` implements the JSON-RPC protocol directly. It requires
`jq` and is meant as a small protocol example rather than a production plugin.

```bash
mkdir -p /tmp/scriptling-plugins
cp examples/plugins/bash/hello-plugin.sh /tmp/scriptling-plugins/hello-plugin
chmod +x /tmp/scriptling-plugins/hello-plugin
scriptling --plugin-dir /tmp/scriptling-plugins -c 'import plugin.hello; print(plugin.hello.greet("Ada"))'
```
