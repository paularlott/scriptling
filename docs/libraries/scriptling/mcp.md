# MCP Library

MCP (Model Context Protocol) tool interaction library. This library provides functions for interacting with MCP servers that expose tools for AI models to use.

For MCP integration with OpenAI clients, see the [AI Library](ai.md) documentation.

## Module Functions

### scriptling.mcp.decode_response(response)

Decodes a raw MCP tool response into scriptling objects.

**Parameters:**

- `response` (dict): Raw tool response dict

**Returns:** object - Decoded response (parsed JSON or string)

**Example:**

```python
import scriptling.mcp as mcp

decoded = mcp.decode_response(raw_response)
```

### scriptling.mcp.Client(base_url, \*\*kwargs)

Creates a new MCP client for connecting to a remote MCP server.

**Parameters:**

- `base_url` (str): URL of the MCP server
- `namespace` (str, optional): Namespace for tool names (e.g., "scriptling" makes tools available as "scriptling/tool_name")
- `bearer_token` (str, optional): Bearer token for authentication

**Returns:** MCPClient - A client instance with methods for interacting with the server

**Example:**

```python
import scriptling.mcp as mcp

# Without namespace or auth
client = mcp.Client("https://api.example.com/mcp")

# With namespace only
client = mcp.Client("https://api.example.com/mcp", namespace="scriptling")

# With bearer token only
client = mcp.Client("https://api.example.com/mcp", bearer_token="your-token-here")

# With both namespace and bearer token
client = mcp.Client(
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
client = mcp.Client("https://api.example.com/mcp")
tools = client.tools()

for tool in tools:
    print(f"{tool.name}: {tool.description}")
    if "input_schema" in tool:
        print(f"  Schema: {tool.input_schema}")
```

### client.call_tool(name, arguments)

Executes a tool by name with the provided arguments.

**Parameters:**

- `name` (str): Tool name to execute
- `arguments` (dict): Tool arguments

**Returns:** dict - Decoded tool response

**Example:**

```python
client = mcp.Client("https://api.example.com/mcp")

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
client = mcp.Client("https://api.example.com/mcp")

# Tools are cached, refresh to get latest
client.refresh_tools()

tools = client.tools()
```

### client.tool_search(query, \*\*kwargs)

Searches for tools using the tool_search MCP tool. This is useful when the server has many tools registered via a discovery registry.

**Parameters:**

- `query` (str): Search query for tool names, descriptions, and keywords
- `max_results` (int, optional): Maximum number of results (default: 10)

**Returns:** list - List of matching tool dicts

**Example:**

```python
client = mcp.Client("https://api.example.com/mcp")

# Search for weather-related tools (default: up to 10 results)
results = client.tool_search("weather")

# Search with custom limit
results = client.tool_search("database", max_results=5)

for tool in results:
    print(f"{tool.name}: {tool.description}")
```

### client.execute_discovered(name, arguments)

Executes a tool by name using the execute_tool MCP tool. This is the only way to call tools that were discovered via tool_search.

**Parameters:**

- `name` (str): Tool name to execute
- `arguments` (dict): Tool arguments

**Returns:** dict - Tool response

**Example:**

```python
client = mcp.Client("https://api.example.com/mcp")

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
import scriptling.mcp as mcp

client = mcp.Client("https://api.example.com/mcp")
result = client.call_tool("calculator", {"expression": "2+2"})
print(result)  # 4
```

### Listing Available Tools

```python
import scriptling.mcp as mcp

client = mcp.Client("https://api.example.com/mcp")
tools = client.tools()

print(f"Available tools: {len(tools)}")
for tool in tools:
    print(f"  - {tool.name}")
    print(f"    {tool.description}")
```

### Using Tool Schemas

```python
import scriptling.mcp as mcp

client = mcp.Client("https://api.example.com/mcp")
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
import scriptling.mcp as mcp

client = mcp.Client("https://api.example.com/mcp")

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
import scriptling.mcp as mcp

client = mcp.Client(
    "https://api.example.com/mcp",
    bearer_token="your-api-token"
)
```

### Bearer Token with Namespace

```python
import scriptling.mcp as mcp

# Namespace and bearer token can be in any order
client = mcp.Client(
    "https://api.example.com/mcp",
    namespace="myservice",
    bearer_token="your-api-token"
)
```

### No Authentication

