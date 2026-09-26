# Client for the mcp-decorated example: lists and exercises everything
# tools/shop.py registers through decorators.
#
# Run from the scriptling repo root:
#   scriptling examples/mcp-decorated/client.py
#
# (assumes `scriptling` is on your PATH)

import scriptling.mcp as mcp

client = mcp.Client(
    "scriptling",
    args=["--mcp-tools", "examples/mcp-decorated/tools"],
    namespace="shop",
)

print("Tools:")
for tool in client.tools():
    print("  -", tool.name + ":", tool.description)

print("\nResources:")
for res in client.list_resources():
    print("  -", res.uri, "(" + res.name + ")")
print("Resource templates:")
for tmpl in client.list_resource_templates():
    print("  -", tmpl.uriTemplate)

print("\nPrompts:")
for p in client.list_prompts():
    print("  -", p.name + ":", p.description)

print("\nSkills:")
for skill in client.skills():
    print("  -", skill.uri)

print("\n--- Exercising them all ---")

print("price_lookup(mug) ->", client.call_tool("shop__price_lookup", {"sku": "mug"}))

config = client.read_resource("config://shop")
print("config://shop ->", config.text)

catalog = client.read_resource("shop://catalog/stationery")
print("shop://catalog/stationery ->", catalog.text.split("\n")[0])

prompt = client.get_prompt("describe", {"sku": "mug", "tone": "punchy"})
print("describe(sku=mug, tone=punchy) ->", prompt["messages"][0]["content"])

regions = client.read_resource("skill://shipping_regions/SKILL.md")
print("shipping_regions SKILL.md ->", regions.text.split("\n")[-2].strip())

client.close()
