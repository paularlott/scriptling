# Messaging Examples

Examples using the `scriptling.messaging` libraries. The same handler code
(`bot_handlers.py`) runs unchanged on every platform.

| File | Runs the bot on |
| --- | --- |
| `bot_handlers.py` | Shared handlers registered by every runner below |
| `telegram_bot.py` | Telegram |
| `discord_bot.py` | Discord (direct messages) |
| `slack_bot.py` | Slack (direct messages via Socket Mode) |
| `console_bot.py` | A local TUI console — no tokens or network required |

## The shared bot interface

Every platform module exposes the same client API:

```python
import scriptling.messaging.telegram as telegram

client = telegram.client(token, allowed_users=["123456789"])  # allowed_users optional

# Register handlers
client.command("/start", "Start the bot", handle_start)   # (name, help text, handler)
client.on_message(handle_message)    # plain (non-command) text
client.on_file(handle_file)          # file/photo attachments
client.on_callback(handle_callback)  # button presses; or on_callback(prefix, handler)
client.auth(handle_auth)             # return True to allow, False to deny

client.run()  # start the event loop (blocks until stopped)
```

Constructors differ only in credentials:

```python
import scriptling.messaging.telegram as telegram
import scriptling.messaging.discord  as discord
import scriptling.messaging.slack    as slack
import scriptling.messaging.console  as messaging_console

tg  = telegram.client(token)
dc  = discord.client(token)
sl  = slack.client(bot_token, app_token)
con = messaging_console.client()   # no arguments — creates its own TUI
```

`allowed_users` installs a default auth handler; updates from anyone else are
silently dropped. Use `client.auth(fn)` for custom logic (it replaces the
default handler).

## The handler `ctx` argument

Every handler receives a `ctx` dict describing the update:

```python
{
    "dest":           "123456789",   # where to reply (chat / channel ID)
    "message_id":     "42",
    "text":           "/echo hello",
    "command":        "/echo",
    "args":           ["hello"],
    "is_callback":    False,
    "callback_id":    "",            # set when a button was pressed
    "callback_token": "",
    "callback_data":  "",            # the pressed button's data value
    "file":           None,          # or {"id", "name", "mime", "size", "url"}
    "user":           {"id": "99", "name": "Alice", "platform": "telegram"},
}
```

`ctx` also carries helper methods:

```python
ctx.reply(text)                     # plain reply
ctx.reply({...})                    # rich message dict (see below)
ctx.reply(text, keyboard=[...])     # reply with buttons
ctx.typing()                        # typing indicator
ctx.answer("Got it!")               # acknowledge a button press
ctx.download()                      # download this update's file → base64 string
ctx.capabilities()                  # list of capability strings
ctx.has_capability("rich_message")  # True/False feature check
```

## Rich messages, keyboards, files

A rich message is a dict; each platform renders what it supports:

```python
ctx.reply({
    "title": "Build finished",
    "body":  "All tests passed.",
    "color": "green",
    "image": "https://example.com/chart.png",
    "url":   "https://scriptling.dev",
})
```

Keyboards are a list of rows; each button is `{"text", "data"}` for a callback
button or `{"text", "url"}` for a link button:

```python
ctx.reply("Choose:", keyboard=[
    [{"text": "Yes", "data": "yes"}, {"text": "No", "data": "no"}],
    [{"text": "Visit", "url": "https://scriptling.dev"}],
])
```

Received files can be downloaded as base64:

```python
f = ctx["file"]
data = ctx.download()
```

## Proactive sends (no event loop required)

```python
client.send_message(dest, "Hello!")
client.send_message(dest, {"title": "Alert", "body": "Disk almost full.", "color": "red"})
client.send_file(dest, "/tmp/report.pdf", filename="report.pdf", caption="Monthly report")
client.send_file(dest, b64_data, filename="chart.png", base64=True)
client.edit_message(dest, message_id, "Updated text")
client.delete_message(dest, message_id)
client.typing(dest)
```

On Slack, open a DM channel first: `channel_id = client.open_dm(user_id)`,
then use it as `dest`.

## Capabilities

Feature support differs per platform. Check it with the exact capability
strings — `rich_message`, `rich_message.title`, `rich_message.body`,
`rich_message.color`, `rich_message.image`, `rich_message.url`, `keyboard`,
`keyboard.callback`, `keyboard.url`, `typing`, `edit_message`,
`delete_message`, `send_file`, `download`:

```python
if ctx.has_capability("rich_message"):
    ctx.reply({"title": "Alert", "body": "Disk almost full.", "color": "red"})
else:
    ctx.reply("Alert: disk almost full.")
```

| Capability | Telegram | Discord | Slack | Console |
| --- | --- | --- | --- | --- |
| rich title / body | ✓ | ✓ | ✓ | ✓ |
| rich color | — | ✓ | ✓ | ✓ |
| rich image | ✓ | ✓ | ✓ | — |
| rich url | — | ✓ | ✓ | — |
| keyboards | ✓ inline | ✓ buttons | ✓ Block Kit | ✓ menu |
| typing | ✓ | ✓ | — | ✓ spinner |
| send / download files | ✓ | ✓ | ✓ | — |
| edit / delete messages | ✓ | ✓ | ✓ | no-op |

## Running the examples

Each runner script has full setup instructions (tokens, scopes, intents) in its
docstring. The console bot needs no setup and is the quickest way to try the
handlers locally:

```bash
./bin/scriptling examples/messaging/console_bot.py
```

Platform runners:

```bash
export TELEGRAM_TOKEN="..."   # from @BotFather
./bin/scriptling examples/messaging/telegram_bot.py

export DISCORD_TOKEN="..."    # from the Discord Developer Portal
./bin/scriptling examples/messaging/discord_bot.py

export SLACK_BOT_TOKEN="xoxb-..."   # Bot User OAuth Token
export SLACK_APP_TOKEN="xapp-..."   # Socket Mode app-level token
./bin/scriptling examples/messaging/slack_bot.py
```

Optionally set `ALLOWED_USERS` (comma-separated platform user IDs) to restrict
who can talk to the bot.