```python
import scriptling.mcp as mcp

client = mcp.Client("https://public-api.example.com/mcp")
```

## Error Handling

```python
import scriptling.mcp as mcp

try:
    client = mcp.Client("https://api.example.com/mcp")
    result = client.call_tool("search", {"query": "golang"})
    print(result)
except Exception as e:
    print("Tool execution failed:", e)
```

## Tool Response Format

Tool responses are automatically decoded from JSON:

```python
import scriptling.mcp as mcp

client = mcp.Client("https://api.example.com/mcp")

# Response is automatically parsed
result = client.call_tool("get_weather", {"city": "London"})

# Access response data
print(result.temperature)  # 15
print(result.condition)    # "Partly cloudy"
print(result.forecast)     # [...]
```

For raw responses, use `mcp.decode_response()`:

```python
import scriptling.mcp as mcp

raw_response = {
    "content": [{"type": "text", "text": '{"temp": 15}'}]
}

decoded = mcp.decode_response(raw_response)
print(decoded)  # {"temp": 15}
```

## Tool Schema

Tools may include an input schema defining their parameters:

```python
import scriptling.mcp as mcp

client = mcp.Client("https://api.example.com/mcp")
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

## Using MCP Tools with AI

MCP servers can be configured during AI client creation using the `remote_servers` parameter. This allows AI models to automatically call MCP tools during completions.

```python
import scriptling.ai as ai
import scriptling.mcp as mcp

# Create AI client with MCP servers configured
ai_client = ai.Client("http://127.0.0.1:1234/v1", remote_servers=[
    {"base_url": "http://127.0.0.1:8080/mcp", "namespace": "scriptling"},
])

# AI can now automatically use tools from the MCP server
response = ai_client.completion(
    "gpt-4",
    [{"role": "user", "content": "Calculate 15 + 27 using the execute_code tool"}]
)
print(response.choices[0].message.content)
```

### Combined Usage

You can use both MCP and AI clients together - one for direct tool access and one for AI completions:

```python
import scriptling.ai as ai
import scriptling.mcp as mcp

# Create AI client with MCP servers configured
ai_client = ai.Client("http://127.0.0.1:1234/v1", remote_servers=[
    {"base_url": "http://127.0.0.1:8080/mcp", "namespace": "scriptling"},
])

# Create MCP client for direct tool access
mcp_client = mcp.Client("http://127.0.0.1:8080/mcp", namespace="scriptling")

# List tools directly
tools = mcp_client.tools()
print(f"Available tools: {len(tools)}")
for tool in tools:
    print(f"  - {tool.name}: {tool.description}")

# Or let the AI use them automatically
response = ai_client.completion(
    "gpt-4",
    [{"role": "user", "content": "What tools are available?"}]
)
print(response.choices[0].message.content)
```

### Namespace Prefixing

When using MCP tools with AI, tools are prefixed with the namespace:

```python
import scriptling.ai as ai

# With namespace="scriptling", tools become "scriptling/tool_name"
ai_client = ai.Client("http://127.0.0.1:1234/v1", remote_servers=[
    {"base_url": "http://127.0.0.1:8080/mcp", "namespace": "scriptling"},
])

# AI will see tools like: scriptling/execute_code, scriptling/tool_search, etc.
response = ai_client.completion(
    "gpt-4",
    [{"role": "user", "content": "Use scriptling/execute_code to calculate 15 + 27"}]
)
```

### Multiple MCP Servers

You can configure multiple MCP servers for the AI client:

```python
import scriptling.ai as ai

ai_client = ai.Client("http://127.0.0.1:1234/v1", remote_servers=[
    {"base_url": "http://127.0.0.1:8080/mcp", "namespace": "scriptling"},
    {"base_url": "http://127.0.0.1:8081/mcp", "namespace": "database"},
    {"base_url": "https://api.example.com/mcp", "namespace": "search", "bearer_token": "secret"},
])

