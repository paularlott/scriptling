# JSON-RPC stdio Server Example

This example demonstrates how to build a concurrent JSON-RPC 2.0 server over
stdin/stdout using the `scriptling.runtime.jsonrpc` library.

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
scriptling --jsonrpc examples/jsonrpc-server/setup.py
```

The server reads newline-delimited JSON-RPC 2.0 messages from stdin and writes
one response per line to stdout. Logs go to stderr and never corrupt the stream.

## Talking to the Server

Pipe requests in on stdin:

```bash
# A simple request
echo '{"jsonrpc":"2.0","method":"echo","params":{"hello":"world"},"id":1}' \
  | scriptling --jsonrpc examples/jsonrpc-server/setup.py
# {"jsonrpc":"2.0","result":{"hello":"world"},"id":1}

# Two requests in one batch (returned as a single JSON array)
echo '[{"jsonrpc":"2.0","method":"add","params":{"a":2,"b":3},"id":1},
       {"jsonrpc":"2.0","method":"add","params":{"a":10,"b":5},"id":2}]' \
  | scriptling --jsonrpc examples/jsonrpc-server/setup.py
# [{"jsonrpc":"2.0","result":5,"id":1},{"jsonrpc":"2.0","result":15,"id":2}]

# A structured error from runtime.jsonrpc.error()
echo '{"jsonrpc":"2.0","method":"divide","params":{"a":1,"b":0},"id":3}' \
  | scriptling --jsonrpc examples/jsonrpc-server/setup.py
# {"jsonrpc":"2.0","error":{"code":-32602,"message":"division by zero","data":{"field":"b"}},"id":3}

# A notification (no id) produces no response at all
echo '{"jsonrpc":"2.0","method":"progress","params":{"done":3,"total":10}}' \
  | scriptling --jsonrpc examples/jsonrpc-server/setup.py
# (no output)
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
- Response logging must target stderr; stdout is the protocol stream.
- Unknown methods return `-32601`; handler exceptions return `-32000`.
- `runtime.jsonrpc.error()` lets a handler emit any JSON-RPC error code/data.

## See Also

- [scriptling.runtime.jsonrpc documentation](https://scriptling.dev/reference/libraries/scriptling/runtime/jsonrpc/)
