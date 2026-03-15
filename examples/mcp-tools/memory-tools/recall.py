import scriptling.mcp.tool as tool
import scriptling.runtime.kv as kv
import scriptling.ai.memory as memory
import os

query = tool.get_string("query", "")
mem_type = tool.get_string("type", "")
limit = tool.get_int("limit", 10)

db = kv.open(os.getenv("SCRIPTLING_MEMORY_DB", "./memory-db"))
mem = memory.new(db, idle_timeout=0)

if query == "" and mem_type == "":
    # Context load: all preferences + top memories by recency/importance
    preferences = mem.list(type="preference", limit=100)
    top = mem.recall("", limit=20)
    pref_ids = [m["id"] for m in preferences]
    memories = preferences + [m for m in top if m["id"] not in pref_ids]
    result = {"memories": memories, "total_memories": mem.count()}
else:
    result = {"memories": mem.recall(query, limit=limit, type=mem_type)}

mem.close()
db.close()

tool.return_object(result)
