# Verifies the sales-dashboard-app package end to end: connects to the
# packaged MCP server over stdio and exercises the app tool, the plain tools,
# the UI resource, the prompt (with and without its required argument) and
# the skill.
#
# Build and check the package first, from the scriptling repo root:
#   scriptling pack examples/mcp-app-package sales-app.zip
#   scriptling examples/mcp-app-package/client.py
#
# SALES_APP_PACKAGE overrides the package path (default ./sales-app.zip).

import os
import sys
import scriptling.mcp as mcp

package = os.getenv("SALES_APP_PACKAGE", "sales-app.zip")

# sys.executable: serve the package from this script's own interpreter,
# never depending on which scriptling is on PATH.
client = mcp.Client(sys.executable, args=["--package", package], namespace="app")

print("== tools ==")
app_tool = None
for tool in client.tools():
    print("  -", tool.name + (" [app]" if tool.is_app else ""))
    if tool.name == "app__sales_report":
        app_tool = tool

assert app_tool is not None and app_tool.is_app, "sales_report must be an app tool"
assert any(t.name == "app__sales_total" for t in client.tools()), "sales_total missing"
assert any(t.name == "app__top_product" for t in client.tools()), "top_product missing"

print("\n== app tool and its UI resource ==")
# sales_report seeds the sales records on first call, so run it before the
# plain tools that read them.
report = client.call_tool("app__sales_report", {})
print("sales_report ->", str(report)[:60] + "...")
assert "Widget" in str(report)

view = client.read_resource("ui://sales-dashboard/dashboard.html")
assert "chart" in view.text.lower(), "dashboard.html should reference the chart"
print("ui://sales-dashboard/dashboard.html ->", str(len(view.text)), "bytes of HTML")

print("\n== plain tools ==")
total = client.call_tool("app__sales_total", {})
print("sales_total ->", total)
assert str(total).startswith("total: ")

top = client.call_tool("app__top_product", {"min_amount": 100})
print("top_product(min_amount=100) ->", top)
assert "Gizmo" in str(top), "expected Gizmo (210.00), got: " + str(top)

print("\n== prompt ==")
prompts = client.list_prompts()
for p in prompts:
    arg_names = [a["name"] for a in (p["arguments"] or [])]
    print("  -", p.name, "args:", arg_names)
assert any(p["name"] == "sales_insight" for p in prompts), "sales_insight missing"

prompt = client.get_prompt("sales_insight", {"period": "2026-09", "region": "emea"})
text = prompt["messages"][0]["content"]
print("sales_insight(period=2026-09, region=emea) ->", text)
assert "2026-09" in text and "emea" in text

prompt_default = client.get_prompt("sales_insight", {"period": "2026-09"})
assert "region: all" in prompt_default["messages"][0]["content"], "optional arg must default"
print("sales_insight(period=2026-09) -> optional region defaults to all")

try:
    client.get_prompt("sales_insight", {})
    raise Exception("missing required argument must fail")
except Exception as e:
    assert "missing required argument" in str(e), "expected -32602 missing argument, got: " + str(e)
    print("sales_insight() -> correctly rejected:", str(e)[:50])

print("\n== skill ==")
skills = client.skills()
for skill in skills:
    print("  -", skill.uri)
assert any(s["uri"] == "skill://dashboard-ops/SKILL.md" for s in skills), "dashboard-ops missing"

ops = client.read_resource("skill://dashboard-ops/SKILL.md")
assert "Dashboard Operations" in ops.text
print("SKILL.md ->", ops.text.split("\n")[0].strip())

regions = client.read_resource("skill://dashboard-ops/references/regions.md")
assert "emea" in regions.text
print("references/regions.md -> supporting file readable")

client.close()
print("\nAll package checks passed.")
