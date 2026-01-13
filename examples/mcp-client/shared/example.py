# MCP Client Example - Using Wrapped Client
# The MCP client was wrapped in Go and passed as the mcp_client global variable

print("Using the MCP client from the wrapped global variable...")
print()
print("Fetching available tools...")
tools = mcp_client.tools()
print(f"Found {len(tools)} tools:")
for tool in tools:
    print(f"  - {tool.name}: {tool.description}")

print()
print("Calling execute_code tool to add two numbers...")

result = mcp_client.call_tool("execute_code", {
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

calendar_tools = mcp_client.tool_search("calendar", max_results=10)
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
        result = mcp_client.execute_discovered(tool_name, {"year": 2025, "month": 1})
        print("Calendar result:")
        print(result)
