import scriptling.mcp.tool as tool
import scriptling.runtime.kv as kv
import scriptling.ai.memory as memory

content = tool.get_string("content")
mem_type = tool.get_string("type", "note")
key = tool.get_string("key", "")
importance = tool.get_float("importance", 0.5)

if not content:
    tool.return_error("content is required")

import os

db = kv.open(os.getenv("SCRIPTLING_MEMORY_DB", "./memory-db"))
mem = memory.new(db, idle_timeout=0)  # compaction managed separately

kwargs = {"type": mem_type, "importance": importance}
if key:
    kwargs["key"] = key

result = mem.remember(content, **kwargs)
mem.close()
db.close()

tool.return_object({
    "status": "remembered",
    "id": result["id"],
    "type": result["type"],
    "key": result["key"],
    "importance": result["importance"]
})
