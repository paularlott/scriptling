# AI Library

AI and LLM functions for interacting with OpenAI-compatible APIs. This library provides two ways to interact with OpenAI:

1. **Wrapped client from Go** - Create a client in Go, wrap it, and pass it to the script
2. **Client instances from scripts** - Create your own client instances from scripts

## Wrapped Client from Go

The recommended pattern for server-side applications is to create the client in Go code, wrap it, and pass it to the script as a global variable:

```go
import "github.com/paularlott/mcp/openai"
import "github.com/paularlott/scriptling/ai"

client, _ := openai.New(openai.Config{
    APIKey: "sk-...",
    BaseURL: "https://api.openai.com/v1",
})

// Wrap and set as global variable
aiClient := ai.WrapClient(client)
p.SetObjectVar("ai_client", aiClient)
```

Then in the script, use the client's instance methods directly:

```python
# Use the client's instance methods
models = ai_client.models()
response = ai_client.chat("gpt-4", {"role": "user", "content": "Hello!"})
```

This pattern allows multiple clients to be used simultaneously and keeps API keys out of scripts.

## Client Instances from Scripts

Create client instances directly from scripts without needing Go code setup.

### ai.new_client(api_key, **kwargs)

Creates a new OpenAI client instance for making API calls.

**Parameters:**
- `api_key` (str): OpenAI API key
- `base_url` (str, optional): Custom base URL (defaults to https://api.openai.com/v1)

**Returns:** OpenAIClient - A client instance with methods for API calls

**Example:**
```python
import ai

# Create a client with default base URL
client = ai.new_client("sk-...")

# Or with a custom base URL (e.g., for LM Studio or compatibility services)
client = ai.new_client("lm-studio", base_url="http://127.0.0.1:1234/v1")
```

## OpenAIClient Class

### client.chat(model, messages...)

Creates a chat completion using this client's configuration.

**Parameters:**
- `model` (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
- `messages` (dict...): One or more message dicts with "role" and "content" keys

**Returns:** dict - Response containing id, choices, usage, etc.

**Example:**
```python
client = ai.new_client("sk-...")
response = client.chat("gpt-4", {"role": "user", "content": "What is 2+2?"})
print(response.choices[0].message.content)
```

### client.models()

Lists all models available for this client configuration.

**Returns:** list - List of model dicts with id, created, owned_by, etc.

**Example:**
```python
client = ai.new_client("sk-...")
models = client.models()
for model in models:
    print(model.id)
```

### client.response_create(input, **kwargs)

Creates a response using the OpenAI Responses API (new structured API).

**Parameters:**
- `input` (list): Input items (messages)
- `model` (str, optional): Model identifier (default: "gpt-4o")

**Returns:** dict - Response object with id, status, output, usage, etc.

**Example:**
```python
client = ai.new_client("sk-...")

# Default model (gpt-4o)
response = client.response_create([
    {"type": "message", "role": "user", "content": "Hello!"}
])
print(response.output)

# Custom model
response = client.response_create([
    {"type": "message", "role": "user", "content": "Hello!"}
], model="gpt-4")
```

### client.response_get(id)

Retrieves a previously created response by its ID.

**Parameters:**
- `id` (str): Response ID

**Returns:** dict - Response object with id, status, output, usage, etc.

**Example:**
```python
client = ai.new_client("sk-...")
response = client.response_get("resp_123")
print(response.status)
```

### client.response_cancel(id)

Cancels a currently in-progress response.

**Parameters:**
- `id` (str): Response ID to cancel

**Returns:** dict - Cancelled response object

**Example:**
```python
client = ai.new_client("sk-...")
response = client.response_cancel("resp_123")
```

### client.add_remote_server(base_url, **kwargs)

Adds a remote MCP server that will be available to all AI calls via this client.

**Parameters:**
- `base_url` (str): URL of the MCP server
- `namespace` (str, optional): Namespace for tools from this server (e.g., "knot")
- `bearer_token` (str, optional): Bearer token for authentication

**Example:**
```python
client = ai.new_client("sk-...")

# Without auth or namespace
client.add_remote_server("https://api.example.com/mcp")

# With namespace only
client.add_remote_server("https://api.example.com/mcp", namespace="knot")

# With bearer token only
client.add_remote_server(
    "https://api.example.com/mcp",
    bearer_token="secret"
)

# With both namespace and bearer token
client.add_remote_server(
    "https://api.example.com/mcp",
    namespace="knot",
    bearer_token="secret"
)

# Now tools from the knot server are available in chat completions
response = client.chat("gpt-4", {"role": "user", "content": "Search for golang news"})
```

### client.remove_remote_server(namespace)

Removes a previously added remote MCP server.

**Parameters:**
- `namespace` (str): Namespace of the server to remove

**Example:**
```python
client = ai.new_client("sk-...")
client.add_remote_server("https://api.example.com/mcp", namespace="knot")
# ... use the server ...
client.remove_remote_server("knot")
```

## Usage Examples

### Basic Chat Completion

```python
import ai

# Using wrapped client from Go
models = ai_client.models()
response = ai_client.chat("gpt-4", {"role": "user", "content": "Hello!"})
print(response.choices[0].message.content)

# Using client instance
client = ai.new_client("sk-...")
response = client.chat("gpt-4", {"role": "user", "content": "Hello!"})
print(response.choices[0].message.content)
```

### Conversation with Multiple Messages

```python
client = ai.new_client("sk-...")

response = client.chat(
    "gpt-4",
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What is the capital of France?"},
    {"role": "assistant", "content": "The capital of France is Paris."},
    {"role": "user", "content": "And what about Germany?"}
)

print(response.choices[0].message.content)
```

### Using Custom Base URL

```python
# For OpenAI-compatible services like LM Studio, local LLMs, etc.
client = ai.new_client(
    "lm-studio",
    base_url="http://127.0.0.1:1234/v1"
)

response = client.chat("mistralai/ministral-3-3b", {"role": "user", "content": "Hello!"})
```

### Using MCP Tools with AI

```python
client = ai.new_client("sk-...")
client.add_remote_server(
    "https://search-api.example.com/mcp",
    namespace="search",
    bearer_token="search-token"
)

# The AI can now use the search tools
response = client.chat("gpt-4", {"role": "user", "content": "Search for recent golang news"})
```

## Error Handling

```python
import ai

try:
    client = ai.new_client("sk-...")
    response = client.chat("gpt-4", {"role": "user", "content": "Hello!"})
    print(response.choices[0].message.content)
except Exception as e:
    print("Error:", e)
```

## Message Format

Messages are dictionaries with the following keys:

- `role` (str): "system", "user", "assistant", or "tool"
- `content` (str): The message content
- `tool_calls` (list, optional): Tool calls made by the assistant
- `tool_call_id` (str, optional): ID for tool response messages

```python
message = {
    "role": "user",
    "content": "What is the weather like?"
}
```
