import scriptling.mcp.tool as tool
import memory_client

query = tool.get_string("query", "")
mem_type = tool.get_string("type", "")
limit = tool.get_int("limit", 10)

mem = memory_client.open_memory()

if query == "" and mem_type == "":
    # Context load: all preferences + top memories by recency/importance
    preferences = mem.list(type="preference", limit=100)
    top = mem.recall("", limit=20)
    pref_ids = [m["id"] for m in preferences]
    memories = preferences + [m for m in top if m["id"] not in pref_ids]
    result = {"memories": memories, "total_memories": mem.count()}
else:
    result = {"memories": mem.recall(query, limit=limit, type=mem_type)}

tool.return_object(result)
