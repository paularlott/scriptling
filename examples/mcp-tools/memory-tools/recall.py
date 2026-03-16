import scriptling.mcp.tool as tool
import memory_client

query = tool.get_string("query", "")
mem_type = tool.get_string("type", "")
limit = tool.get_int("limit", 10)

mem = memory_client.open_memory()

if query == "" and mem_type == "":
    # Context load: all preferences + top limit non-preferences
    preferences = mem.recall(type="preference", limit=-1)
    others = mem.recall(type="!preference", limit=limit)
    pref_ids = {m["id"] for m in preferences}
    memories = preferences + [m for m in others if m["id"] not in pref_ids]
    result = {"memories": memories, "total_memories": mem.count()}
else:
    result = {"memories": mem.recall(query, limit=limit, type=mem_type)}

tool.return_object(result)
