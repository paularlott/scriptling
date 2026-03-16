import scriptling.mcp.tool as tool
import memory_client

mem = memory_client.open_memory()
result = mem.compact()

tool.return_object({
    "status": "compacted",
    "removed": result["removed"],
    "remaining": result["remaining"]
})
