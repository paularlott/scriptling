# Tool Hints Example

MCP tools can carry a display title and behaviour hints that tell a client
what a tool does before it calls it. This example declares them on the server
and reads them back in a client.

[tools/notes.py](tools/notes.py) declares tools with `@mcp.tool`, and
[tools/share_note.toml](tools/share_note.toml) does the same in a `.toml`
sidecar:

| Tool | Title | Hints |
|------|-------|-------|
| `search_notes` | Search Notes | `read_only=True, open_world=False` |
| `add_note` | Add Note | `destructive=False, open_world=False` |
| `delete_note` | Delete Note | `destructive=True, idempotent=True, open_world=False` |
| `share_note` | Share Note | `destructive=false, idempotent=true, open_world=true` (.toml) |
| `legacy_export` | | none |

What each hint means when `True`:

- `read_only`: the tool changes nothing.
- `destructive`: the tool may delete or overwrite, not only add.
- `idempotent`: calling it again with the same arguments has no further effect.
- `open_world`: the tool reaches outside systems, such as the web.

Leave a hint out when you cannot promise it. Clients then assume the tool may
be destructive and may reach outside systems, which is why `legacy_export`
gets the most cautious treatment.

[client.py](client.py) lists the tools with `client.tools()`, where each dict
has `title`, `read_only`, `destructive`, `idempotent` and `open_world` (`None`
for a hint the server did not state). It then decides which calls to confirm
with a person first:

    notes__add_note: safe to call
    notes__delete_note: ask the person first
    notes__legacy_export: ask the person first, reaches outside systems
    notes__search_notes: safe to call
    notes__share_note: safe to call, reaches outside systems

## Running

Start the server as part of the client script (it launches `scriptling` as a
stdio MCP server over this folder's tools):

    scriptling examples/mcp-tool-hints/client.py

Or serve the same folder over HTTP and point the client at it:

    scriptling --server :8080 --mcp-tools ./examples/mcp-tool-hints/tools
    scriptling examples/mcp-tool-hints/client.py http://127.0.0.1:8080/mcp

## Notes

- Hints are the server's claims, not guarantees: a client uses them to decide
  when to ask a person, and the server must still enforce what a tool may do.
- `client.py` asserts what it reads, so it doubles as a check; `go test` runs
  it against the served folder (`scriptling-cli/server/tool_hints_example_test.go`).
