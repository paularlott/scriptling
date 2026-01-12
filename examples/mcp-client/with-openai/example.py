# MCP + OpenAI Integration Example
# Both clients are passed from Go, with MCP tools attached to the AI client

print("Fetching available tools from MCP server...")
tools = mcp_client.tools()
print(f"Found {len(tools)} tools:")
for tool in tools:
    print(f"  - {tool.name}: {tool.description}")

print()
print("Now asking the AI to use MCP tools...")
print()

# The AI can now use tools from the scriptling MCP server
response = ai_client.chat(
    "mistralai/ministral-3-3b",
    {"role": "system", "content": "You have access to a scriptling MCP server. Use the execute_code tool to calculate 15 + 27."},
    {"role": "user", "content": "What is 15 plus 27?"}
)

print("Response:")
print(response.choices[0].message.content)
