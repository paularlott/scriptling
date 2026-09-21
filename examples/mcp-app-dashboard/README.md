# mcp-app-dashboard

A Scriptling counterpart to the Go [`mcp-app-dashboard`](https://github.com/paularlott/mcp/tree/main/examples/mcp-app-dashboard)
example: the same sales dashboard (data table, Chart.js chart, an "Add Sale"
form that writes back through a second tool), built entirely as `tools/*.toml`
+ `.py` and a static resource — no Go code at all.

It demonstrates the [MCP Apps extension](https://github.com/modelcontextprotocol/ext-apps)
(SEP-1865) support added to Scriptling's tool/resource metadata:

- **`[ui]` in a tool's `.toml`** links it to a companion `ui://` resource.
  `sales_report`'s `.toml` sets `resourceUri`; `add_sale`'s also sets
  `visibility = ["app"]`, since it's meant to be called by the dashboard's own
  form, not the model.
- **`[ui]` in a resource's `_{stem}.toml` sidecar** declares CSP hints for the
  resource's `resources/read` response. `resources/ui/sales-dashboard/_dashboard.toml`
  declares `resourceDomains = ["https://cdn.jsdelivr.net"]`, since the
  dashboard loads AlpineJS (the `@alpinejs/csp` build — see the note in
  [`dashboard.html`](resources/ui/sales-dashboard/dashboard.html) about why)
  and Chart.js from there.

## Layout

```
tools/
  sales_report.toml   description + [ui] linking to the dashboard
  sales_report.py      returns the current sales records
  add_sale.toml        parameters + [ui] with visibility = ["app"]
  add_sale.py           appends a record, returns the updated list
resources/
  ui/sales-dashboard/dashboard.html    the UI itself (shared with the Go example)
  ui/sales-dashboard/_dashboard.toml   mimeType + [ui.csp]
```

`resources/ui/sales-dashboard/dashboard.html` is served at
`ui://sales-dashboard/dashboard.html` — the first path segment under
`--mcp-resources` is always the URI scheme, and a static file keeps its
extension in the URI (see [Resources and Prompts](https://scriptling.dev/docs/cli/mcp-server/#resources-and-prompts)).

## Storage: one KV key per sale, not a shared list

Both tools use `scriptling.runtime`'s KV store (available out of the box, no
`--kv-storage` flag needed — it just runs in-memory), but deliberately
**don't** store the sales as one shared list value. A naive
`get(list) → append → set(list)` is a classic lost-update race: if two
`tools/call` requests overlap, both read the same starting list, and whichever
`set()` finishes last silently discards the other's addition (this is exactly
what building this example initially did, and it was caught by testing with
overlapping concurrent requests — a real host issuing a prefetch `resources/read`
alongside a `tools/call`, for instance, could trigger it).

Instead, each sale is its own key (`sale:000001`, `sale:000002`, ...),
numbered with `kv.incr("sale_seq")` — atomic, so concurrent `add_sale` calls
always get distinct keys and never collide or overwrite each other. Reading
the report just lists and sorts `sale:*`. The one remaining edge case — two
*never-before-seeded* calls racing on the "is anything seeded yet?" check —
can write duplicate (but identical, harmless) seed rows; a real application
storing anything less trivial than demo seed data would want a proper
init-once mechanism, which is out of scope for this example.

## Structured content

Both tools return their data with `tool.return_structured({"records": ...})`,
which sets the MCP result's `structuredContent` field (matching the Go
example's `NewToolResponseStructured`) — plus, per the MCP spec's backwards
compatibility guidance, the same JSON as a text content block, so clients
that don't read `structuredContent` still get it. `dashboard.html` reads
`structuredContent` first and only falls back to `JSON.parse()`-ing the text
block for servers that don't set it, so the same HTML file keeps working
against either this or a more minimal server that only returns text.

`return_structured()` requires a dict (a JSON object), per the MCP spec's
requirement that `structuredContent` be an object — use `return_object()`
instead for a list, string, or other JSON value that isn't a dict.

## Running

```bash
# Over stdio (the transport MCP hosts use for a subprocess)
scriptling --mcp-tools ./tools --mcp-resources ./resources

# Or over HTTP, for the bundled host-simulator test harness
scriptling --server :8091 --mcp-tools ./tools --mcp-resources ./resources
```

Then open the Go repo's [`mcp-app-host-harness`](https://github.com/paularlott/mcp/tree/main/examples/mcp-app-host-harness)
(a standalone static page, not Scriptling-specific) pointed at
`http://localhost:8091/mcp` to see it actually render.

## Testing

This example was verified with `scriptling`'s own MCP protocol handlers
(`go test ./scriptling-cli/mcp/...` in the main repo covers the `[ui]` TOML
parsing and CSP metadata attachment) plus manual protocol-level runs of this
example specifically:

```bash
# tools/list carries _meta.ui with the right resourceUri/visibility;
# resources/read returns the HTML with _meta.ui.csp; sequential tools/call
# requests accumulate correctly (4 seed records -> 5 after one add_sale).
```

A sequential request/response run (the realistic case — a real host waits for
each response before sending the next) shows exactly this: 4 seed records,
then 5 after `add_sale`, then 5 again on the next `sales_report`, no
duplicates, no lost updates. Firing every request in one unread batch (an
adversarial stress test, not realistic host behavior) can surface the
transient duplicate-seed and stale-read edge cases described above; that's
expected of the simplified storage model, not a protocol bug.
