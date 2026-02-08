# AI Library

AI and LLM functions for interacting with OpenAI-compatible APIs. This library provides:

1. **AI Client** - Create clients and make completions
2. **Tool Registry** - Build tool schemas for AI agents
3. **Thinking Extractor** - Extract reasoning blocks from AI responses
4. **Wrapped client from Go** - Pass Go-created clients to scripts

## Thinking Extractor

### ai.extract_thinking(text)

Extracts thinking/reasoning blocks from AI model responses. Many models include their reasoning in special blocks (like `<think>...</think>`) which you may want to process separately from the main response.

**Supported Formats:**

- XML-style: `<think>...</think>`, `<thinking>...</thinking>`
- OpenAI style: `<Thought>...</Thought>`
- Markdown blocks: ` ```thinking\n...\n``` `, ` ```thought\n...\n``` `
- Claude style: `<antThinking>...</antThinking>`

**Parameters:**

- `text` (str): The AI response text to process

**Returns:** dict - Contains:

- `thinking` (list): List of extracted thinking block strings
- `content` (str): The cleaned response text with thinking blocks removed

**Example:**

```python
import scriptling.ai as ai

response_text = """<think>
Let me analyze this step by step.
The user wants to know about Python.
</think>

Python is a high-level programming language known for its readability."""

result = ai.extract_thinking(response_text)

# Access the thinking blocks
for thought in result["thinking"]:
    print("Model reasoning:", thought)

# Get the cleaned response
print("Response:", result["content"])
# Output: "Python is a high-level programming language known for its readability."
```

**With Agent Responses:**

```python
import scriptling.ai as ai
import scriptling.ai.agent as agent

bot = agent.Agent(client, tools=tools, system_prompt="...")
response = bot.trigger("Explain Python")

# Extract and display thinking separately
result = ai.extract_thinking(response.content)

if result["thinking"]:
    print("=== Model Reasoning ===")
    for thought in result["thinking"]:
        print(thought)
    print()

print("=== Response ===")
print(result["content"])
```

## Tool Registry

Build OpenAI-compatible tool schemas for AI agents. See [Agent Library](agent.md) for complete agent examples.

### ai.ToolRegistry()

Creates a new tool registry.

**Example:**

```python
import scriptling.ai as ai

tools = ai.ToolRegistry()
tools.add("read_file", "Read a file", {"path": "string"}, lambda args: os.read_file(args["path"]))
schemas = tools.build()
client.set_tools(schemas)
```

See [Agent Library](agent.md) for detailed ToolRegistry documentation.

## AI Client

### Wrapped Client from Go

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
- `remote_servers` (list, optional): List of remote MCP server configs, each a dict with:
  - `base_url` (str, required): URL of the MCP server
  - `namespace` (str, optional): Namespace prefix for tools from this server
  - `bearer_token` (str, optional): Bearer token for authentication

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

# With MCP servers configured
client = ai.new_client("http://127.0.0.1:1234/v1", remote_servers=[
    {"base_url": "http://127.0.0.1:8080/mcp", "namespace": "scriptling"},
    {"base_url": "https://api.example.com/mcp", "namespace": "search", "bearer_token": "secret"},
])

# Future: Other services
client = ai.new_client("https://api.anthropic.com", service="anthropic", api_key="...")
```

## AIClient Class

All client methods are instance methods on the client object returned by ai.new_client() or ai.WrapClient().

### client.completion(model, messages, **kwargs)

Creates a chat completion using this client's configuration.

**Parameters:**

- `model` (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
- `messages` (list): List of message dicts with "role" and "content" keys
- `tools` (list, optional): List of tool schema dicts from ToolRegistry.build()
- `temperature` (float, optional): Sampling temperature (0.0-2.0)
- `max_tokens` (int, optional): Maximum tokens to generate

**Returns:** dict - Response containing id, choices, usage, etc.

**Example:**

```python
client = ai.new_client("", api_key="sk-...")
response = client.completion("gpt-4", [{"role": "user", "content": "What is 2+2?"}])
print(response.choices[0].message.content)
```

**With Tool Calling:**

```python
import scriptling.ai as ai

# Create tools registry
tools = ai.ToolRegistry()
tools.add("get_time", "Get current time", {}, lambda args: "12:00 PM")
tools.add("read_file", "Read a file", {"path": "string"}, lambda args: os.read_file(args["path"]))

# Build schemas and pass to completion
schemas = tools.build()
response = client.completion("gpt-4", [{"role": "user", "content": "What time is it?"}], tools=schemas)
```

### client.completion_stream(model, messages, **kwargs)

Creates a streaming chat completion using this client's configuration. Returns a ChatStream object that can be iterated over.

**Parameters:**

- `model` (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
- `messages` (list): List of message dicts with "role" and "content" keys
- `tools` (list, optional): List of tool schema dicts from ToolRegistry.build()
- `temperature` (float, optional): Sampling temperature (0.0-2.0)
- `max_tokens` (int, optional): Maximum tokens to generate

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

**With Tool Calling:**

```python
tools = ai.ToolRegistry()
tools.add("get_weather", "Get weather for a city", {"city": "string"}, weather_handler)
schemas = tools.build()

stream = client.completion_stream("gpt-4", [{"role": "user", "content": "What's the weather in Paris?"}], tools=schemas)
# Stream chunks...
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

MCP servers should be configured during client creation. See the Go documentation for how to wrap clients with MCP servers before passing them to scripts.

```python
# The client provided by the Go code already has MCP servers configured
# Tools from those servers are automatically available to AI calls
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
