import scriptling.mcp.tool as tool
import memory_client

content = tool.get_string("content")
mem_type = tool.get_string("type", "note")
importance = tool.get_float("importance", 0.5)

if not content:
    tool.return_error("content is required")

mem = memory_client.open_memory()
result = mem.remember(content, type=mem_type, importance=importance)

tool.return_object({
    "status": "remembered",
    "id": result["id"],
    "type": result["type"],
    "importance": result["importance"]
})
