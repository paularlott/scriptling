# MCP over stdio

Scriptling can run as an MCP server over **stdio** (newline-delimited JSON-RPC
2.0 on stdin/stdout) — the transport MCP hosts use to launch a server as a
subprocess — and it can **consume** stdio MCP servers from a script.

## Scriptling as a stdio MCP server

The MCP tool flags (`--mcp-tools <dir>` and/or `--mcp-exec-script`) enable the
MCP server. Without `--server` it serves over stdio instead of HTTP:

```bash
# Serve the example tools over stdio
scriptling --mcp-tools examples/mcp-tools/tools

# Or just the code-execution tool
scriptling --mcp-exec-script
```

Logs go to stderr in this mode so stdout stays a clean protocol stream. Try it
by piping requests in:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | scriptling --mcp --mcp-exec-script
```

### Using with an MCP host (e.g. Claude Desktop)

```json
{
  "mcpServers": {
    "scriptling": {
      "command": "scriptling",
      "args": ["--mcp-tools", "/absolute/path/to/tools"]
    }
  }
}
```

Add `--server <addr>` and the same tools are served over HTTP at `/mcp` — see
[../mcp-tools/](../mcp-tools/).

## Consuming a stdio MCP server from a script

`mcp.Client()` chooses the transport from its first argument: an `http://` or
`https://` URL connects over HTTP; anything else is launched as a stdio server
subprocess.

```python
import scriptling.mcp as mcp

# stdio: launch a local server binary
client = mcp.Client("scriptling", args=["--mcp-exec-script"], namespace="local")

# HTTP: connect to a URL
# client = mcp.Client("https://example.com/mcp", namespace="remote", bearer_token="secret")

for tool in client.tools():
    print(tool.name)

result = client.call_tool("local__execute_script", {"code": "print(6 * 7)"})
print(result)

client.close()  # shuts the subprocess down (no-op for HTTP)
```

Run the full example:

```bash
scriptling examples/mcp-stdio/client.py
```

## See Also

- [MCP Tools](../mcp-tools/) — writing tools and serving them over HTTP
- [MCP Client](../mcp-client/) — embedding the client from Go