# AI now has access to tools from all three servers
response = ai_client.completion(
    "gpt-4",
    [{"role": "user", "content": "Search for recent golang news and save to database"}]
)
```

## See Also

- [AI Library](ai.md) - AI client and completion functions
- [Agent Library](agent.md) - Building AI agents with automatic tool execution

---

## MCP Tool Helpers (scriptling.mcp.tool)

The `scriptling.mcp.tool` sub-library provides helper functions for authoring MCP tools in scriptling. These functions simplify parameter access and result handling when implementing tool scripts.

### Registration

The tool helpers are a separate, optional sub-library. Register it explicitly when needed:

```python
mcp.RegisterToolHelpers(sl)  # Registers scriptling.mcp.tool
```

### Environment Variables

MCP tool helpers read and write to environment variables set by the MCP tool execution environment:

- **`__mcp_params`**: Dictionary containing tool parameters
- **`__mcp_response`**: String containing the tool's result

### Parameter Access Functions

#### mcp.tool.get_int(name, default=0)

Get an integer parameter from the tool's input arguments.

**Parameters:**

- `name` (str): Parameter name
- `default` (int): Default value if parameter is missing (default: 0)

**Returns:** int

**Example:**

```python
import scriptling.mcp.tool as tool

project_id = tool.get_int("project_id", 0)
limit = tool.get_int("limit", 100)
```

#### mcp.tool.get_float(name, default=0.0)

Get a float parameter from the tool's input arguments.

**Parameters:**

- `name` (str): Parameter name
- `default` (float): Default value if parameter is missing (default: 0.0)

**Returns:** float

**Example:**

```python
import scriptling.mcp.tool as tool

price = tool.get_float("price", 0.0)
percentage = tool.get_float("percentage", 100.0)
```

#### mcp.tool.get_string(name, default="")

Get a string parameter from the tool's input arguments. Automatically trims whitespace.

**Parameters:**

- `name` (str): Parameter name
- `default` (str): Default value if parameter is missing or empty (default: "")

**Returns:** str

**Example:**

```python
import scriptling.mcp.tool as tool

name = tool.get_string("name", "guest")
query = tool.get_string("query")
```

#### mcp.tool.get_bool(name, default=false)

Get a boolean parameter from the tool's input arguments. Handles "true"/"false" strings and numeric 0/1.

**Parameters:**

- `name` (str): Parameter name
- `default` (bool): Default value if parameter is missing (default: false)

**Returns:** bool

**Example:**

```python
import scriptling.mcp.tool as tool

enabled = tool.get_bool("enabled", True)
verbose = tool.get_bool("verbose")
```

#### mcp.tool.get_list(name, default=None)

Get a list parameter from the tool's input arguments. If the parameter is a string, splits by comma.

**Parameters:**

- `name` (str): Parameter name
- `default` (list): Default value if parameter is missing (default: empty list)

**Returns:** list

**Example:**

```python
import scriptling.mcp.tool as tool

ids = tool.get_list("ids")              # "1,2,3" → ["1", "2", "3"]
tags = tool.get_list("tags", ["all"])   # "tag1, tag2" → ["tag1", "tag2"]
```

#### mcp.tool.get_string_list(name, default=None)

Get a string array parameter (array:string type) from the tool's input arguments.

**Parameters:**

- `name` (str): Parameter name
- `default` (list): Default value if parameter is missing (default: empty list)

**Returns:** list of strings

**Example:**

```python
import scriptling.mcp.tool as tool

args = tool.get_string_list("arguments")  # ["--verbose", "-o", "file.txt"]
tags = tool.get_string_list("tags", ["default"])
```

#### mcp.tool.get_int_list(name, default=None)

Get an integer array parameter (array:int type) from the tool's input arguments.

**Parameters:**

- `name` (str): Parameter name
- `default` (list): Default value if parameter is missing (default: empty list)

**Returns:** list of integers

**Example:**

```python
import scriptling.mcp.tool as tool

ids = tool.get_int_list("ids")  # [1, 2, 3, 4]
ports = tool.get_int_list("ports", [8080])
```

#### mcp.tool.get_float_list(name, default=None)

Get a float array parameter (array:float type) from the tool's input arguments.

**Parameters:**

- `name` (str): Parameter name
- `default` (list): Default value if parameter is missing (default: empty list)

**Returns:** list of floats

**Example:**

```python
import scriptling.mcp.tool as tool

prices = tool.get_float_list("prices")  # [19.99, 29.99, 39.99]
weights = tool.get_float_list("weights", [1.0])
```

#### mcp.tool.get_bool_list(name, default=None)

Get a boolean array parameter (array:bool type) from the tool's input arguments.

**Parameters:**

- `name` (str): Parameter name
- `default` (list): Default value if parameter is missing (default: empty list)

**Returns:** list of booleans

**Example:**

```python
import scriptling.mcp.tool as tool

