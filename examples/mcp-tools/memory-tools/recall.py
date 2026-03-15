import scriptling.mcp.tool as tool
import scriptling.runtime.kv as kv
import scriptling.ai.memory as memory
import os

query = tool.get_string("query", "")
mem_type = tool.get_string("type", "")
limit = tool.get_int("limit", 10)

db = kv.open(os.getenv("SCRIPTLING_MEMORY_DB", "./memory-db"))
mem = memory.new(db, idle_timeout=0)

memories = mem.recall(query, limit=limit, type=mem_type)

mem.close()
db.close()

tool.return_object({"memories": memories})
