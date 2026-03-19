# Messaging Examples

Examples using the `scriptling.messaging` libraries.

## Telegram Bot (`telegram_bot.py`)

A simple echo bot demonstrating the `scriptling.messaging.telegram` library.

**Features:**
- `/start` — welcome message
- `/me` — show user ID, chat ID, name
- `/buttons` — inline keyboard with callbacks
- `/echo <text>` — echo text back
- `/help` — auto-generated help
- Default handler — echoes any non-command message
- File/photo handler — shows metadata for received files and photos

**Setup:**

```bash
export TELEGRAM_TOKEN="your-bot-token-from-botfather"
export TELEGRAM_ALLOWED_USERS="123456789,987654321"  # optional allowlist
```

**Run:**

```bash
./bin/scriptling examples/messaging/telegram_bot.py
```

**Key API patterns shown:**

```python
import scriptling.messaging.telegram as telegram

# Create client (allowed_users is optional)
client = telegram.client(token, allowed_users=["123456"])

# Send messages
telegram.send_message(client, chat_id, "Hello!")
telegram.send_message(client, chat_id, "*Bold*", parse_mode="Markdown")

# Inline keyboard
kb = telegram.inline_keyboard([
    [{"text": "Yes", "callback_data": "yes"}, {"text": "No", "callback_data": "no"}],
    [{"text": "Visit", "url": "https://scriptling.dev"}],
])
telegram.send_message(client, chat_id, "Choose:", reply_markup=kb)

# Answer a button press
telegram.answer_callback(client, update["callback_id"], "Got it!")

# Send media
telegram.send_photo(client, chat_id, "/tmp/image.png")
telegram.send_photo(client, chat_id, "https://example.com/img.png", caption="A photo")
telegram.send_photo(client, chat_id, b64_data, base64=True)
telegram.send_file(client, chat_id, "/tmp/report.pdf", filename="report.pdf")

# Download received file/photo (returns base64 string)
data = update["download"]()

# Poll for updates — blocks until stopped (Ctrl+C)
telegram.poll_updates(client, handler)
```

The `update` dict passed to handlers:

```python
{
    "chat_id":      123456,
    "user":         {"id": "99", "name": "Alice", "platform": "telegram"},
    "text":         "/start",
    "is_callback":  False,
    "callback_id":  "",
    "callback_data": "",
    "file":         None,   # or {"id": "...", "name": "...", "mime": "...", "size": 1024}
    "photo":        None,   # or {"id": "...", "width": 1280, "height": 720, "size": 98765}
    "download":     <function>,  # call download() to get base64 bytes
}
```
