# PHP Plugin Server

A Scriptling plugin written in plain PHP, served over HTTP. It exists to show
the plugin protocol in another language: the whole contract is two JSON-RPC
methods (`scriptling.handshake` and `function.call`) over HTTP POST, so any
language that can read and write JSON can serve one.

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
scriptling --plugin https://plugins.internal:8443 --plugin-insecure app.py
```

`--plugin-insecure` applies to the https plugin URLs of that run.

## What the server implements

| Method | Purpose |
|---|---|
| `scriptling.handshake` | answers the protocol version and declares the library: its name (imported as `plugin.phpdemo`), version, and function schema |
| `function.call` | dispatches `greet`, `echo` (any value round trip) and `server_info` |

Values travel as tagged objects (`{"type": "string", "value": "..."}` for a
string, `entries` for a dict), which is how scripts see native types across
the wire. Unknown methods and functions answer with JSON-RPC errors, which
the host surfaces to scripts as ordinary errors.
