import scriptling.mcp.tool as tool
import scriptling.runtime.kv as kv
import scriptling.ai.memory as memory
import os

id = tool.get_string("id", "")

if not id:
    tool.return_error("id is required")

db = kv.open(os.getenv("SCRIPTLING_MEMORY_DB", "./memory-db"))
mem = memory.new(db, idle_timeout=0)

ok = mem.forget(id)

mem.close()
db.close()

tool.return_object({"status": "forgotten" if ok else "not_found", "removed": ok})
