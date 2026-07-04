# MCP Resources & Prompts Example

This example shows Scriptling's MCP server exposing **tools, resources, and
prompts** — all three MCP primitives — defined as files in folders. No Go code,
no startup registration: drop files in the right directory and they are served.

It demonstrates the file conventions:

- **Tools** (`--mcp-tools`): `name.toml` + `name.py`.
- **Resources** (`--mcp-resources`): files served verbatim, **no metadata**.
  A `{var}` in the path makes a resource template; the `.py` is run with the
  extracted variable.
- **Prompts** (`--mcp-prompts`): `name.md` for a static prompt, or
  `name.toml` + `name.py` for an arg-driven prompt.

## Layout

```
tools/
  greet.toml          tool metadata (description, parameters)
  greet.py            tool implementation
resources/
  docs/about.md       STATIC resource -> docs://about.md (served verbatim)
  greeting/{name}.py  TEMPLATE resource -> greeting://{name} (.py is run)
prompts/
  summarize.md        STATIC prompt -> "summarize" (single user message)
  review.toml         prompt metadata (declares arguments)
  review.py           prompt implementation (dynamic, arg-driven)
```

## Resources: the path is the URI

The first directory under `--mcp-resources` is the URI scheme; the rest of the
path mirrors the URI. So:

| File | URI | Behaviour |
|---|---|---|
| `resources/docs/about.md` | `docs://about.md` | static — file served verbatim |
| `resources/greeting/{name}.py` | `greeting://{name}` | template — `.py` run with `name` |

Read `greeting://Ada` and the `{name}.py` runs with `name = "Ada"`. A `.py` with
**no** `{var}` in its path is served as source text, not executed.

## Running

```bash
# Over stdio (the transport MCP hosts use for a subprocess)
scriptling \
  --mcp-tools ./tools \
  --mcp-resources ./resources \
  --mcp-prompts ./prompts

# Or over HTTP
scriptling --server :8000 \
  --mcp-tools ./tools \
  --mcp-resources ./resources \
  --mcp-prompts ./prompts
```

## Trying it (HTTP)

```bash
# Tools
curl -X POST http://127.0.0.1:8000/mcp -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"greet","arguments":{"name":"Ada"}}}'

# Static resource (file served verbatim)
curl -X POST http://127.0.0.1:8000/mcp -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"docs://about.md"}}'

# Templated resource (.py runs with the {name} var)
curl -X POST http://127.0.0.1:8000/mcp -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"greeting://Ada"}}'

# List resources and templates
curl -X POST http://127.0.0.1:8000/mcp -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"resources/list"}'
curl -X POST http://127.0.0.1:8000/mcp -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":5,"method":"resources/templates/list"}'

# Static prompt (returns the .md as a user message)
curl -X POST http://127.0.0.1:8000/mcp -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":6,"method":"prompts/get","params":{"name":"summarize"}}'

# Dynamic prompt (.py runs with declared arguments)
curl -X POST http://127.0.0.1:8000/mcp -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":7,"method":"prompts/get","params":{"name":"review","arguments":{"code":"print(1)","language":"python"}}}'
```

## Live reload

Editing, adding, or removing a file in any of the three folders triggers an
automatic debounced reload, and the server emits `notifications/tools/listChanged`,
`notifications/resources/listChanged`, and `notifications/prompts/listChanged` so
connected clients re-fetch. `SIGHUP` / `SIGUSR1` force a reload.

## See also

- [MCP Server Mode](https://scriptling.org/docs/cli/mcp-server/)
- [MCP client library](https://scriptling.org/reference/libraries/scriptling/mcp/client/)
- [mcp-tools example](../mcp-tools/) — tools only
