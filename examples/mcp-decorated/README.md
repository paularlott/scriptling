# Decorated Registrations Example

This example registers every kind of MCP entry from script code with the
decorators of `scriptling.runtime.mcp`, no `.toml` sidecars and no separate
folders: [tools/shop.py](tools/shop.py) defines

- a tool with `@mcp.tool` (`price_lookup`),
- a static resource with `@mcp.resource` (`config://shop`, JSON),
- a resource template with `@mcp.resource(template=True)` (`shop://catalog/{category}`),
- a prompt with `@mcp.prompt` (`describe`),
- a skill with `@mcp.skill` (`shipping_regions`, with a supporting `regions.md` file).

## Running

Start the server as part of the client script (it launches `scriptling` as a
stdio MCP server over this folder's tools):

    scriptling examples/mcp-decorated/client.py

Or serve the same folder over HTTP and point any MCP client at it:

    scriptling --server :8080 --mcp-tools ./examples/mcp-decorated/tools

## Notes

- Resources and prompts run their function on every read or render, in a
  fresh interpreter; skills run once at startup (skill content is static).
- A skill's `SKILL.md` frontmatter `name` must match the decorated function's
  name, and the `files=` dict supplies supporting files.
- Decorated registrations live in the MCP tools folder and reload with it.
