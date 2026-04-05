# OpenAI AI Examples

This directory contains examples demonstrating how to use the AI library with OpenAI-compatible APIs (including LM Studio).

## Prerequisites

1. **Install LM Studio** - Download from [lmstudio.ai](https://lmstudio.ai/)
2. **Start LM Studio server**:
   - Open LM Studio
   - Go to the "Local Server" tab
   - Start the server on `127.0.0.1:1234`
3. **Load a model** (e.g., `mistralai/ministral-3-3b`)

## Examples

### shared/ - Using a Wrapped Client

This example demonstrates the wrapped client pattern where the OpenAI client is configured in Go code, wrapped as a scriptling object, and passed to the script as a global variable.

```bash
cd shared
go run main.go
```

**How it works:**

- Go code creates an `openai.Client` configured for the OpenAI API
- Client is wrapped via `ai.WrapClient()` and set as a global variable via `p.SetObjectVar()`
- Script uses instance methods like `ai_client.models()` and `ai_client.chat()` directly
- This pattern allows multiple clients to be used simultaneously

**Use this pattern when:**

- You want to manage the client configuration in Go
- Multiple scripts need to share the same client
- You want to keep API keys out of scripts
- You need to support multiple different clients simultaneously

### instance/ - Creating Client from Script

This example demonstrates creating a client instance directly from the script without any pre-configuration in Go.

```bash
cd instance
go run main.go
```

**How it works:**

- No client is configured in Go
- Script creates its own client via `ai.Client()`
- Script uses instance methods like `client.models()` and `client.completion()`
- The `example.py` script handles all connection details

**Use this pattern when:**

- You want scripts to be self-contained
- Each script needs different client configurations
- You're writing scripts that can run standalone

### streaming/ - Streaming Chat Completions

This example demonstrates streaming responses from an OpenAI-compatible API, which is ideal for interactive applications and long-form content generation.

```bash
cd streaming
go run main.go
```

**How it works:**

- Script creates a client via `ai.Client()`
- Uses `client.completion_stream()` to get streaming responses
- Iterates through chunks with `stream.next()`
- Prints content in real-time as it arrives

**Use this pattern when:**

- You want real-time response streaming
- Building interactive chat applications
- Generating long-form content with progressive display
- Providing immediate feedback to users

### tools/ - Local Tool Calling Examples

These examples expose a local tool to the model, execute it from the script, and then send the tool result back for a final answer.

```bash
../../../bin/scriptling tools/example.py
../../../bin/scriptling tools/streaming.py
```

**How it works:**

- Registers a local `echo_tool` via `ai.ToolRegistry()`
- Sends the tool schema to the model with `tools=schemas`
- Detects tool calls in the assistant response
- Executes the tool locally and prints when it was invoked
- Sends the tool result back to the model for the final response

**Use this pattern when:**

- You want a minimal proof that tool calling works end to end
- You need to debug model-side tool call behavior
- You want both non-streaming and streaming tool-call examples

### scriptlingcoder/ - AI Coding Assistant with Custom Tools

An interactive AI coding assistant that can read, write, and modify files using custom tools. Inspired by [nanocode](https://github.com/1rgs/nanocode).

```bash
cd scriptlingcoder
../../../bin/scriptling scriptlingcoder.py
```

**⚠️ WARNING**: This example executes AI-generated code and shell commands. Use at your own risk!

**How it works:**

- Defines custom tools (read, write, edit, glob, grep, bash)
- Registers tools via `tools` parameter in `completion()` - tools are sent to AI but NOT executed by client
- AI can call tools, script executes them locally and returns results
- Supports multi-turn conversations with tool execution

**Use this pattern when:**

- You need custom tools that aren't MCP servers
- You want full control over tool execution
- Building AI agents that interact with local systems
- Creating specialized coding assistants

**Features:**
- File operations (read, write, edit)
- Search (glob patterns, regex grep)
- Shell command execution
- Interactive chat interface
- Conversation history

## Scripts

### shared/example.py

Uses the wrapped client passed from Go:

```python
print("Using the AI client from the wrapped global variable...")
print()

print("Fetching available models from LM Studio...")
models_response = ai_client.models()
models = models_response.data
print(f"Found {len(models)} models:")

print()
print("Running chat completion...")
response = ai_client.completion(
    "mistralai/ministral-3-3b",
    [{"role": "user", "content": "What is 2 + 2?"}]
)
```

### instance/example.py

Creates its own client instance:

```python
import scriptling.ai as ai

print("Creating OpenAI client for LM Studio...")
client = ai.Client("http://127.0.0.1:1234/v1")

print()
print("Fetching available models...")
models_response = client.models()
models = models_response.data

print()
print("Running chat completion...")
response = client.completion(
    "gemma4:e4b",
    [{"role": "user", "content": "What is 2 + 2?"}]
)
```

### streaming/example.py

Demonstrates streaming responses:

```python
import scriptling.ai as ai

client = ai.Client("http://127.0.0.1:1234/v1")

# Create a streaming completion
stream = client.completion_stream(
    "gemma4:e4b",
    [{"role": "user", "content": "Write a short haiku about coding in Python."}]
)

# Stream the response chunks
while True:
    chunk = stream.next()
    if chunk is None:
        break

    if chunk.choices and len(chunk.choices) > 0:
        delta = chunk.choices[0].delta
        if delta and delta.content:
            print(delta.content, end='', flush=True)
```

### tools/example.py

Demonstrates a complete non-streaming tool call:

```python
import scriptling.ai as ai

client = ai.Client("http://127.0.0.1:11434/v1")

tools = ai.ToolRegistry()
tools.add("echo_tool", "Echo a message back to the assistant", {"message": "string"}, echo_tool)
schemas = tools.build()

response = client.completion(
    "gemma4:e4b",
    [{"role": "user", "content": "Call the echo_tool with message 'hello from tool test'."}],
    tools=schemas
)
```

### tools/streaming.py

Demonstrates streaming a tool-enabled turn:

```python
stream = client.completion_stream(
    "gemma4:e4b",
    [{"role": "user", "content": "Call the echo_tool with message 'hello from streaming tool test'."}],
    tools=schemas
)
```

## Expected Output

Both examples will produce similar output:

```
Fetching available models from LM Studio...
Found 1 models:
  - mistralai/ministral-3b

Running chat completion with mistralai/ministral-3-3b...

Response:
4
```

## Troubleshooting

**Connection refused**: Make sure LM Studio server is running on port 1234

**Model not found**: Make sure the model is loaded in LM Studio

**Empty response**: Try a different model or adjust the prompt
