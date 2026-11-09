# Client for the mcp-tool-hints example: lists the tools with their titles
# and behaviour hints, and decides from the hints which calls to confirm
# with a person before making them.
#
# Run from the scriptling repo root:
#   scriptling examples/mcp-tool-hints/client.py
#
# (launches its own interpreter via sys.executable as a stdio MCP server), or
# against a server already running over HTTP:
#   scriptling --server :8080 --mcp-tools ./examples/mcp-tool-hints/tools
#   scriptling examples/mcp-tool-hints/client.py http://127.0.0.1:8080/mcp

import sys
import scriptling.mcp as mcp

if len(sys.argv) > 1:
    client = mcp.Client(sys.argv[1], namespace="notes")
else:
    # sys.executable: launch this script's own interpreter, not whatever
    # "scriptling" resolves to on PATH.
    client = mcp.Client(
        sys.executable,
        args=["--mcp-tools", "examples/mcp-tool-hints/tools"],
        namespace="notes",
    )


def hint(value):
    if value == None:
        return "-"
    return "yes" if value else "no"


def needs_confirmation(tool):
    # Read-only tools are safe to call. Otherwise ask unless the server says
    # the tool is not destructive: a missing hint means "may be destructive".
    if tool.read_only:
        return False
    return tool.destructive != False


def reaches_outside(tool):
    # A missing open_world hint means "may reach outside systems".
    return tool.open_world != False


print("%-24s %-14s %-9s %-11s %-10s %-10s" % ("tool", "title", "read-only", "destructive", "idempotent", "open world"))
tools = {}
for tool in client.tools():
    tools[tool.name] = tool
    print("%-24s %-14s %-9s %-11s %-10s %-10s" % (
        tool.name, tool.title, hint(tool.read_only), hint(tool.destructive),
        hint(tool.idempotent), hint(tool.open_world)))

print("")
for name in sorted(tools.keys()):
    tool = tools[name]
    notes = []
    if needs_confirmation(tool):
        notes.append("ask the person first")
    else:
        notes.append("safe to call")
    if reaches_outside(tool):
        notes.append("reaches outside systems")
    print(name + ": " + ", ".join(notes))

# A read-only tool can be called straight away.
print("")
print("notes__search_notes ->", client.call_tool("notes__search_notes", {"query": "milk"}))

# What this example expects, so it doubles as a check.
expected = {
    "notes__search_notes": (False, False),
    "notes__add_note": (False, False),
    "notes__share_note": (False, True),
    "notes__delete_note": (True, False),
    "notes__legacy_export": (True, True),
}
assert sorted(tools.keys()) == sorted(expected.keys()), "tools: " + str(sorted(tools.keys()))
for name, want in expected.items():
    got = (needs_confirmation(tools[name]), reaches_outside(tools[name]))
    assert got == want, name + ": got " + str(got) + ", want " + str(want)
assert tools["notes__delete_note"].title == "Delete Note"
assert tools["notes__share_note"].idempotent == True
assert tools["notes__legacy_export"].title == "" and tools["notes__legacy_export"].destructive == None

client.close()
print("")
print("OK")
