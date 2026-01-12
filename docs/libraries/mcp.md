# MCP Library

MCP (Model Context Protocol) tool interaction library. This library provides functions for interacting with MCP servers that expose tools for AI models to use.

For MCP integration with OpenAI clients, see the [AI Library](ai.md) documentation.

## Module Functions

### mcp.decode_response(response)

Decodes a raw MCP tool response into scriptling objects.

**Parameters:**
- `response` (dict): Raw tool response dict

**Returns:** object - Decoded response (parsed JSON or string)

**Example:**
```python
import mcp

decoded = mcp.decode_response(raw_response)
```

### mcp.new_client(base_url, **kwargs)

Creates a new MCP client for connecting to a remote MCP server.

**Parameters:**
- `base_url` (str): URL of the MCP server
- `namespace` (str, optional): Namespace for tool names (e.g., "scriptling" makes tools available as "scriptling/tool_name")
- `bearer_token` (str, optional): Bearer token for authentication

**Returns:** MCPClient - A client instance with methods for interacting with the server

**Example:**
```python
import mcp

# Without namespace or auth
client = mcp.new_client("https://api.example.com/mcp")

# With namespace only
client = mcp.new_client("https://api.example.com/mcp", namespace="scriptling")

# With bearer token only
client = mcp.new_client("https://api.example.com/mcp", bearer_token="your-token-here")

# With both namespace and bearer token
client = mcp.new_client(
    "https://api.example.com/mcp",
    namespace="scriptling",
    bearer_token="your-token-here"
)
```

**Note:** When using a namespace, all tool names will be prefixed. For example, if the server has a tool called "execute_code" and you use namespace "scriptling", the tool will be available as "scriptling/execute_code". The namespace is automatically added to all tool names and stripped when calling tools.

## MCPClient Class

### client.tools()

Lists all tools available from this MCP server.

**Returns:** list - List of tool dicts with name, description, input_schema

**Example:**
```python
client = mcp.new_client("https://api.example.com/mcp")
tools = client.tools()

for tool in tools:
    print(f"{tool.name}: {tool.description}")
    if "input_schema" in tool:
        print(f"  Schema: {tool.input_schema}")
```

### client.call_tool(name, arguments={})

Executes a tool by name with the provided arguments.

**Parameters:**
- `name` (str): Tool name to execute
- `arguments` (dict, optional): Tool arguments (default: {})

**Returns:** dict - Decoded tool response

**Example:**
```python
client = mcp.new_client("https://api.example.com/mcp")

result = client.call_tool("search", {
    "query": "golang programming",
    "limit": 10
})

print(result)
```

### client.refresh_tools()

Explicitly refreshes the cached list of tools from the server.

**Returns:** null

**Example:**
```python
client = mcp.new_client("https://api.example.com/mcp")

# Tools are cached, refresh to get latest
client.refresh_tools()

tools = client.tools()
```

### client.tool_search(query, **kwargs)

Searches for tools using the tool_search MCP tool. This is useful when the server has many tools registered via a discovery registry.

**Parameters:**
- `query` (str): Search query for tool names, descriptions, and keywords
- `max_results` (int, optional): Maximum number of results (default: 10)

**Returns:** list - List of matching tool dicts

**Example:**
```python
client = mcp.new_client("https://api.example.com/mcp")

# Search for weather-related tools (default: up to 10 results)
results = client.tool_search("weather")

# Search with custom limit
results = client.tool_search("database", max_results=5)

for tool in results:
    print(f"{tool.name}: {tool.description}")
```

### client.execute_discovered(name, arguments={})

Executes a tool by name using the execute_tool MCP tool. This is the only way to call tools that were discovered via tool_search.

**Parameters:**
- `name` (str): Tool name to execute
- `arguments` (dict, optional): Tool arguments (default: {})

**Returns:** dict - Tool response

**Example:**
```python
client = mcp.new_client("https://api.example.com/mcp")

# First search for tools
results = client.tool_search("weather")
if results:
    # Then execute a discovered tool
    result = client.execute_discovered(results[0].name, {
        "location": "San Francisco"
    })
    print(result)
```

## Usage Examples

### Basic Tool Execution

```python
import mcp

client = mcp.new_client("https://api.example.com/mcp")
result = client.call_tool("calculator", {"expression": "2+2"})
print(result)  # 4
```

### Listing Available Tools

```python
import mcp

client = mcp.new_client("https://api.example.com/mcp")
tools = client.tools()

print(f"Available tools: {len(tools)}")
for tool in tools:
    print(f"  - {tool.name}")
    print(f"    {tool.description}")
```

### Using Tool Schemas

```python
import mcp

client = mcp.new_client("https://api.example.com/mcp")
tools = client.tools()

for tool in tools:
    if tool.name == "search":
        # Check what parameters are required
        schema = tool.input_schema
        print(f"Search tool schema: {schema}")

        # Call with proper arguments
        result = client.call_tool("search", {
            "query": "golang",
            "limit": 5
        })
```

### Searching for Tools

```python
import mcp

client = mcp.new_client("https://api.example.com/mcp")

# Find database-related tools
db_tools = client.tool_search("database", max_results=20)

for tool in db_tools:
    print(f"{tool.name}: {tool.description}")

    # Execute if found
    if tool.name == "query_database":
        result = client.execute_discovered("query_database", {
            "sql": "SELECT * FROM users LIMIT 10"
        })
        print(result)
```

## Authentication

### Bearer Token Authentication

```python
import mcp

client = mcp.new_client(
    "https://api.example.com/mcp",
    bearer_token="your-api-token"
)
```

### Bearer Token with Namespace

```python
import mcp

# Namespace and bearer token can be in any order
client = mcp.new_client(
    "https://api.example.com/mcp",
    namespace="myservice",
    bearer_token="your-api-token"
)
```

### No Authentication

```python
import mcp

client = mcp.new_client("https://public-api.example.com/mcp")
```

## Error Handling

```python
import mcp

try:
    client = mcp.new_client("https://api.example.com/mcp")
    result = client.call_tool("search", {"query": "golang"})
    print(result)
except Exception as e:
    print("Tool execution failed:", e)
```

## Tool Response Format

Tool responses are automatically decoded from JSON:

```python
import mcp

client = mcp.new_client("https://api.example.com/mcp")

# Response is automatically parsed
result = client.call_tool("get_weather", {"city": "London"})

# Access response data
print(result.temperature)  # 15
print(result.condition)    # "Partly cloudy"
print(result.forecast)     # [...]
```

For raw responses, use `mcp.decode_response()`:

```python
import mcp

raw_response = {
    "content": [{"type": "text", "text": '{"temp": 15}'}]
}

decoded = mcp.decode_response(raw_response)
print(decoded)  # {"temp": 15}
```

## Tool Schema

Tools may include an input schema defining their parameters:

```python
import mcp

client = mcp.new_client("https://api.example.com/mcp")
tools = client.tools()

for tool in tools:
    if tool.name == "search":
        schema = tool.input_schema
        # Example schema:
        # {
        #     "type": "object",
        #     "properties": {
        #         "query": {"type": "string"},
        #         "limit": {"type": "integer"}
        #     },
        #     "required": ["query"]
        # }
```
