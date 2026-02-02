# Agent Library

Agentic AI loop for building AI agents with automatic tool execution. The agent handles the complete agentic loop including tool calling, execution, and response formatting.

## Quick Start

```python
import scriptling.ai as ai
import scriptling.agent as agent

# Create AI client
client = ai.new_client("http://127.0.0.1:1234/v1", api_key="sk-...")

# Create tool registry
tools = ai.ToolRegistry()
tools.add("read_file", "Read a file", {"path": "string"}, lambda args: os.read_file(args["path"]))

# Create agent
bot = agent.Agent(client, tools=tools, system_prompt="You are a helpful assistant", model="gpt-4")

# One-shot trigger
response = bot.trigger("What files are in the current directory?", max_iterations=10)
print(response.content)

# Interactive session (requires scriptling.console)
bot.interact()
```

## ToolRegistry Class

### ai.ToolRegistry()

Creates a new tool registry for building OpenAI-compatible tool schemas.

**Example:**

```python
import scriptling.ai as ai

registry = ai.ToolRegistry()
```

### registry.add(name, description, params, handler)

Adds a tool to the registry.

**Parameters:**

- `name` (str): Tool name
- `description` (str): Tool description for the AI
- `params` (dict): Parameter definitions with types
- `handler` (callable): Function to execute when tool is called

**Parameter Types:**

- `string` - String parameter (required)
- `integer` - Integer parameter (required)
- `number` - Number parameter (required, mapped to integer)
- `boolean` - Boolean parameter (required)
- `array` - Array parameter (required)
- `object` - Object parameter (required)
- `string?` - Optional string (add `?` suffix for optional)
- `integer?` - Optional integer
- etc.

**Example:**

```python
tools = ai.ToolRegistry()

# Simple tool
tools.add("get_time", "Get current time", {}, lambda args: "12:00 PM")

# Tool with required parameters
tools.add("read_file", "Read a file", {
    "path": "string"
}, lambda args: os.read_file(args["path"]))

# Tool with optional parameters
tools.add("search", "Search files", {
    "query": "string",
    "limit": "integer?",
    "path": "string?"
}, lambda args: search_files(args["query"], args.get("limit", 10)))
```

### registry.build()

Builds OpenAI-compatible tool schemas.

**Returns:** list - List of tool schema dicts

**Example:**

```python
tools = ai.ToolRegistry()
tools.add("read", "Read file", {"path": "string"}, read_func)

schemas = tools.build()
client.set_tools(schemas)
```

### registry.get_handler(name)

Gets a tool handler by name.

**Parameters:**

- `name` (str): Tool name

**Returns:** callable - Tool handler function

**Example:**

```python
handler = tools.get_handler("read_file")
result = handler({"path": "config.json"})
```

## Agent Class

### agent.Agent(client, tools=None, system_prompt="", model="")

Creates an AI agent with automatic tool execution.

**Parameters:**

- `client` (AIClient): AI client instance (from `ai.new_client()`)
- `tools` (ToolRegistry, optional): Tool registry with available tools
- `system_prompt` (str, optional): System prompt for the agent
- `model` (str, optional): Model to use (if not provided, client must handle model selection)

**Example:**

```python
import scriptling.ai as ai
import scriptling.agent as agent

client = ai.new_client("http://127.0.0.1:1234/v1")
tools = ai.ToolRegistry()
tools.add("read", "Read file", {"path": "string"}, read_func)

bot = agent.Agent(
    client=client,
    tools=tools,
    system_prompt="You are a coding assistant",
    model="gpt-4"
)
```

### agent.trigger(message, max_iterations=1)

Processes a message with the agent, executing tools as needed.

**Parameters:**

- `message` (str or dict): User message to process
- `max_iterations` (int): Maximum tool call rounds (default: 1)

**Returns:** dict - Agent's response message

**Behavior:**

- Strips `<think>...</think>` blocks from responses (for non-interactive use)
- Executes tools automatically
- Maintains conversation history
- Stops after max_iterations or when no more tool calls

**Example:**

