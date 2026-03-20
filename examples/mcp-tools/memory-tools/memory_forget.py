import scriptling.mcp.tool as tool
import memory_client

id = tool.get_string("id", "")

if not id:
    tool.return_error("id is required")

mem = memory_client.open_memory()
ok = mem.forget(id)

tool.return_object({"status": "forgotten" if ok else "not_found", "removed": ok})
