"""
Shared bot handlers — used by both telegram_bot.py and discord_bot.py.

Demonstrates cross-platform conformance: identical handler functions work
on Telegram and Discord via the unified ctx interface.
"""

import time


def handle_start(ctx):
    ctx.reply(f"Hello {ctx['user']['name']}! I'm an echo bot.\n\nSend me any message and I'll echo it back.\nUse /help for available commands.")


def handle_whoami(ctx):
    user = ctx["user"]
    ctx.reply(
        f"User ID: {user['id']}\n"
        f"Name: {user['name']}\n"
        f"Platform: {user['platform']}\n"
        f"Dest: {ctx['dest']}"
    )


def handle_rich(ctx):
    ctx.reply({
        "title": "Rich Message Demo",
        "body": "This message demonstrates rich content.\nTelegram renders the title in bold; Discord uses an embed.",
        "color": "blue",
        "image": "https://upload.wikimedia.org/wikipedia/commons/thumb/4/47/PNG_transparency_demonstration_1.png/280px-PNG_transparency_demonstration_1.png",
        "url": "https://scriptling.dev",
    })


def handle_buttons(ctx):
    ctx.reply("Choose an option:", keyboard=[
        [
            {"text": "Option A", "data": "btn_a"},
            {"text": "Option B", "data": "btn_b"},
        ],
        [
            {"text": "Visit Scriptling", "url": "https://scriptling.dev"},
        ],
    ])


def handle_thinking(ctx):
    # Re-send typing every 4s — Telegram's indicator expires after ~5s
    elapsed = 0
    while elapsed < 8:
        ctx.typing()
        time.sleep(4)
        elapsed = elapsed + 4
    ctx.reply("...I've thought about it. 42.")


def handle_echo(ctx):
    if ctx["args"]:
        ctx.reply(" ".join(ctx["args"]))
    else:
        ctx.reply("Usage: /echo <text>")


def handle_file(ctx):
    f = ctx["file"]
    ctx.reply(f"Received file:\nName: {f['name']}\nType: {f['mime']}\nSize: {f['size']} bytes")


def handle_callback(ctx):
    ctx.answer(f"You pressed: {ctx['callback_data']}")
    ctx.reply(f"Button pressed: {ctx['callback_data']}")


def handle_message(ctx):
    if ctx["text"]:
        ctx.reply(f"Echo: {ctx['text']}")


def handle_capabilities(ctx):
    caps = ctx.capabilities()
    ctx.reply("Platform: " + ctx["user"]["platform"] + "\n" + "\n".join("  " + c for c in caps))


def register_common(client):
    """Register all shared handlers on a client (telegram or discord)."""
    client.command("/start",        "Start the bot",         handle_start)
    client.command("/whoami",       "Show your details",     handle_whoami)
    client.command("/rich",         "Send a rich message",   handle_rich)
    client.command("/buttons",      "Show buttons",          handle_buttons)
    client.command("/echo",         "Echo text back",        handle_echo)
    client.command("/thinking",     "Demo typing indicator", handle_thinking)
    client.command("/capabilities", "Show capabilities",     handle_capabilities)
    client.on_callback(handle_callback)
    client.on_file(handle_file)
    client.on_message(handle_message)
