# MCP Tools Example

This example demonstrates how to use Scriptling's built-in MCP (Model Context Protocol) server to expose tools to AI assistants like Claude.

## What is MCP?

MCP is a protocol that allows AI assistants to discover and execute tools. Scriptling's built-in MCP server lets you write tools in Python and expose them through a standard HTTP endpoint.

## Tool Structure

Each tool consists of two files in the tools directory:

| File | Purpose |
|------|---------|
| `toolname.toml` | Metadata: description, keywords, parameters |
| `toolname.py` | Implementation: the actual tool logic |

## Example Tools

This example includes four tools:

| Tool | Description |
|------|-------------|
| `greet` | Generate personalized greeting messages |
| `encode_base64` | Encode text to base64 |
| `decode_base64` | Decode base64 to text |
| `format_json` | Format and validate JSON strings |

## Running the MCP Server

Start the MCP server from the project root:

```bash
# Basic usage
scriptling --server :8000 --mcp-tools examples/mcp-tools/tools

# With authentication
scriptling --server :8000 --mcp-tools examples/mcp-tools/tools --bearer-token your-secret-token

# With TLS (self-signed certificate)
scriptling --server :8443 --mcp-tools examples/mcp-tools/tools --tls-generate
```

## MCP Endpoint

The MCP server is available at `/mcp`. This endpoint handles JSON-RPC 2.0 requests.

## Testing with curl

List available tools:

```bash
curl -X POST http://localhost:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

Call a tool:

```bash
curl -X POST http://localhost:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"greet","arguments":{"name":"World","enthusiasm":3}}}'
```

## Writing a Tool

### 1. Create the Metadata (TOML)

```toml
description = "Brief description of what the tool does"
keywords = ["related", "search", "terms"]
discoverable = true  # Optional: hide from search results

[[parameters]]
name = "param_name"
type = "string"  # string, int, float, bool
description = "What this parameter does"
required = true

[[parameters]]
name = "optional_param"
type = "int"
description = "An optional parameter"
required = false
```

### 2. Create the Implementation (Python)

```python
import scriptling.mcp.tool as tool

# Get parameters with defaults
name = tool.get_string("name")
count = tool.get_int("count", 1)  # Default: 1
enabled = tool.get_bool("enabled", False)  # Default: False
value = tool.get_float("value", 0.0)  # Default: 0.0

# Your tool logic here
result = f"Hello, {name}!"

# Return the result
tool.return_string(result)

# Or return an error
# tool.return_error("Something went wrong")
```

## Parameter Functions

| Function | Description |
|----------|-------------|
| `tool.get_string(name, default=None)` | Get a string parameter |
| `tool.get_int(name, default=None)` | Get an integer parameter |
| `tool.get_float(name, default=None)` | Get a float parameter |
| `tool.get_bool(name, default=None)` | Get a boolean parameter |

## Return Functions

| Function | Description |
|----------|-------------|
| `tool.return_string(text)` | Return a text result |
| `tool.return_error(message)` | Return an error message |

## Hot Reloading

The MCP server automatically reloads tools when `.toml` files change. You can also trigger a manual reload:

```bash
# Send SIGHUP to reload tools
kill -HUP <pid>
```

## Using with Claude Desktop

Add to your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "scriptling": {
      "url": "http://localhost:8000/mcp",
      "headers": {
        "Authorization": "Bearer your-secret-token"
      }
    }
  }
}
```

## Using with Libraries

Tools can import libraries from the same directory or from `--libdir`:

```python
import mylib  # Loads mylib.py from tools directory or libdir
import scriptling.mcp.tool as tool

result = mylib.process(tool.get_string("input"))
tool.return_string(result)
```

## Key Points

- Each tool must have both `.toml` and `.py` files with matching names
- Parameters are accessed via `tool.get_*()` functions
- Always return a result with `tool.return_string()` or `tool.return_error()`
- Tools run in isolated environments with full standard library access
- The server supports hot-reloading for development

## See Also

- [HTTP Server Example](../http-server/) - HTTP routes and handlers
- [scriptling.runtime.http documentation](../../docs/libraries/scriptling/runtime-http.md)
