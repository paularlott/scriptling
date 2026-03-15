import scriptling.mcp.tool as tool
import scriptling.runtime.kv as kv
import scriptling.ai.memory as memory
import os

id_or_key = tool.get_string("id_or_key", "")

if not id_or_key:
    tool.return_error("id_or_key is required")

db = kv.open(os.getenv("SCRIPTLING_MEMORY_DB", "./memory-db"))
mem = memory.new(db, idle_timeout=0)

ok = mem.forget(id_or_key)

mem.close()
db.close()

tool.return_object({"status": "forgotten" if ok else "not_found", "removed": ok})