flags = tool.get_bool_list("flags")  # [true, false, true]
options = tool.get_bool_list("options", [false])
```

### Result Functions

Result functions set the tool's response and immediately stop script execution using `SystemExit`.

#### mcp.tool.return_string(text)

Return a string result and stop execution.

**Parameters:**

- `text` (str): The result text

**Example:**

```python
import scriptling.mcp.tool as tool

tool.return_string("Search completed successfully")
# Code here will not execute
```

#### mcp.tool.return_object(obj)

Return an object as JSON and stop execution.

**Parameters:**

- `obj` (dict|list): The result object

**Example:**

```python
import scriptling.mcp.tool as tool

tool.return_object({"status": "success", "count": 42})
# Code here will not execute
```

#### mcp.tool.return_toon(obj)

Return an object encoded as TOON (compact text format optimized for LLMs) and stop execution.

**Parameters:**

- `obj` (dict|list): The result object

**Example:**

```python
import scriptling.mcp.tool as tool

tool.return_toon({"result": data})
# Code here will not execute
```

#### mcp.tool.return_error(message)

Return an error message and stop execution with error code 1.

**Parameters:**

- `message` (str): Error message

**Example:**

```python
import scriptling.mcp.tool as tool

if not customer_id:
    tool.return_error("Customer ID is required")
    # Code here will not execute

tool.return_error("Customer not found")
```

### Complete Tool Example

Here's a complete example of an MCP tool using the helpers:

```python
# Recommended: import as alias (cleanest)
import scriptling.mcp.tool as tool

# Get parameters with defaults
name = tool.get_string("name", "guest")
age = tool.get_int("age", 0)
verbose = tool.get_bool("verbose", False)

# Validate inputs
if age < 0:
    tool.return_error("Age must be positive")

# Process and return result
result = {
    "greeting": f"Hello, {name}!",
    "age": age,
    "category": "adult" if age >= 18 else "minor"
}

if verbose:
    import time
    result["timestamp"] = time.time()

tool.return_object(result)
```

### Usage in Tool Scripts

When implementing MCP tools, the execution environment sets `__mcp_params` before running your script:

```python
# Tool script: greet_user.sl
import scriptling.mcp.tool as tool

name = tool.get_string("name", "guest")
age = tool.get_int("age", 0)

tool.return_object({
    "message": f"Hello, {name}! You are {age} years old."
})
```

The tool execution environment will:

1. Set `__mcp_params` dict with tool parameters
2. Run the script
3. Catch the `SystemExit` exception from `return_*` functions
4. Extract the result from `__mcp_response`
5. Return it to the MCP client

### Go Helper Function

For Go applications that need to run MCP tool scripts, the `RunToolScript` helper function simplifies the process:

```go
import (
    "context"
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/extlibs/mcp"
)

// Create scriptling instance and register tool helpers
sl := scriptling.New()
mcp.RegisterToolHelpers(sl)

// Define tool script
script := `
import scriptling.mcp.tool as tool

name = tool.get_string("name", "guest")
age = tool.get_int("age", 0)

tool.return_object({
    "message": f"Hello, {name}! You are {age} years old."
})
`

// Run the tool with parameters
params := map[string]interface{}{
    "name": "Alice",
    "age":  30,
}

response, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
// response: JSON string with the result
// exitCode: 0 for success, 1 for error
// err: Go error if execution failed
```

**Function Signature:**

```go
func RunToolScript(
    ctx context.Context,
    sl *scriptling.Scriptling,
    script string,
    params map[string]interface{}
) (response string, exitCode int, err error)
```

**Parameters:**

- `ctx`: Context for cancellation and timeouts
- `sl`: Scriptling instance (must have tool helpers registered)
- `script`: The tool script code
- `params`: Map of parameter name to value

**Returns:**

- `response`: The tool response from `__mcp_response` (usually JSON)
- `exitCode`: 0 for success, 1 for error
- `err`: Go error if execution failed (nil for exitCode 0)

### Best Practices

1. **Always use defaults**: Provide sensible default values for optional parameters
2. **Validate early**: Check required parameters and return errors immediately
3. **Return quickly**: Use `return_*` functions as soon as you have a result
4. **Handle errors**: Use `return_error()` for user-facing error messages
5. **Choose the right format**: Use `return_string` for text, `return_object` for structured data, `return_toon` for LLM-optimized output
