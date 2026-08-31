# PHP Plugin Server

A Scriptling plugin written in plain PHP, served over HTTP. It exists to show
the plugin protocol in another language: the contract is a handful of
JSON-RPC methods over HTTP POST (`scriptling.handshake`, `function.call`, and
the `object.*` pair for classes), so any language that can read and write
JSON can serve one.

## Run it

With PHP's built-in server:

```bash
PHPDEMO_FROM=my-laptop php -S 127.0.0.1:8080 index.php
```

`PHPDEMO_FROM` is optional (it defaults to `php`) and demonstrates the one
difference between the plugin kinds: an HTTP plugin owns its own environment,
because the host connects to it rather than spawning it. `--plugin-env` passes
variables to executable plugins, which the host does spawn.

Then load it and call it:

```bash
scriptling --plugin http://127.0.0.1:8080 -c '
import plugin.phpdemo as d

print(d.greet("Ada"))            # Hello, Ada (from my-laptop)
print(d.echo({"a": 1, "b": [2, 3]}))   # values round trip any type
info = d.server_info()
print(info["php"], info["library"])
'
```

## Classes

The handshake's schema lists classes and their methods; the host turns that
into a proxy class. Instances answer `object.new` (construct) and
`object.call_method` (any listed method):

```bash
scriptling --plugin http://127.0.0.1:8080 -c '
import plugin.phpdemo as d

g = d.Greeter("Ada")
print(g.greet())          # Hello, Ada (from php)
print(g.shout())          # HELLO, ADA (FROM PHP)
g = g.rename("Bob")       # mutation returns a fresh instance; rebind
print(g.greet())          # Hello, Bob (from php)
'
```

The PHP built-in server forgets everything between requests, so this example
keeps each Greeter's state inside the object id itself (base64 JSON) and
`rename` returns a new instance instead of mutating. A server with storage (a
database, Redis) would keep instances there and put its key in the id; the
protocol takes either.

## Authentication

Start the server with a token and it demands it on every request:

```bash
PHPDEMO_TOKEN=seekrit php -S 127.0.0.1:8080 index.php
```

```bash
# refused: Error: plugin http://127.0.0.1:8080 failed to load: ... 401
scriptling --plugin http://127.0.0.1:8080 -c 'import plugin.phpdemo'

# accepted
scriptling --plugin http://127.0.0.1:8080            --plugin-header "Authorization=Bearer seekrit"            -c 'import plugin.phpdemo as d; print(d.greet("Ada"))'
```

Username and password in the URL travel as Basic auth instead:
`--plugin https://user:pass@host:8443`. An explicit Authorization header wins
over URL credentials.

## https and self-signed certificates

Terminate TLS in front of the server (a reverse proxy, or a development
certificate for the built-in server). A certificate a CA signed loads without
anything extra; a self-signed one needs the host to skip verification:

```bash
scriptling --plugin https://plugins.internal:8443 \
           --plugin-insecure https://plugins.internal:8443 app.py
```

Verification is skipped only for the URLs the flag names.

## What the server implements

| Method | Purpose |
|---|---|
| `scriptling.handshake` | answers the protocol version and declares the library: its name (imported as `plugin.phpdemo`), version, function schema and class schema |
| `function.call` | dispatches `greet`, `echo` (any value round trip) and `server_info` |
| `object.new` | constructs a class instance, answering with its remote reference |
| `object.call_method` | dispatches `Greeter.greet`, `Greeter.shout` and `Greeter.rename` |
| `object.destroy` | instances are stateless here (the id is the state), so this is a no-op |

Values travel as tagged objects (`{"type": "string", "value": "..."}` for a
string, `entries` for a dict), which is how scripts see native types across
the wire. Unknown methods and functions answer with JSON-RPC errors, which
the host surfaces to scripts as ordinary errors.
