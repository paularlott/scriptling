# JSON-RPC Server Example

This example demonstrates how to build a concurrent JSON-RPC 2.0 server using
the `scriptling.runtime.jsonrpc` library. The same handler registration works
over stdio or over HTTP.

## What It Shows

- Registering JSON-RPC methods with `runtime.jsonrpc.method(name, "lib.func")`
- Registering fire-and-forget notifications with `runtime.jsonrpc.notification()`
- Returning structured errors with `runtime.jsonrpc.error(code, message, data)`
- Handler libraries referenced by string (each request runs on a fresh,
  isolated evaluator, the same concurrency model as `runtime.http`)
- Batches and notifications (handled by the server, no extra code needed)

## Files

| File          | Purpose                                           |
| ------------- | ------------------------------------------------- |
| `setup.py`    | Entry point — registers methods and notifications |
| `handlers.py` | Handler functions invoked per request             |

## Running the Example

Start the stdio server from the project root:

```bash
scriptling --json-rpc examples/jsonrpc-server/setup.py
```

The server reads newline-delimited JSON-RPC 2.0 messages from stdin and writes
one response per line to stdout. Logs go to stderr and never corrupt the stream.

Start the HTTP server instead:

```bash
scriptling --server :8000 --json-rpc examples/jsonrpc-server/setup.py
```

HTTP JSON-RPC is served at `POST /json-rpc`. It can run alongside normal
`runtime.http` routes and MCP tools, for example:

```bash
scriptling --server :8000 --json-rpc --mcp-tools examples/mcp-tools/tools examples/jsonrpc-server/setup.py
```

## Talking to the Stdio Server

Pipe requests in on stdin:

```bash
# A simple request
echo '{"jsonrpc":"2.0","method":"echo","params":{"hello":"world"},"id":1}' \
  | scriptling --json-rpc examples/jsonrpc-server/setup.py
# {"jsonrpc":"2.0","result":{"hello":"world"},"id":1}

# A mixed batch: request responses are returned as one array; notifications
# in the batch are handled but omitted from the response.
echo '[{"jsonrpc":"2.0","method":"add","params":{"a":2,"b":3},"id":1},
       {"jsonrpc":"2.0","method":"progress","params":{"done":1,"total":2}},
       {"jsonrpc":"2.0","method":"add","params":{"a":10,"b":5},"id":2}]' \
  | scriptling --json-rpc examples/jsonrpc-server/setup.py
# [{"jsonrpc":"2.0","result":5,"id":1},{"jsonrpc":"2.0","result":15,"id":2}]

# A structured error from runtime.jsonrpc.error()
echo '{"jsonrpc":"2.0","method":"divide","params":{"a":1,"b":0},"id":3}' \
  | scriptling --json-rpc examples/jsonrpc-server/setup.py
# {"jsonrpc":"2.0","error":{"code":-32602,"message":"division by zero","data":{"field":"b"}},"id":3}

# A notification (no id) produces no response at all
echo '{"jsonrpc":"2.0","method":"progress","params":{"done":3,"total":10}}' \
  | scriptling --json-rpc examples/jsonrpc-server/setup.py
# (no output)

# An all-notification batch also produces no response
echo '[{"jsonrpc":"2.0","method":"progress","params":{"done":1}},
       {"jsonrpc":"2.0","method":"progress","params":{"done":2}}]' \
  | scriptling --json-rpc examples/jsonrpc-server/setup.py
# (no output)
```

## Talking to the HTTP Server

Send the same JSON-RPC objects to `/json-rpc`:

```bash
curl -X POST http://127.0.0.1:8000/json-rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"echo","params":{"hello":"world"},"id":1}'
# {"jsonrpc":"2.0","result":{"hello":"world"},"id":1}

curl -X POST http://127.0.0.1:8000/json-rpc \
  -H "Content-Type: application/json" \
  -d '[{"jsonrpc":"2.0","method":"add","params":{"a":2,"b":3},"id":1},
       {"jsonrpc":"2.0","method":"progress","params":{"done":1,"total":2}},
       {"jsonrpc":"2.0","method":"add","params":{"a":10,"b":5},"id":2}]'
# [{"jsonrpc":"2.0","result":5,"id":1},{"jsonrpc":"2.0","result":15,"id":2}]

curl -i -X POST http://127.0.0.1:8000/json-rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"progress","params":{"done":3,"total":10}}'
# HTTP/1.1 204 No Content
```

## Concurrency Model

Each request is dispatched on its own goroutine with a fresh Scriptling
evaluator, so a slow handler never blocks a fast one. This matches
`runtime.http`, MCP, and WebSocket serving. Handlers cannot share in-memory
state across requests; coordinate through `runtime.kv` or `runtime.sync`
instead.

## Key Points

- Handlers are referenced as `"library.function"` strings, not closures, so the
  server can spin up an isolated evaluator per request.
- A single JSON-RPC object is handled as one call; a JSON-RPC array is handled
  as a batch and replies with one array containing only request responses.
- Notifications are requests without an `id`. They run their registered handler
  and never produce a response, including inside batches. HTTP notifications
  return `204 No Content`.
- Response logging must target stderr; stdout is the protocol stream.
- Unknown methods return `-32601`; handler exceptions return `-32000`.
- `runtime.jsonrpc.error()` lets a handler emit any JSON-RPC error code/data.

## See Also

- [scriptling.runtime.jsonrpc documentation](https://scriptling.dev/reference/libraries/scriptling/runtime/jsonrpc/)
