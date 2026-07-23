import scriptling.runtime.mcp as mcp
import scriptling.ai as ai
import scriptling.ai.memory as memory
import scriptling.runtime.kv as kv
import os


def _open_memory():
    """Open the memory store, configured from environment variables."""
    ai_base_url = os.getenv("SCRIPTLING_AI_BASE_URL", "")
    ai_model = os.getenv("SCRIPTLING_AI_MODEL", "")
    ai_token = os.getenv("SCRIPTLING_AI_TOKEN", "")
    ai_provider = os.getenv("SCRIPTLING_AI_PROVIDER", "openai")

    client = ai.Client(ai_base_url, api_key=ai_token, provider=ai_provider) if ai_base_url and ai_model else None

    db = kv.open(os.getenv("SCRIPTLING_MEMORY_DB", "./memory-db"))
    return memory.new(db, ai_client=client, model=ai_model)


@mcp.tool(
    "Store information in long-term memory for future reference",
    params={
        "content": "The information to remember",
        "type": "Category: 'fact', 'preference', 'event', or 'note' (default: 'note')",
        "importance": {"type": "number", "description": "Importance 0.0-1.0; higher values survive compaction longer (default: 0.5). Use 0.9 for critical facts, 0.5 for general notes."},
    },
    keywords=["memory", "remember", "store", "save", "persist"],
)
def memory_remember(content, type="note", importance=0.5):
    if not content:
        raise ValueError("content is required")
    mem = _open_memory()
    result = mem.remember(content, type=type, importance=importance)
    return {"status": "remembered", "id": result["id"], "type": result["type"], "importance": result["importance"]}


@mcp.tool(
    "Search long-term memory for relevant context. Always pass a query matching the topic being discussed. Call with no arguments at conversation start to load all preferences + top 10 recent memories.",
    params={
        "query": "Keyword search against memory content (e.g. 'employer', 'dark mode', 'api key'). Omit only to load full context at conversation start.",
        "type": "Filter by type: 'fact', 'preference', 'event', 'note', or '!type' to exclude",
        "limit": {"type": "integer", "description": "Maximum number of memories to return for keyword searches (default: 10)"},
    },
    keywords=["memory", "recall", "search", "find", "retrieve", "remember", "context", "load"],
)
def memory_recall(query="", type="", limit=10):
    mem = _open_memory()
    memories = mem.recall(query, limit=limit, type=type)
    return {"memories": memories, "total_memories": mem.count()}


@mcp.tool(
    "Remove information from long-term memory",
    params={"id": "Memory ID to forget (returned by remember)"},
    keywords=["memory", "forget", "delete", "remove"],
)
def memory_forget(id):
    if not id:
        raise ValueError("id is required")
    mem = _open_memory()
    ok = mem.forget(id)
    return {"status": "forgotten" if ok else "not_found", "removed": ok}


@mcp.tool(
    "Manually trigger memory compaction to prune decayed and expired memories",
    keywords=["memory", "compact", "prune", "cleanup", "maintenance"],
)
def memory_compact():
    mem = _open_memory()
    result = mem.compact()
    return {"status": "compacted", "removed": result["removed"], "remaining": result["remaining"]}
