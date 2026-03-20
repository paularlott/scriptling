import scriptling.mcp.tool as tool
import memory_client

query = tool.get_string("query", "")
mem_type = tool.get_string("type", "")
limit = tool.get_int("limit", 10)

mem = memory_client.open_memory()

memories = mem.recall(query, limit=limit, type=mem_type)
result = {"memories": memories, "total_memories": mem.count()}

tool.return_object(result)
