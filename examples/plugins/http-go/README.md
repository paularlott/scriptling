# HTTP Go Plugin

This example exposes the full Scriptling plugin protocol over HTTP instead of
stdio. The server mounts `plugin.NewServer(...).ServeHTTP` at `/json-rpc`, and a
Scriptling script loads it with `scriptling.plugin.load(..., scriptling=True)`.

HTTP plugin transport is request/response only: it supports plugin handshakes,
function calls, object lifecycle, and batches, but the server cannot initiate
callbacks back to the client. Host callbacks and `plugin.Logger(ctx)` require
the bidirectional stdio transport, so keep using executable plugins for those.

## Run

From the repository root:

```bash
go run ./examples/plugins/http-go
```

In another terminal:

```bash
scriptling examples/plugins/http-go/client.py
```

Expected output:

```text
1.0.0
Hello, Ada
15
unloaded
```

## HTTPS with a self-signed certificate

Clients can opt into skipping TLS verification for trusted local/internal
endpoints:

```python
name = scriptling.plugin.load(
    "hello_http",
    "https://127.0.0.1:8443/json-rpc",
    scriptling=True,
    insecure_skip_tls=True,
    headers={"Authorization": "Bearer token"},
)
```
