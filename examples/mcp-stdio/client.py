# Consuming a stdio MCP server from Scriptling.
#
# mcp.Client() picks the transport from its first argument: an http(s):// URL
# connects over HTTP, anything else is launched as a stdio server subprocess.
#
# Here we launch another Scriptling process as a stdio MCP server (its exec tool
# is enabled with --mcp-exec-script; without --server it serves over stdio) and
# call a tool on it.
#
# Run with:
#   scriptling examples/mcp-stdio/client.py
#
# (assumes `scriptling` is on your PATH)

import scriptling.mcp as mcp

# Launch the stdio server subprocess. Tool names are prefixed with the namespace.
client = mcp.Client("scriptling", args=["--mcp-exec-script"], namespace="local")

print("Tools:")
for tool in client.tools():
    print("  -", tool.name)

result = client.call_tool("local__execute_script", {"code": "print(6 * 7)"})
print("execute_script(print(6*7)) ->", result)

# Always close a stdio client to shut the subprocess down.
client.close()
