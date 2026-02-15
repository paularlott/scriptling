# Telegram Bot Example (Polling Mode)

A simple Telegram bot that uses **long polling** to receive updates. No web server or public URL required!

## Quick Start

### Step 1: Get a Bot Token

1. Open Telegram and search for @BotFather
2. Send `/newbot` and follow the instructions
3. Copy the API token you receive

### Step 2: Set Your Token

```bash
export TELEGRAM_TOKEN="your_bot_token_here"
```

### Step 3: (Optional) Restrict to Specific Users

To limit who can interact with your bot, set the allowed user IDs:

```bash
export TELEGRAM_ALLOWED_USERS="123456789,987654321"
```

Use `/me` command to get your user ID.

### Step 4: Run the Bot

```bash
scriptling bot.py
```

That's it! The bot will start polling for messages immediately.

## Bot Features

- **Echo responses** - Any text message is echoed back
- **/start** - Welcome message with your name
- **/help** - Auto-generated list of available commands
- **/me** - Show your user details (User ID, Chat ID, etc.)
- **/buttons** - Shows navigation inline keyboard
- **/actions** - Shows action inline keyboard
- **/count** - Demonstrates persistent state (counts your uses)
- **/echo <text>** - Echo custom text

## How It Works

This bot uses **long polling**:

1. The bot repeatedly calls Telegram's `getUpdates` API
2. When a new message arrives, the registered handler is called
3. The bot processes the message and sends a response
4. The loop continues until you press Ctrl+C

**Pros of polling:**

- Simple to set up and run
- No web server required
- Works behind NAT/firewalls
- Great for development and testing

**Cons of polling:**

- Not instant (120 second polling timeout, but updates return immediately when available)
- Uses more API calls than webhooks
- Not ideal for high-traffic bots

## Project Structure

```
telegram-bot/
├── bot.py            # Main bot script (uses command framework)
├── telegram.py       # Telegram Bot API library
├── telegram.bot.py   # Command framework with auto-generated /help
└── README.md         # This file
```

## Command Framework

The bot uses `telegram.bot.New()` which provides a clean command registration pattern:

```python
import telegram.bot

bot = telegram.bot.New(token, allowed_users=[123456789])

def handle_start(cmd):
    cmd.reply("Hello " + cmd.get_user_name() + "!")

def handle_echo(cmd):
    if cmd.args:
        cmd.reply(" ".join(cmd.args))

def handle_menu(cmd):
    keyboard = telegram.inline_keyboard([
        [{"text": "Option 1", "callback_data": "menu_opt1"},
         {"text": "Option 2", "callback_data": "menu_opt2"}]
    ])
    cmd.reply("Choose:", reply_markup=keyboard)

def handle_menu_button(bot, callback_query):
    data = callback_query["data"]
    bot.answer_callback_query(callback_query["id"], text="Selected: " + data)

# Register commands with help text and optional button handlers
bot.command("/start", handle_start, "Start the bot")
bot.command("/echo", handle_echo, "Echo text back")
bot.command("/menu", handle_menu, "Show menu", button_handler=handle_menu_button)

# Handle non-command messages
def handle_default(cmd):
    cmd.reply("Echo: " + cmd.get_text())

bot.default(handle_default)

# Start polling
bot.run()
```

### Command Context

Each command handler receives a `Command` object with convenient methods:

| Method            | Description                          |
| ----------------- | ------------------------------------ |
| `get_chat_id()`   | Get the chat ID                      |
| `get_user()`      | Get user object from the message     |
| `get_user_id()`   | Get user's ID                        |
| `get_user_name()` | Get user's first name                |
| `get_text()`      | Get full message text                |
| `reply(text)`     | Send a text reply                    |
| `reply_markdown()`| Send a markdown-formatted reply      |
| `typing()`        | Send typing indicator                |

The `args` attribute is directly accessible for command arguments.

### Framework Methods

| Method                                   | Description                          |
| ---------------------------------------- | ------------------------------------ |
| `bot.command(name, fn, help, button_handler)` | Register a command handler with optional button handler |
| `bot.callback(prefix, fn)`               | Register callback handler for buttons (manual prefix) |
| `bot.default(fn)`                        | Handle non-command messages          |
| `bot.no_help()`                          | Disable auto-generated /help         |
| `bot.run(timeout=120)`                   | Start polling for updates            |

## Extending the Bot

### Adding New Commands

Simply add a new handler function and register it:

```python
def handle_weather(cmd):
    if cmd.args:
        city = " ".join(cmd.args)
        cmd.reply(f"Weather in {city}: Sunny!")
    else:
        cmd.reply("Usage: /weather <city>")

bot.command("/weather", handle_weather, "Get weather for a city")
```

### Handling Inline Buttons

The easiest way is to use `button_handler` when registering a command. The callback data prefix is auto-generated from the command name:

```python
def handle_menu(cmd):
    # For "/menu", use "menu_" prefix in callback_data
    keyboard = telegram.inline_keyboard([
        [{"text": "Delete", "callback_data": "menu_delete"},
         {"text": "Edit", "callback_data": "menu_edit"}]
    ])
    cmd.reply("Choose an action:", reply_markup=keyboard)

def handle_menu_button(bot, callback_query):
    data = callback_query["data"]  # "menu_delete" or "menu_edit"
    action = data.replace("menu_", "")
    bot.answer_callback_query(callback_query["id"], text=f"Action: {action}")
    bot.send_message(callback_query["message"]["chat"]["id"], f"You chose: {action}")

# Auto-registers callback handler with "menu_" prefix
bot.command("/menu", handle_menu, "Show menu", button_handler=handle_menu_button)
```

For manual control, you can still use `bot.callback(prefix, handler)`:

```python
bot.callback("admin_", handle_admin_buttons)  # Matches "admin_delete", "admin_edit", etc.
```

### Storing User State

Use `scriptling.kv` to persist data across restarts:

```python
# Store user data
scriptling.kv.set(f"user:{chat_id}", {"name": "Alice", "count": 0})

# Retrieve user data
user = scriptling.kv.get(f"user:{chat_id}", default={})

# Increment a counter
count = scriptling.kv.incr(f"count:{chat_id}")
```

## Environment Variables

| Variable                  | Description                                          | Required |
| ------------------------- | ---------------------------------------------------- | -------- |
| `TELEGRAM_TOKEN`          | Your bot token from @BotFather                       | Yes      |
| `TELEGRAM_ALLOWED_USERS`  | Comma-separated list of allowed user IDs (optional)  | No       |

## See Also

- [scriptling.kv](../../docs/scriptling.kv.md) - Key-value store documentation
- [Telegram Bot API](https://core.telegram.org/bots/api) - Official API documentation
