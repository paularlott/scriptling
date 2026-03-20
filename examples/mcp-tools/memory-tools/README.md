# Memory MCP Tools

Long-term memory tools for LLM agents via the Scriptling MCP server. Any MCP-compatible client (Claude Desktop, Cursor, Zed, etc.) can use these tools to persist and recall information across sessions.

## Tools

| Tool               | Description                                                                                         |
| ------------------ | --------------------------------------------------------------------------------------------------- |
| `memory_remember`  | Store information with optional type and importance; returns a UUIDv7 ID                            |
| `memory_recall`    | Hybrid keyword + semantic search. No arguments: loads full context (all preferences + top memories) |
| `memory_forget`    | Remove a memory by ID                                                                               |
| `memory_compact`   | Manually trigger compaction; returns remaining count                                                |

## Storage

The tools read the `SCRIPTLING_MEMORY_DB` environment variable for the storage path, defaulting to `./memory-db` in the current directory. The path is a **directory** (not a file) — snapshotkv creates it automatically if it doesn't exist.

```bash
export SCRIPTLING_MEMORY_DB=/home/user/.scriptling/memory
```

## AI Provider (LLM Deduplication)

Set these environment variables to enable LLM-based deduplication. When similar memories are found during `remember()` or `compact()`, the LLM decides whether to merge them or keep them separate. Both `SCRIPTLING_AI_BASE_URL` and `SCRIPTLING_AI_MODEL` must be set.

| Variable                 | Description                                                                                 |
| ------------------------ | ------------------------------------------------------------------------------------------- |
| `SCRIPTLING_AI_BASE_URL` | Base URL of the AI provider (e.g. `http://127.0.0.1:1234/v1`)                               |
| `SCRIPTLING_AI_PROVIDER` | Provider type: `openai`, `claude`, `gemini`, `ollama`, `zai`, `mistral` (default: `openai`) |
| `SCRIPTLING_AI_MODEL`    | Model name (e.g. `qwen3-8b`, `gpt-4o-mini`)                                                 |
| `SCRIPTLING_AI_TOKEN`    | API key / bearer token (optional for local providers)                                       |

```bash
export SCRIPTLING_AI_BASE_URL=http://127.0.0.1:1234/v1
export SCRIPTLING_AI_MODEL=qwen3-8b
# export SCRIPTLING_AI_TOKEN=sk-...  # required for hosted providers
```

## Running the MCP Server

```bash
# Build the CLI first
task build

# Basic (rule-based deduplication only)
SCRIPTLING_MEMORY_DB=~/.scriptling/memory \
  ./bin/scriptling --server :8000 --mcp-tools ./examples/mcp-tools/memory-tools

# With LLM deduplication (local provider)
SCRIPTLING_MEMORY_DB=~/.scriptling/memory \
SCRIPTLING_AI_BASE_URL=http://127.0.0.1:1234/v1 \
SCRIPTLING_AI_MODEL=qwen3-8b \
  ./bin/scriptling --server :8000 --mcp-tools ./examples/mcp-tools/memory-tools

# With bearer token authentication
SCRIPTLING_MEMORY_DB=~/.scriptling/memory \
  ./bin/scriptling --server :8000 --mcp-tools ./examples/mcp-tools/memory-tools --bearer-token your-secret
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

## Using with the Scriptling Agent

When using the `scriptling.ai.agent` library, pass a memory object directly to the `Agent` constructor. Memory tools are wired automatically and the system prompt is augmented with memory instructions and any stored preferences:

```python
import scriptling.ai as ai
import scriptling.ai.agent as agent
import scriptling.ai.memory as memory
import scriptling.runtime.kv as kv

mem = memory.new(kv.open("./memory-db"))

bot = agent.Agent(
    ai.Client("http://127.0.0.1:1234/v1"),
    model="qwen3-8b",
    system_prompt="You are a helpful assistant.",
    memory=mem
)

response = bot.trigger("What do you know about me?")
print(response.content)
```

The agent automatically:

- Registers `memory_remember`, `memory_recall`, and `memory_forget` as tools
- Appends memory usage instructions to the system prompt
- Pre-loads all stored `preference` memories into the system prompt for immediate context without needing a tool call

### With LLM Deduplication

To enable intelligent deduplication when similar memories are found:

```python
import scriptling.ai as ai
import scriptling.ai.memory as memory
import scriptling.runtime.kv as kv

client = ai.Client("http://127.0.0.1:1234/v1")
mem = memory.new(kv.open("./memory-db"), ai_client=client, model="qwen3-8b")

bot = agent.Agent(client, model="qwen3-8b", memory=mem)
```

## System Prompt

Add this to your LLM's system prompt to enable memory-aware behaviour:

```
You are a helpful assistant with long-term memory.

At the start of every new conversation, call memory_recall() with no arguments to load your stored
preferences and recent context before responding.

You have access to the following memory tools:
- memory_remember(content, type, importance) — store something for later; returns an id
- memory_recall() — call with no arguments at conversation start to load full context
- memory_recall(query) — search your memory by keyword and semantic similarity mid-conversation
- memory_forget(id) — remove something from memory by ID

Guidelines for using memory:
- Store one fact per memory — do not combine multiple subjects into a single remember() call.
- Keep memory content concise: a single clear sentence, no padding or filler.
- Be proactive: if information comes up in conversation that could be useful in a future session, store it without waiting to be asked. When in doubt, store it.
- Store technical details, product names, configurations, project context, decisions made, and anything the user might ask about again later.
- When the user shares personal information (name, preferences, goals, API keys, project details),
  store it immediately using remember() with an appropriate type.
- Use type="preference" for anything about how the user likes things done — editors, formats,
  communication style, language. Recall preferences proactively before making suggestions.
- Before answering questions that might benefit from context, call memory_recall(query) to check your memory.
- Use importance=0.9 for critical facts (names, API keys, deadlines) and importance=0.5 for general notes.
- When the user asks you to forget something, call memory_forget() with the ID from when it was remembered.
- Do not mention the memory tools to the user unless they ask — just use them silently.
```

## Memory Types

| Type         | Use for                                                                                  |
| ------------ | ---------------------------------------------------------------------------------------- |
| `fact`       | Objective information (names, IDs, limits)                                               |
| `preference` | User preferences (themes, formats, styles) — pre-loaded into system prompt, never decays |
| `event`      | Things that happened (deployments, meetings)                                             |
| `note`       | Agent's own notes (default)                                                              |

## Importance and Compaction

The `importance` field (0.0–1.0) controls how long a memory survives compaction:

| Importance    | Behaviour                                                 |
| ------------- | --------------------------------------------------------- |
| 0.9–1.0       | Survives for the full 180-day hard cap                    |
| 0.5 (default) | ~1 year effective lifetime for facts, ~3 months for notes |
| 0.1           | Pruned quickly — a note at 0.1 is gone in ~7 days         |

**Compaction is manual** — call `memory_compact()` to prune old/decayed memories. `preference` type memories never decay regardless of importance — they are only removed after the 180-day hard age cap (based on last access).

## Deduplication

When storing a memory, the system checks for similar existing memories using MinHash similarity:

- **Similarity ≥ 85%**: Auto-merges into the existing memory
- **Similarity 50–85%** (with AI client): LLM decides whether to merge or keep separate
- **Similarity < 50%**: Creates new memory

This prevents duplicate memories from accumulating while preserving genuinely distinct information.