```python
# Simple query (no tools)
response = bot.trigger("What is 2+2?")
print(response.content)  # "4"

# Query that uses tools
response = bot.trigger("List all Python files", max_iterations=10)
print(response.content)

# With message dict
response = bot.trigger({
    "role": "user",
    "content": "Read config.json"
}, max_iterations=5)
```

### agent.interact()

Runs an interactive CLI session. Requires `scriptling.console` library.

**Features:**

- Reads user input from stdin
- Displays thinking blocks in purple (from `<think>...</think>` tags)
- Strips thinking markers from final output
- Supports commands:
  - `/q` or `exit` - Quit
  - `/c` - Clear conversation history

**Example:**

```python
bot = agent.Agent(client, tools=tools, system_prompt="Coding assistant")
bot.interact()
```

**Output:**

```
────────────────────────────────────────────────────────────────────────────────
❯ List all Python files
────────────────────────────────────────────────────────────────────────────────

Let me search for Python files...

⏺ Found 3 Python files:
- main.py
- utils.py
- test.py

────────────────────────────────────────────────────────────────────────────────
❯ /q
```

### agent.get_messages()

Gets the current conversation history.

**Returns:** list - List of message dicts

**Example:**

```python
messages = bot.get_messages()
for msg in messages:
    print(f"{msg['role']}: {msg.get('content', '')}")
```

### agent.set_messages(messages)

Sets the conversation history.

**Parameters:**

- `messages` (list): List of message dicts

**Example:**

```python
# Start with existing conversation
bot.set_messages([
    {"role": "system", "content": "You are a helpful assistant"},
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi! How can I help?"}
])

response = bot.trigger("What's the weather?")
```

## Complete Example

```python
#!/usr/bin/env scriptling
import scriptling.ai as ai
import scriptling.agent as agent
import os

# Create AI client
client = ai.new_client("http://127.0.0.1:1234/v1", api_key=os.getenv("OPENAI_API_KEY", ""))

# Create tools
tools = ai.ToolRegistry()

def read_file(args):
    content = os.read_file(args["path"])
    lines = content.split("\n")
    offset = args.get("offset", 0)
    limit = args.get("limit", len(lines))
    return "\n".join(lines[offset:offset+limit])

def write_file(args):
    os.write_file(args["path"], args["content"])
    return "ok"

tools.add("read", "Read file with optional offset/limit", {
    "path": "string",
    "offset": "integer?",
    "limit": "integer?"
}, read_file)

tools.add("write", "Write content to file", {
    "path": "string",
    "content": "string"
}, write_file)

# Create agent
bot = agent.Agent(
    client=client,
    tools=tools,
    system_prompt="Concise coding assistant. cwd: " + os.getcwd(),
    model="gpt-4"
)

# Interactive session
bot.interact()
```

## Tool Handler Interface

Tool handlers receive a dict of arguments and can:

1. **Return values** - Automatically converted to strings for the AI
2. **Return None** - Treated as empty response
3. **Raise exceptions** - Caught and returned as error messages

```python
# Simple handler
def get_time(args):
    import datetime
    return str(datetime.datetime.now())

# Handler with error handling
def read_file_safe(args):
    try:
        return os.read_file(args["path"])
    except Exception as e:
        return f"Error: {e}"

# Handler that returns structured data
def list_files(args):
    files = os.listdir(args.get("path", "."))
    return "\n".join(files)
```

## Thinking Blocks

The agent automatically handles `<think>...</think>` blocks:

- **In `trigger()`**: Strips thinking blocks from responses
- **In `interact()`**: Displays thinking in purple, then strips from final output

This allows models to show their reasoning process in interactive mode while keeping programmatic responses clean.

**Example model output:**

```
<think>
The user wants to list Python files. I should use the glob tool to search for *.py files.
</think>

I'll search for Python files in the current directory.
```

**Interactive display:**

```
The user wants to list Python files. I should use the glob tool to search for *.py files.

⏺ I'll search for Python files in the current directory.
```

**Programmatic response:**

```python
response = bot.trigger("List Python files")
print(response.content)  # "I'll search for Python files in the current directory."
```

## See Also

- [AI Library](ai.md) - AI client and completion functions
- [scriptlingcoder example](../../examples/openai/scriptlingcoder/) - Complete AI coding assistant
