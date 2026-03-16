import scriptling.mcp.tool as tool
import memory_client

mem_type = tool.get_string("type", "")
limit = tool.get_int("limit", 50)

mem = memory_client.open_memory()
memories = mem.list(mem_type, limit=limit)

tool.return_object({"memories": memories, "total": mem.count()})
