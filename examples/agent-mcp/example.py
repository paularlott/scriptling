# AI Agent using a real MCP server over stdio.
#
# The agent gets:
# - every tool the server exposes, registered under its namespaced name
#   (price_lookup becomes shop__price_lookup), with the server's own schema
# - the server's skills, listed in the system prompt with namespace-qualified
#   skill:// URIs, fetched on demand through the get_skill tool
#
# The MCP server here is another Scriptling process serving the tools and
# skills directories next to this script. Any MCP server works the same way:
# an http(s):// target connects over HTTP, anything else launches as a stdio
# subprocess.
#
# Run from the scriptling repo root with an OpenAI-compatible endpoint:
#   OPENAI_BASE_URL=http://127.0.0.1:11434/v1 OPENAI_MODEL=gemma4:e4b \
#     scriptling examples/agent-mcp/example.py
#
# (assumes `scriptling` is on your PATH)

import os
import scriptling.ai as ai
import scriptling.ai.agent as agent
import scriptling.mcp as mcp

base_url = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:11434/v1")
model = os.getenv("OPENAI_MODEL", "gemma4:e4b")

# A local tool living next to the remote ones.
def local_note(args):
    return "noted: " + args["text"]

tools = ai.ToolRegistry()
tools.add("local_note", "Note something for later in this conversation", {"text": "string"}, local_note)

# The MCP server: launched as a stdio subprocess serving this example's
# tools and skills. The namespace is required: it prefixes the server's tool
# names and routes skill URIs back to this server.
shop = mcp.Client(
    "scriptling",
    args=["--mcp-tools", "examples/agent-mcp/tools", "--mcp-skills", "examples/agent-mcp/skills"],
    namespace="shop",
)

print("Available tools:")
for schema in tools.build():
    print("  -", schema["function"]["name"])

bot = agent.Agent(
    ai.Client(base_url),
    tools=tools,
    mcp_servers=[shop],
    system_prompt="You are a shop assistant.",
    model=model,
)

response = bot.trigger(
    "Look up the price of the product with SKU 'notebook', then follow the shop's product-copy skill to write a description for it.",
    max_iterations=10
)
print()
print(response.content)

# stdio servers are owned by whoever created them: close to shut the subprocess down.
shop.close()
