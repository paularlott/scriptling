# No .toml file for this example: both tools below are registered entirely
# via the @mcp.tool(...) decorator (metadata, parameters, and the MCP Apps
# [ui] linkage all come from the decorator's own keyword arguments).
import random
import scriptling.runtime as runtime
import scriptling.runtime.mcp as mcp
import scriptling.mcp.tool as tool

kv = runtime.kv.default

WHEEL_RESOURCE = "ui://prize-wheel/wheel.html"

PRIZES = [
    "10 Bonus Points",
    "Free Coffee",
    "Try Again",
    "50 Bonus Points",
    "Sticker Pack",
    "JACKPOT: 500 Points",
    "Try Again",
    "Gift Card",
]


# The icon is an inline SVG data: URI (no external asset needed for the demo)
# — illustrates the MCP icons convention. claim_prize deliberately has no
# icon of its own; icons aren't required on every tool.
WHEEL_ICON = {
    "src": "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24'%3E%3Ccircle cx='12' cy='12' r='10' fill='%23ffcf4d'/%3E%3Cpath d='M12 2v20M2 12h20' stroke='%231a1233' stroke-width='1'/%3E%3C/svg%3E",
    "mimeType": "image/svg+xml",
}


@mcp.tool("Spin the prize wheel and see what you win",
          ui={"resourceUri": WHEEL_RESOURCE},
          icons=[WHEEL_ICON])
def spin_wheel():
    index = random.randint(0, len(PRIZES) - 1)
    # Include the claim history here too, not just in claim_prize's own
    # result: the view only ever receives a tool's result via the single
    # ui/notifications/tool-result the host sends right after mount (for
    # whichever tool triggered this mount) or a live in-app call — a fresh
    # spin_wheel mount otherwise has no way to know about prior claims, and
    # neither does a reloaded conversation replaying this same stored result.
    history = [kv.get(key) for key in sorted(kv.keys("claim:*"))]
    tool.return_structured({"index": index, "prize": PRIZES[index], "prizes": PRIZES, "history": history})


# App-only: the wheel's own "Claim" button calls this after the spin
# animation lands, not the model — the model only ever spins. No resourceUri
# here (optional per spec): the wheel view is already open by the time this
# gets called, so there's no view of its own for this tool to render.
@mcp.tool("Claim a prize won from the wheel (called by the wheel's own UI, not the model)",
          params={"index": {"type": "int", "description": "Winning wedge index", "required": True}},
          ui={"visibility": ["app"]})
def claim_prize(index):
    # MCP arguments arrive as JSON; a JSON number decodes as a float
    # regardless of whether it looked like "2" or "2.0", so an "int"-typed
    # parameter still needs an explicit int() before it can index a list.
    index = int(index)
    if index < 0 or index >= len(PRIZES):
        raise ValueError("index out of range")
    prize = PRIZES[index]
    seq = kv.incr("claim_seq")
    kv.set("claim:%06d" % seq, prize)
    history = [kv.get(key) for key in sorted(kv.keys("claim:*"))]
    tool.return_structured({"prize": prize, "history": history})


# App-only, read-only: the wheel view calls this once on load (see wheel.html)
# to fetch the current history itself, rather than relying solely on the
# snapshot pushed alongside whichever tool call triggered this particular
# mount — that snapshot is only ever as fresh as its own originating call
# (see spin_wheel's own comment above), so a claim made anywhere else in the
# conversation after this view's own mount wouldn't otherwise show up until
# the next spin. No side effects, so it's safe to call speculatively on
# every load. Hidden from the model like claim_prize: it has nothing to add
# to the conversation, it's purely for the view's own use.
@mcp.tool("Get the current claim history (called by the wheel's own UI on load, not the model)",
          ui={"visibility": ["app"]})
def get_history():
    history = [kv.get(key) for key in sorted(kv.keys("claim:*"))]
    tool.return_structured({"history": history})
