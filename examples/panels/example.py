#!/usr/bin/env scriptling
"""
Panels Demo — shows a multi-panel TUI layout with background updates.

Run:
    ./bin/scriptling examples/panels/example.py

Layout:
    ┌──────────┬─────────────────────┬──────────────┐
    │  Logs    │      Main Chat      │  CPU Stats   │
    │          │                     ├──────────────┤
    │          │                     │ Memory Stats │
    └──────────┴─────────────────────┴──────────────┘

Type messages to chat. Tab cycles panel focus. /layout toggles panels.
Ctrl+C or /exit to quit.
"""

import scriptling.console as console
import scriptling.runtime as runtime
import time

# ── Create panels (independent of layout) ──────────────────────────────

logs = console.create_panel("logs", width=-25, min_width=15, scrollable=True, title="Logs")
right = console.create_panel(width=-30, min_width=16)
cpu_panel = console.create_panel("cpu", height=-50, title="CPU")
mem_panel = console.create_panel("mem", height=-50, title="Memory")

# ── Build layout tree ─────────────────────────────────────────────────

right.add_row(cpu_panel)
right.add_row(mem_panel)

console.add_left(logs)
console.add_right(right)

# ── Shared state for background updaters ──────────────────────────────

shared = runtime.sync.Shared("panel_state", {
    "log_index": 0,
    "cpu_val": 25.0,
    "mem_val": 40.0,
    "running": True,
})

# ── Slash commands ────────────────────────────────────────────────────

def cmd_clear(args):
    console.main_panel().clear()

def cmd_layout(args):
    if console.has_panels():
        console.clear_layout()
    else:
        console.add_left(logs)
        console.add_right(right)

console.register_command("clear", "Clear main output", cmd_clear)
console.register_command("layout", "Toggle panel layout", cmd_layout)

# ── Helper (copied as sibling into clean env) ────────────────────────

def make_bar(pct, width=16):
    """Build a text progress bar."""
    filled = int(pct / 100 * width)
    return "█" * filled + "░" * (width - filled)

# ── Background tasks ─────────────────────────────────────────────────
#
# Background functions run in clean environments — imports (console,
# runtime, time) and sibling functions (make_bar) are copied in, but
# data (panels, shared state) must be looked up by name.

LOG_MESSAGES = [
    "Starting service on :8080",
    "Connected to database",
    "Request received: GET /api/health",
    "Cache miss for key user:42",
    "Retrying connection attempt 1/3",
    "TLS handshake completed",
    "Worker pool expanded to 8",
    "Scheduled job completed",
    "Incoming WebSocket connection",
    "Session token refreshed",
]

def pump_logs(messages):
    """Background task that appends log messages to the logs panel."""
    logs = console.panel("logs")
    shared = runtime.sync.Shared("panel_state")

    while True:
        state = shared.get()
        if not state["running"]:
            break

        idx = state["log_index"]
        msg = messages[idx % len(messages)]

        timestamp = time.strftime("%H:%M:%S")
        line = console.styled(console.DIM, timestamp) + " " + msg
        logs.write(line + "\n")

        state["log_index"] = idx + 1
        shared.set(state)
        time.sleep(0.8)


def update_cpu():
    """Background task that updates the CPU stats panel."""
    cpu_panel = console.panel("cpu")
    shared = runtime.sync.Shared("panel_state")

    while True:
        state = shared.get()
        if not state["running"]:
            break

        cpu = state["cpu_val"]
        # Random walk
        cpu = cpu + (hash(time.time()) % 100 - 50) / 5.0
        if cpu < 5:
            cpu = 5
        if cpu > 95:
            cpu = 95

        bar = make_bar(cpu)
        content = console.styled(console.PRIMARY, "CPU Usage") + "\n\n"
        content += "  " + bar + "\n\n"
        content += "  %.0f%%" % cpu
        cpu_panel.set_content(content)

        state["cpu_val"] = cpu
        shared.set(state)
        time.sleep(1)


def update_mem():
    """Background task that updates the Memory stats panel."""
    mem_panel = console.panel("mem")
    shared = runtime.sync.Shared("panel_state")

    while True:
        state = shared.get()
        if not state["running"]:
            break

        mem = state["mem_val"]
        mem = mem + (hash(time.time()) % 100 - 50) / 10.0
        if mem < 20:
            mem = 20
        if mem > 90:
            mem = 90

        bar = make_bar(mem)
        content = console.styled(console.PRIMARY, "Memory Usage") + "\n\n"
        content += "  " + bar + "\n\n"
        content += "  %.0f%%" % mem
        mem_panel.set_content(content)

        state["mem_val"] = mem
        shared.set(state)
        time.sleep(1)


# ── Start background tasks ────────────────────────────────────────────

runtime.background("log_pump", "pump_logs", LOG_MESSAGES)
runtime.background("cpu_updater", "update_cpu")
runtime.background("mem_updater", "update_mem")

# ── Submit handler ────────────────────────────────────────────────────

def on_submit(text):
    console.main_panel().stream_start(label="Echo Bot")
    console.main_panel().stream_chunk('You said: "' + text + '"')
    console.main_panel().stream_end()

console.on_submit(on_submit)

# ── Welcome message and run ───────────────────────────────────────────

main = console.main_panel()
console.set_status("Panels Demo", "Tab: cycle · /layout: toggle · /exit: quit")
main.add_message(console.styled(console.PRIMARY, "Panels Demo") + " ready!")
main.add_message(
    "This example shows a four-panel layout:\n"
    "\n"
    "  Left: Live log stream\n"
    "  Center: Chat (main panel)\n"
    "  Right top: CPU stats\n"
    "  Right bottom: Memory stats\n"
    "\n"
    "Type a message and press Enter to chat.\n"
    "Press Tab to cycle focus between panels.\n"
    "Try /clear or /layout commands."
)

console.run()
