# Memory MCP Tools

Long-term memory tools for LLM agents via the Scriptling MCP server. Any MCP-compatible client (Claude Desktop, Cursor, Zed, etc.) can use these tools to persist and recall information across sessions.

## Tools

| Tool | Description |
|------|-------------|
| `remember` | Store information with optional type, key, and importance |
| `recall` | Search memories by keyword, matched against both content and keys |
| `forget` | Remove a memory by ID or key |
| `list_memories` | List all memories, optionally filtered by type |

## Storage

The tools read the `SCRIPTLING_MEMORY_DB` environment variable for the storage path, defaulting to `./memory-db` in the current directory. The path is a **directory** (not a file) — snapshotkv creates it automatically if it doesn't exist.

```bash
# Use a custom location
export SCRIPTLING_MEMORY_DB=/home/user/.scriptling/memory
```

## Running the MCP Server

```bash
# Build the CLI first
task build

# Start the MCP server with the memory tools
SCRIPTLING_MEMORY_DB=~/.scriptling/memory ./bin/scriptling --server :8000 --mcp-tools ./examples/mcp-tools/memory-tools

# With bearer token authentication
SCRIPTLING_MEMORY_DB=~/.scriptling/memory ./bin/scriptling --server :8000 --mcp-tools ./examples/mcp-tools/memory-tools --bearer-token your-secret
```

## Client Configuration

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "scriptling-memory": {
      "url": "http://localhost:8000/mcp",
      "headers": {
        "Authorization": "Bearer your-secret"
      }
    }
  }
}
```

**Cursor** (`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "scriptling-memory": {
      "url": "http://localhost:8000/mcp",
      "headers": {
        "Authorization": "Bearer your-secret"
      }
    }
  }
}
```

## Usage Examples

Once connected, the LLM can use the tools naturally:

```
User: My name is Alice, please remember that.
LLM:  [calls remember(content="User's name is Alice", type="fact", key="user_name", importance=0.9)]
      Got it, I'll remember that!

User: What's my name?
LLM:  [calls recall(query="user name")]
      Your name is Alice.

User: I prefer dark mode in all editors.
LLM:  [calls remember(content="User prefers dark mode", type="preference", key="ui_theme", importance=0.7)]
      Noted!

User: Forget my name.
LLM:  [calls forget(id_or_key="user_name")]
      Done, I've forgotten your name.

User: Show me everything you remember.
LLM:  [calls list_memories()]
      Here's what I have stored: ...
```

## Memory Types

| Type | Use for |
|------|---------|
| `fact` | Objective information (names, IDs, limits) |
| `preference` | User preferences (themes, formats, styles) |
| `event` | Things that happened (deployments, meetings) |
| `note` | Agent's own notes (default) |

## Importance

The `importance` field (0.0–1.0) controls compaction behaviour:

- Memories with `importance >= 0.8` are **never** automatically removed
- Lower importance memories are removed after the configured idle timeout
- Default is `0.5`; use `0.9`–`1.0` for critical facts like API keys or names

## Using with the Agent Library

For use inside a Scriptling agentic loop rather than as standalone MCP tools:

### System Prompt

The system prompt is key to making memory work well. The LLM needs to know when to store, recall, and forget. Here's a prompt that works reliably:

```
You are a helpful assistant with long-term memory.

You have access to the following memory tools:
- remember(content, type, key, importance) — store something for later
- recall(query) — search your memory by keyword; matches against both content and keys
- forget(id_or_key) — remove something from memory by ID or key
- list_memories(type, limit) — see everything stored

Guidelines for using memory:
- When the user shares personal information (name, preferences, goals, API keys, project details),
  store it immediately using remember() with an appropriate type and a short descriptive key.
- Use type="preference" for anything about how the user likes things done — editors, formats,
  communication style, language. Recall preferences proactively before making suggestions.
- Before answering questions that might benefit from context ("what do I prefer?", "what's my name?",
  "what are we working on?"), call recall() to check your memory first.
- Use importance=0.9 for critical facts (names, API keys, deadlines) and importance=0.5 for general notes.
- When the user asks you to forget something, call forget() with the key or ID.
- Do not mention the memory tools to the user unless they ask — just use them silently.
```

### Example Script

```python
import scriptling.runtime.kv as kv
import scriptling.ai.memory as memory
import scriptling.ai as ai
import scriptling.ai.agent as agent

SYSTEM_PROMPT = """
You are a helpful assistant with long-term memory.

You have access to the following memory tools:
- remember(content, type, key, importance) — store something for later
- recall(query) — search your memory by keyword; matches against both content and keys
- forget(id_or_key) — remove something from memory by ID or key
- list_memories(type, limit) — see everything stored

Guidelines for using memory:
- When the user shares personal information (name, preferences, goals, API keys, project details),
  store it immediately using remember() with an appropriate type and a short descriptive key.
- Use type="preference" for anything about how the user likes things done — editors, formats,
  communication style, language. Recall preferences proactively before making suggestions.
- Before answering questions that might benefit from context ("what do I prefer?", "what's my name?",
  "what are we working on?"), call recall() to check your memory first.
- Use importance=0.9 for critical facts (names, API keys, deadlines) and importance=0.5 for general notes.
- When the user asks you to forget something, call forget() with the key or ID.
- Do not mention the memory tools to the user unless they ask — just use them silently.
"""

client = ai.Client("http://127.0.0.1:1234/v1")
tools = ai.ToolRegistry()

# Create memory backed by the default kv store
mem = memory.new(kv.default, idle_timeout=24)

# Register memory tools so the LLM can call them
tools.add("remember", "Store information in long-term memory",
    {"content": "string", "type": "string?", "key": "string?", "importance": "float?"},
    lambda args: mem.remember(args["content"],
        type=args.get("type", "note"),
        key=args.get("key", ""),
        importance=float(args.get("importance", 0.5))))

tools.add("recall", "Search long-term memory by keyword, matched against content and keys",
    {"query": "string?"},
    lambda args: mem.recall(args.get("query", "")))

tools.add("forget", "Remove a memory by ID or key",
    {"id_or_key": "string"},
    lambda args: mem.forget(args["id_or_key"]))

bot = agent.Agent(client, tools=tools,
    system_prompt=SYSTEM_PROMPT,
    model="gpt-4")

bot.interact()
```
