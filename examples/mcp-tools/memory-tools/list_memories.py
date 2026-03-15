import scriptling.mcp.tool as tool
import scriptling.runtime.kv as kv
import scriptling.ai.memory as memory
import os

mem_type = tool.get_string("type", "")
limit = tool.get_int("limit", 50)

db = kv.open(os.getenv("SCRIPTLING_MEMORY_DB", "./memory-db"))
mem = memory.new(db, idle_timeout=0)

memories = mem.list(mem_type, limit=limit)
total = mem.count()

mem.close()
db.close()

tool.return_object({"memories": memories, "total": total})
