# Tools declared with @mcp.tool, each with a title and the behaviour hints
# that describe what it does. MCP clients read these from tools/list to
# decide, for example, which calls to confirm with a person first.
#
#   read_only    the tool changes nothing
#   destructive  the tool may delete or overwrite, not only add
#   idempotent   calling it again with the same arguments has no further effect
#   open_world   the tool reaches outside systems (the web, other services)
#
# Leave a hint out (or pass None) when you cannot promise it: clients then
# assume destructive and open_world, and not read_only or idempotent.

import scriptling.runtime.mcp as mcp


@mcp.tool("Search the notes for a word", title="Search Notes", read_only=True, open_world=False)
def search_notes(query):
    return "2 notes mention " + query


@mcp.tool("Add a note", title="Add Note", destructive=False, open_world=False)
def add_note(text):
    return "added: " + text


@mcp.tool("Delete a note by id", title="Delete Note", destructive=True, idempotent=True, open_world=False)
def delete_note(id):
    return "deleted " + id


@mcp.tool("Export every note in an old format")
def legacy_export():
    # No hints: clients must assume the worst (destructive, open world).
    return "exported"
