"""
Discord Bot Example

Getting a bot token:
    1. Go to https://discord.com/developers/applications
    2. Click "New Application", give it a name
    3. Go to the "Bot" tab and click "Add Bot"
    4. Under "Token" click "Reset Token" and copy it
    5. Under "Privileged Gateway Intents" enable "Message Content Intent"
    6. To invite the bot: OAuth2 → URL Generator → scope "bot" → copy and open the URL

Finding your user ID (for ALLOWED_USERS):
    Settings → Advanced → enable Developer Mode, then right-click your username → Copy User ID.

Setup:
    export DISCORD_TOKEN="your-bot-token"
    export ALLOWED_USERS="123456789,987654321"  # optional

Run:
    ./bin/scriptling examples/messaging/discord_bot.py
"""

import scriptling.messaging.discord as discord
import bot_handlers
import logging
import os

token = os.environ.get("DISCORD_TOKEN", "")
if not token:
    logging.error("DISCORD_TOKEN environment variable not set")
    exit(1)

allowed_users = [u.strip() for u in os.environ.get("ALLOWED_USERS", "").split(",") if u.strip()]
client = discord.client(token, allowed_users=allowed_users) if allowed_users else discord.client(token)

# ── Standalone send (no event loop needed) ────────────────────────────────────
# client.send_message("1234567890123456789", "Hello from Scriptling!")
# client.send_message("1234567890123456789", {"title": "Alert", "body": "Something happened.", "color": "red"})

# ── Auth ───────────────────────────────────────────────────────────────────────
# For custom auth logic (overrides allowed_users):
# def handle_auth(ctx):
#     return ctx["user"]["id"] in ["123456789012345678"]
# client.auth(handle_auth)

# ── Register handlers ──────────────────────────────────────────────────────────

bot_handlers.register_common(client)

logging.info("Bot started. Connecting to Discord gateway...")
client.run()
