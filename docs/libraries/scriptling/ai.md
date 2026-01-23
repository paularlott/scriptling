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
response = ai_client.completion("gpt-4", [{"role": "user", "content": "Hello!"}])
```

This pattern allows multiple clients to be used simultaneously and keeps API keys out of scripts.

## Client Instances from Scripts

Create client instances directly from scripts without needing Go code setup.

### scriptling.ai.new_client(base_url, \*\*kwargs)

Creates a new AI client instance for making API calls to supported services.

**Parameters:**

- `base_url` (str): Base URL of the API (defaults to https://api.openai.com/v1 if empty)
- `service` (str, optional): Service type ("openai" by default)
- `api_key` (str, optional): API key for authentication

**Returns:** AIClient - A client instance with methods for API calls

**Example:**

```python
import scriptling.ai as ai

# OpenAI API (default service)
client = ai.new_client("", api_key="sk-...")

# LM Studio / Local LLM
client = ai.new_client("http://127.0.0.1:1234/v1")

# Explicitly specify service (same as default)
client = ai.new_client("", service="openai", api_key="sk-...")

# Future: Other services
client = ai.new_client("https://api.anthropic.com", service="anthropic", api_key="...")
```

## OpenAIClient Class

### client.completion(model, messages)

Creates a chat completion using this client's configuration.

**Parameters:**

- `model` (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
- `messages` (list): List of message dicts with "role" and "content" keys

**Returns:** dict - Response containing id, choices, usage, etc.

**Example:**

```python
client = ai.new_client("", api_key="sk-...")
response = client.completion("gpt-4", [{"role": "user", "content": "What is 2+2?"}])
print(response.choices[0].message.content)
```

### client.completion_stream(model, messages)

Creates a streaming chat completion using this client's configuration. Returns a ChatStream object that can be iterated over.

**Parameters:**

- `model` (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
- `messages` (list): List of message dicts with "role" and "content" keys

**Returns:** ChatStream - A stream object with a `next()` method

**Example:**

```python
client = ai.new_client("", api_key="sk-...")
stream = client.completion_stream("gpt-4", [{"role": "user", "content": "Count to 10"}])
while True:
    chunk = stream.next()
    if chunk is None:
        break
    if chunk.choices and len(chunk.choices) > 0:
        delta = chunk.choices[0].delta
        if delta.content:
            print(delta.content, end="")
print()
```

### client.embedding(model, input)

Creates an embedding vector for the given input text(s) using the specified model.

**Parameters:**

- `model` (str): Model identifier (e.g., "text-embedding-3-small", "text-embedding-3-large")
- `input` (str or list): Input text(s) to embed - can be a string or list of strings

**Returns:** dict - Response containing data (list of embeddings with index, embedding, object), model, and usage

**Example:**

```python
client = ai.new_client("", api_key="sk-...")

# Single text embedding
response = client.embedding("text-embedding-3-small", "Hello world")
print(response.data[0].embedding)

# Batch embedding
response = client.embedding("text-embedding-3-small", ["Hello", "World"])
for emb in response.data:
    print(emb.embedding)

# Using embeddings for similarity search
texts = ["cat", "dog", "car", "bicycle"]
response = client.embedding("text-embedding-3-small", texts)

# Query similarity
query_resp = client.embedding("text-embedding-3-small", "vehicle")
query_emb = query_resp.data[0].embedding

# Find most similar (simplified - in practice use proper cosine similarity)
import math
for i, text_emb in enumerate(response.data):
    # Simple dot product as example (use cosine similarity in production)
    similarity = sum(a * b for a, b in zip(query_emb, text_emb.embedding))
    print(f"{texts[i]}: {similarity}")
```

## ChatStream Class

### stream.next()

Advances to the next response chunk and returns it.

**Returns:** dict - The next response chunk, or null if the stream is complete

**Example:**

```python
stream = client.completion_stream("gpt-4", [{"role": "user", "content": "Hello!"}])
while True:
    chunk = stream.next()
    if chunk is None:
        break
    if chunk.choices and len(chunk.choices) > 0:
        delta = chunk.choices[0].delta
        if delta.content:
            print(delta.content, end="")
```

### client.models()

Lists all models available for this client configuration.

**Returns:** list - List of model dicts with id, created, owned_by, etc.

**Example:**

```python
client = ai.new_client("", api_key="sk-...")
models = client.models()
for model in models:
    print(model.id)
```

### client.response_create(model, input)

Creates a response using the OpenAI Responses API (new structured API).

**Parameters:**

- `model` (str): Model identifier (e.g., "gpt-4o", "gpt-4")
- `input` (list): Input items (messages)

**Returns:** dict - Response object with id, status, output, usage, etc.

**Example:**

```python
client = ai.new_client("", api_key="sk-...")

