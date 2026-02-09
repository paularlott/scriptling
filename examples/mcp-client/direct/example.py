# Direct MCP Client Example
# Creates a client to connect to the scriptling MCP server

import scriptling.mcp as mcp

print("Creating MCP client for localhost:8080...")
client = mcp.Client("http://127.0.0.1:8080/mcp")

print()
print("Fetching available tools...")
tools = client.tools()
print(f"Found {len(tools)} tools:")
for tool in tools:
    print(f"  - {tool.name}: {tool.description}")

print()
print("Calling execute_code tool to add two numbers...")
print()

result = client.call_tool("execute_code", {
    "code": """
# Simple scriptling code to add two numbers
a = 15
b = 27
print(f"{a} + {b} = {a + b}")
"""
})

print("Result:")
print(result)

print()
print("Searching for calendar-related tools...")
print()

calendar_tools = client.tool_search("calendar", max_results=10)
print(f"Found {len(calendar_tools)} calendar tools:")
for tool in calendar_tools:
    name = tool.get("name", "unknown")
    desc = tool.get("description", "")
    print(f"  - {name}: {desc}")

if len(calendar_tools) > 0:
    print()
    print("Executing the first calendar tool...")
    # Execute a discovered tool using execute_discovered
    tool_name = calendar_tools[0].get("name")
    if tool_name:
        # Try to generate calendar for current month
        result = client.execute_discovered(tool_name, {"year": 2025, "month": 1})
        print("Calendar result:")
        print(result)
