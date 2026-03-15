# Memory MCP Tools

Long-term memory tools for LLM agents via the Scriptling MCP server. Any MCP-compatible client (Claude Desktop, Cursor, Zed, etc.) can use these tools to persist and recall information across sessions.

## Tools

| Tool | Description |
|------|-------------|
| `remember` | Store information with optional type and importance; returns a UUIDv7 ID |
| `recall` | No arguments: loads full context (all preferences + top memories). With a query: keyword search against content |
| `forget` | Remove a memory by ID |
| `list_memories` | List all memories, optionally filtered by type |

## Storage

The tools read the `SCRIPTLING_MEMORY_DB` environment variable for the storage path, defaulting to `./memory-db` in the current directory. The path is a **directory** (not a file) — snapshotkv creates it automatically if it doesn't exist.

```bash
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

## System Prompt

Add this to your LLM's system prompt to enable memory-aware behaviour:

```
You are a helpful assistant with long-term memory.

At the start of every new conversation, call recall() with no arguments before responding to
the user. This loads all your preferences and recent activity so you have full context before
you begin.

You have access to the following memory tools:
- remember(content, type, importance) — store something for later; returns an id
- recall() — call with no arguments at conversation start to load full context
- recall(query) — search your memory by keyword mid-conversation
- forget(id) — remove something from memory by ID
- list_memories(type, limit) — see everything stored

Guidelines for using memory:
- When the user shares personal information (name, preferences, goals, API keys, project details),
  store it immediately using remember() with an appropriate type.
- Use type="preference" for anything about how the user likes things done — editors, formats,
  communication style, language. Recall preferences proactively before making suggestions.
- Before answering questions that might benefit from context, call recall(query) to check your memory.
- Use importance=0.9 for critical facts (names, API keys, deadlines) and importance=0.5 for general notes.
- When the user asks you to forget something, call forget() with the ID from when it was remembered.
- Do not mention the memory tools to the user unless they ask — just use them silently.
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
