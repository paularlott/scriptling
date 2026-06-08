# Plugin Examples

These examples demonstrate Scriptling plugins loaded from executable files with
`--plugin-dir`.

## Go Plugin

`hello-go` uses typed Go functions and native resource classes:

```bash
go build -o /tmp/scriptling-plugins/hello-go ./examples/plugins/hello-go
scriptling --plugin-dir /tmp/scriptling-plugins -c 'import plugin.hello; print(plugin.hello.greet("Ada"))'
```

It exposes:

- `plugin.hello.greet(name)`
- `plugin.hello.Config(values).get(key)`

## Builder Plugin

`builder` shows the environment-style builder path:

```go
fb := object.NewFunctionBuilder()
fb.Function(func(name string) string { return "built:" + name })
server.RegisterFunc("label", fb.Build())

cb := object.NewClassBuilder("Counter")
server.RegisterClass(cb.Build())
```

## Embedded Scriptling Plugin

`embedded-scriptling` registers Scriptling-authored behavior that is embedded in
the plugin executable at startup.

## Mixed Wrapper Plugin

`mixed-wrapper` exposes one generated function and one plugin-supplied wrapper
that calls a hidden RPC helper with `scriptling.plugin.call_function`.

## Bash Plugin

`bash/hello-plugin.sh` implements the JSON-RPC protocol directly. It requires
`jq` and is meant as a small protocol example rather than a production plugin.

```bash
mkdir -p /tmp/scriptling-plugins
cp examples/plugins/bash/hello-plugin.sh /tmp/scriptling-plugins/hello-plugin
chmod +x /tmp/scriptling-plugins/hello-plugin
scriptling --plugin-dir /tmp/scriptling-plugins -c 'import plugin.hello; print(plugin.hello.greet("Ada"))'
```
