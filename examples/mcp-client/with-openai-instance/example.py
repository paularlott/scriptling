# MCP + OpenAI Integration Example
# Creates both clients in the script, with MCP servers attached to the AI client

import scriptling.ai as ai
import scriptling.mcp as mcp

print("Creating OpenAI client for LM Studio...")
ai_client = ai.Client("http://127.0.0.1:1234/v1", remote_servers=[
    {"base_url": "http://127.0.0.1:8080/mcp", "namespace": "scriptling"},
])

print()
print("Creating MCP client for scriptling MCP server (for direct tool access)...")
mcp_client = mcp.Client("http://127.0.0.1:8080/mcp", namespace="scriptling")

print()
print("Fetching available tools from MCP server...")
tools = mcp_client.tools()
print(f"Found {len(tools)} tools:")
for tool in tools:
    print(f"  - {tool.name}: {tool.description}")

print()
print("Now asking the AI to use MCP tools...")
print()

# The AI can now use tools from the scriptling MCP server
response = ai_client.completion(
    "mistralai/ministral-3-3b",
    [
        {"role": "system", "content": "You have access to a scriptling MCP server. Use the execute_code tool to calculate 15 + 27."},
        {"role": "user", "content": "What is 15 plus 27?"}
    ]
)

print("Response:")
print(response.choices[0].message.content)