response = client.response_create("gpt-4o", [
    {"type": "message", "role": "user", "content": "Hello!"}
])
print(response.output)
```

### client.response_get(id)

Retrieves a previously created response by its ID.

**Parameters:**

- `id` (str): Response ID

**Returns:** dict - Response object with id, status, output, usage, etc.

**Example:**

```python
client = ai.new_client("", api_key="sk-...")
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
client = ai.new_client("", api_key="sk-...")
response = client.response_cancel("resp_123")
```

### client.add_remote_server(base_url, \*\*kwargs)

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

### client.remove_remote_server(prefix)

Removes a previously added remote MCP server.

**Parameters:**

- `prefix` (str): Prefix/namespace of the server to remove

**Example:**

```python
client = ai.new_client("", api_key="sk-...")
client.add_remote_server("https://api.example.com/mcp", namespace="knot")
# ... use the server ...
client.remove_remote_server("knot")
```

### client.set_tools(tools)

Sets custom tools that will be sent to the AI but NOT executed by the client. Tool calls will be returned in the response for manual execution by your script.

This is useful when you want to define custom tools that interact with your local system or application, rather than using MCP servers.

**Parameters:**

- `tools` (list): List of tool dicts with "type", "function" (name, description, parameters)

**Example:**

```python
client = ai.new_client("http://127.0.0.1:1234/v1")

# Define custom tools
tools = [
    {
        "type": "function",
        "function": {
            "name": "read_file",
            "description": "Read a file from the filesystem",
            "parameters": {
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "File path"}
                },
                "required": ["path"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "write_file",
            "description": "Write content to a file",
            "parameters": {
                "type": "object",
                "properties": {
                    "path": {"type": "string"},
                    "content": {"type": "string"}
                },
                "required": ["path", "content"]
            }
        }
    }
]

client.set_tools(tools)

# Now when you call the AI, it can use these tools
response = client.completion("gpt-4", [{"role": "user", "content": "Read config.json"}])

# Check if the AI wants to call a tool
if response.choices[0].message.tool_calls:
    for tool_call in response.choices[0].message.tool_calls:
        tool_name = tool_call.function.name
        tool_args = tool_call.function.arguments
        
        # Execute the tool yourself
        if tool_name == "read_file":
            result = os.read_file(tool_args["path"])
            # Send result back to AI...
```

**See also:** [examples/openai/scriptlingcoder](../../examples/openai/scriptlingcoder/) for a complete example of using custom tools to build an AI coding assistant.

## Usage Examples

### Basic Chat Completion

```python
import scriptling.ai as ai

# Using wrapped client from Go
models = ai_client.models()
response = ai_client.completion("gpt-4", [{"role": "user", "content": "Hello!"}])
print(response.choices[0].message.content)

# Using client instance
client = ai.new_client("", api_key="sk-...")
response = client.completion("gpt-4", [{"role": "user", "content": "Hello!"}])
print(response.choices[0].message.content)
```

### Conversation with Multiple Messages

```python
client = ai.new_client("", api_key="sk-...")

response = client.completion(
    "gpt-4",
    [
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "What is the capital of France?"},
        {"role": "assistant", "content": "The capital of France is Paris."},
        {"role": "user", "content": "And what about Germany?"}
    ]
)

print(response.choices[0].message.content)
```

### Streaming Chat Completion

```python
client = ai.new_client("", api_key="sk-...")

stream = client.completion_stream("gpt-4", [{"role": "user", "content": "Count to 10"}])
while True:
    chunk = stream.next()
    if chunk is None:
        break
    if chunk.choices and len(chunk.choices) > 0:
        delta = chunk.choices[0].delta
        if delta.content:
            print(delta.content, end="")
print()
```

### Using Custom Base URL

```python
# For OpenAI-compatible services like LM Studio, local LLMs, etc.
client = ai.new_client("http://127.0.0.1:1234/v1")

response = client.completion("mistralai/ministral-3-3b", [{"role": "user", "content": "Hello!"}])
```

### Using MCP Tools with AI

```python
client = ai.new_client("", api_key="sk-...")
client.add_remote_server(
    "https://search-api.example.com/mcp",
    namespace="search",
    bearer_token="search-token"
)

# The AI can now use the search tools
response = client.completion("gpt-4", [{"role": "user", "content": "Search for recent golang news"}])
```

## Error Handling

```python
import scriptling.ai as ai

try:
    client = ai.new_client("", api_key="sk-...")
    response = client.completion("gpt-4", [{"role": "user", "content": "Hello!"}])
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
