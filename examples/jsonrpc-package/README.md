# JSON-RPC Package Example

A JSON-RPC 2.0 server shipped as a package — no script file or flags beyond
`--package` and the transport selector.

## Layout

```
jsonrpc-package/
├── manifest.toml     # serve = ["json-rpc"]
├── setup.py          # registers methods (the main entry point)
└── lib/
    └── handlers.py   # method handlers
```

## Demo: stdio mode

The package speaks newline-delimited JSON-RPC 2.0 on stdin/stdout.

```bash
# Run from a folder (development)
echo '{"jsonrpc":"2.0","method":"echo","params":{"msg":"hi"},"id":1}' | \
  scriptling --package examples/jsonrpc-package

# Or from a zip (production)
scriptling pack examples/jsonrpc-package jsonrpc-package.zip
echo '{"jsonrpc":"2.0","method":"add","params":{"a":2,"b":3},"id":2}' | \
  scriptling --package jsonrpc-package.zip
```

## Demo: HTTP mode

Mount the JSON-RPC endpoint at `/json-rpc` over HTTP.

```bash
# Run from a folder (development)
scriptling --server :8000 --package examples/jsonrpc-package

# Or from a zip (production)
scriptling --server :8000 --package jsonrpc-package.zip
```

```bash
# Call a method
curl -X POST http://127.0.0.1:8000/json-rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"echo","params":{"msg":"hi"},"id":1}'
```

## Methods

| Method | Params | Returns |
|--------|--------|---------|
| `echo` | any | params unchanged |
| `add` | `{"a": int, "b": int}` | `a + b` |
| `divide` | `{"a": int, "b": int}` | `a / b` (error if `b == 0`) |
| `progress` (notification) | any | nothing (side-effect only) |
