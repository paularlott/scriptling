# MCP Per-User Tools Example

This example shows Scriptling's MCP server exposing a **different tool set to
every caller**. Middleware authenticates each request and registers the MCP
entries that caller is allowed to see — for the life of that request. There is
no global registry to keep in sync: every MCP message over HTTP runs the
middleware again, so `tools/list` shows exactly what that caller's credentials
earn them, and `tools/call` re-checks it — one user can never invoke another
user's tool, even by name.

| Caller | Tools | Resources / prompts |
|---|---|---|
| no token | — (401) | — |
| `alice` | `greet`, `alpha_tool` | — |
| `bob` | `greet`, `beta_tool`, `gamma_tool` | `user://bob/notes/{topic}` resource, `bob_report` prompt |
| `carol` | `greet` | — |

`greet` is a static tool (registered from `tools/` at startup, visible to
everyone); everything else is registered per request by the middleware in
`auth.py`. Static entries always win on a name collision, so a per-user tool
can never shadow a tool the whole server shares.

## Layout

```
setup.py        registers the middleware and starts the server
auth.py         the middleware: token -> user, then per-user registrations
usertools.py    implementations for the per-user tools, resource and prompt
tools/          static tools served to everyone (greet)
```

## Running the Example

```bash
scriptling --server :8000 --mcp-tools examples/mcp-per-user-tools/tools examples/mcp-per-user-tools/setup.py
```

Point any MCP client at `http://localhost:8000/mcp` with an
`Authorization` header. With curl:

```bash
# alice sees greet + alpha_tool
curl -s -H 'Authorization: Bearer alice-key' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' localhost:8000/mcp

# bob sees greet + beta_tool + gamma_tool
curl -s -H 'Authorization: Bearer bob-key' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' localhost:8000/mcp

# and can call one of his own tools
curl -s -H 'Authorization: Bearer bob-key' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"beta_tool","arguments":{"x":"hi"}}}' localhost:8000/mcp

# but alice cannot — the middleware for her request never registered it
curl -s -H 'Authorization: Bearer alice-key' \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"beta_tool","arguments":{"x":"hi"}}}' localhost:8000/mcp
```

## The middleware

```python
import scriptling.runtime.mcp as mcp

def check(request):
    user = TOKENS.get(request.header("authorization", ""))
    if user is None:
        return {"status": 401, "body": "unauthorized"}
    request.context["user"] = user

    if user == "alice":
        mcp.register_request_tool("alpha_tool", handler="usertools.alpha", ...)
    ...
    return None
```

Handlers are ordinary `"module.function"` references: they run on a fresh
interpreter per call, read their arguments as keyword parameters (or via
`tool.get_string()`), and can ask `tool.request_context()` who is calling.

## Over stdio

Middleware only runs over HTTP. When the same app is served with
`scriptling --mcp-tools ... --stdio` (no HTTP server), gate on
`mcp.transport()` and register the entries unconditionally instead:

```python
if mcp.transport() == "stdio":
    # No middleware over stdio: expose the extra tools to everyone.
    ...
```
