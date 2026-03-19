"""
Telegram Bot Example

Getting a bot token:
    1. Open Telegram and message @BotFather
    2. Send /newbot and follow the prompts
    3. Copy the token BotFather gives you

Finding your user ID (for ALLOWED_USERS):
    Message @userinfobot — it will reply with your numeric user ID.

Setup:
    export TELEGRAM_TOKEN="your-bot-token-from-botfather"
    export ALLOWED_USERS="123456789,987654321"  # optional

Run:
    ./bin/scriptling examples/messaging/telegram_bot.py
"""

import scriptling.messaging.telegram as telegram
import bot_handlers
import logging
import os

token = os.environ.get("TELEGRAM_TOKEN", "")
if not token:
    logging.error("TELEGRAM_TOKEN environment variable not set")
    exit(1)

allowed_users = [u.strip() for u in os.environ.get("ALLOWED_USERS", "").split(",") if u.strip()]
client = telegram.client(token, allowed_users=allowed_users) if allowed_users else telegram.client(token)

# ── Standalone send (no event loop needed) ────────────────────────────────────
# client.send_message("123456789", "Hello from Scriptling!")
# client.send_message("123456789", {"title": "Alert", "body": "Something happened."})

# ── Auth ───────────────────────────────────────────────────────────────────────
# For custom auth logic (overrides allowed_users):
# def handle_auth(ctx):
#     return ctx["user"]["id"] in ["123456789"]
# client.auth(handle_auth)

# ── Register handlers ──────────────────────────────────────────────────────────

bot_handlers.register_common(client)

logging.info("Bot started. Press Ctrl+C to stop.")
client.run()
