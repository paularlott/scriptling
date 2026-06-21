# Plugin Examples

These examples demonstrate Scriptling plugins loaded from executable files with
`--plugin-dir`.

## C Plugin

`hello-c` is a feature-complete C plugin built with the Scriptling C Plugin SDK
(`scriptling_plugin.h` / `scriptling_plugin.c`). It handles requests concurrently
with one thread per request and demonstrates functions, classes with constructors
and destructors, read/write properties, constants, and callbacks.
The SDK accepts JSON-RPC single requests, batched requests, and notifications
without ids; mixed batches return only the entries that require responses.

```bash
make -C examples/plugins/hello-c
mkdir -p /tmp/scriptling-plugins
cp examples/plugins/hello-c/hello-c /tmp/scriptling-plugins/
scriptling --plugin-dir /tmp/scriptling-plugins -c 'import plugin.hello; print(plugin.hello.greet("Ada"))'
```

It exposes:

- `plugin.hello.greet(name)` — function
- `plugin.hello.label(name)` — function
- `plugin.hello.stream(callback)` — function that invokes a callback
- `plugin.hello.Config(name).get()` — class with method
- `plugin.hello.Counter(start).inc(amount)`, `.get()` — class with mutable state
- `plugin.hello.Counter.value` — read/write property
- `plugin.hello.Counter.label` — read-only property
- `plugin.hello.default_name` — constant

See `hello-c/README.md` for the full SDK documentation.

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

## HTTP Go Plugin

`http-go` exposes the same Scriptling plugin protocol over HTTP. It is loaded
with `scriptling.plugin.load(url, scriptling=True)` instead of `--plugin-dir`,
then imported through the generated `plugin.hello_http` proxy.

```bash
go run ./examples/plugins/http-go
scriptling examples/plugins/http-go/client.py
```

It exposes:

- `greet(name)` — function via `RegisterFunc`
- `Counter(start).inc(amount)` — class via `RegisterClass`
- `default_name` — constant

HTTP plugin transport is request/response only: it supports handshakes,
generated `import plugin.*` proxies, function calls, object lifecycle, and
batches, but the server cannot initiate callbacks back to the client. Host
callbacks and `plugin.Logger(ctx)` require stdio plugins.

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

## Callback Plugin

`callback` demonstrates passing a Scriptling function into a Go plugin. The
plugin invokes it several times while the outer function is still running.

```bash
go build -o /tmp/scriptling-plugins/callback ./examples/plugins/callback
cat > /tmp/callback-demo.sl <<'EOF'
import plugin.callback
events = []

def on_event(e):
    events.append(e)

print(plugin.callback.stream(on_event))
print(events)
EOF
scriptling --plugin-dir /tmp/scriptling-plugins /tmp/callback-demo.sl
```

## Property Plugin

`properties` demonstrates read-only and read/write properties on a plugin class.

```bash
go build -o /tmp/scriptling-plugins/properties ./examples/plugins/properties
cat > /tmp/properties-demo.sl <<'EOF'
import plugin.properties

c = plugin.properties.Counter(10)
c.value = c.value + 5
print(c.value)
print(c.label)
EOF
scriptling --plugin-dir /tmp/scriptling-plugins /tmp/properties-demo.sl
```

## Logger Plugin

`logger` demonstrates writing plugin logs through the host logger.

```bash
go build -o /tmp/scriptling-plugins/logger ./examples/plugins/logger
scriptling --plugin-dir /tmp/scriptling-plugins -c 'import plugin.logger; print(plugin.logger.work("Ada", ["demo", 1]))'
```
