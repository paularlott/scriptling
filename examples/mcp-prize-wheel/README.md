# mcp-prize-wheel

A second Scriptling MCP Apps example, deliberately different from
[`mcp-app-dashboard`](../mcp-app-dashboard) in two ways:

1. **No `.toml` file at all for the tools.** `spin_wheel`, `claim_prize`,
   and `get_history` are all registered with the `@mcp.tool(...)` decorator
   — description, parameters, and the `[ui]` linkage all come from the
   decorator's own keyword arguments, in a single `.py` file.
2. **A more "impressive" UI**: an animated SVG prize wheel (pure vanilla
   JS, no AlpineJS, no CDN dependency at all) that spins with a real CSS
   transition, lands on a result, and bursts confetti on a jackpot.

## Layout

```
tools/
  prize_wheel.py    # spin_wheel + claim_prize + get_history, all @mcp.tool-decorated — no .toml
resources/
  ui/prize-wheel/wheel.html    # the wheel itself
  ui/prize-wheel/_wheel.toml   # mimeType + name only — no CSP needed (nothing is external)
```

## What it demonstrates

- **Decorated tool → UI linkage.** `@mcp.tool(..., ui={"resourceUri": "ui://prize-wheel/wheel.html"})`
  — the same `[ui]` shape as the TOML format, just as a keyword argument.
  See [MCP Apps](https://scriptling.dev/reference/libraries/mcp/mcp-apps/)
  for the full reference across all three registration styles (folder,
  decorated, and dynamic/per-request).
- **`visibility: ["app"]`** on `claim_prize`: the wheel's own "Claim" button
  calls it after the spin animation lands; a compliant host hides it from
  the model, since claiming isn't something the model should decide.
- **`tool.return_structured(...)`** for both tools' results.
- **No external dependencies at all.** Unlike `mcp-app-dashboard` (which
  loads AlpineJS + Chart.js from a CDN and declares `[ui.csp]` for it),
  this UI is self-contained vanilla JS/CSS/SVG — so its resource sidecar
  needs no CSP declaration; the host's restrictive same-origin default is
  already enough.
- **Server-authoritative state**, same pattern as the dashboard example:
  each claimed prize is its own KV entry (`claim:000001`, ...) numbered via
  `kv.incr()`, not a shared list — see the dashboard example's README for
  why that avoids a lost-update race under concurrent calls.
- **`get_history` (`visibility: ["app"]`, no `resourceUri`)**: a pure,
  side-effect-free action tool the view calls once on load. A view only ever
  receives the `ui/notifications/tool-result` for the tool call that
  triggered its own mount (or one it makes itself) — never a snapshot of
  what happened after that, e.g. a claim made in a session that's since
  reloaded. `get_history` is `wheel.html`'s way of recovering that: it's
  called unconditionally right after `ui/initialize`, independent of
  whichever tool actually mounted the view.
- **Auto-resize**: after `ui/initialize`, `wheel.html` reports its own
  content height via `ui/notifications/size-changed` (using a
  `ResizeObserver` on `document.documentElement`, capped to the host's
  declared `containerDimensions.maxHeight` if any) so a compliant host can
  size the iframe to fit instead of a fixed guess — see the
  [MCP Apps guide's Container Dimensions section](https://github.com/paularlott/mcp/blob/main/docs/guides/mcp-apps.md#container-dimensions-and-auto-resize)
  for the mechanism and the `mcp-app-host-harness` example for the host side.

## Running

```bash
# Over stdio
scriptling --mcp-tools ./tools --mcp-resources ./resources

# Over HTTP, for the host-simulator test harness
scriptling --server :8095 --mcp-tools ./tools --mcp-resources ./resources
```

Open the Go repo's
[`mcp-app-host-harness`](https://github.com/paularlott/mcp/tree/main/examples/mcp-app-host-harness)
pointed at `http://localhost:8095/mcp`, connect, and call `spin_wheel`.

## Testing

Verified at the protocol level (stdio, both happy and unhappy paths —
`tools/list` shows the right `_meta.ui` per tool including `visibility`,
`spin_wheel`/`claim_prize`/`get_history` return real `structuredContent`, an
out-of-range or missing `index` on `claim_prize` errors cleanly, and
`get_history` reflects a claim made after the view's own mount — the exact
gap it exists to cover) and live in a browser against the host-simulator
harness: connect, spin (wheel animates and lands on the server-chosen
prize), claim (history updates, confetti fires on a jackpot), spin again.
Also verified live through a real host (`lmchatkit`'s chat UI): the iframe
grows from a placeholder height to the wheel's actual content height on
mount, and grows further as the winnings list gets taller, with no resize
feedback loop.

One thing this caught: MCP tool arguments arrive as JSON, and a JSON number
decodes as a float regardless of whether it was written as `2` or `2.0` — so
even a parameter declared `"type": "int"` needs an explicit `int(...)` in
the script before using it to index a list, unlike real Python's `json`
module (which preserves the int/float distinction from the source text).
`claim_prize` does this; see the comment in `prize_wheel.py`.
