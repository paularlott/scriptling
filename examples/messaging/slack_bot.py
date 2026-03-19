"""
Slack Bot Example

Setup:
    1. Go to https://api.slack.com/apps and create a new app "From scratch"
    2. Under "Socket Mode" enable it and generate an App-Level Token (xapp-...) with
       the connections:write scope — this is your SLACK_APP_TOKEN
    3. Under "OAuth & Permissions" add bot scopes:
         chat:write, files:write, users:read, im:history
    4. Under "Event Subscriptions" enable events and subscribe to bot events:
         message.im
    5. Under "App Home":
         - Check "Always Show My Bot as Online"
         - Under "Show Tabs" enable "Messages Tab"
         - Check "Allow users to send Slash commands and messages from the messages tab"
    6. Under "Slash Commands" create each command your bot handles (e.g. /start, /help);
       the Request URL is ignored in Socket Mode — use any placeholder (https://example.com)
    7. Under "Interactivity & Shortcuts" enable interactivity (required for buttons)
    8. Install (or reinstall) the app to your workspace — copy the Bot User OAuth Token (xoxb-...)
       as your SLACK_BOT_TOKEN
    9. Users can DM the bot directly — no channel invite needed

Finding your user ID (for ALLOWED_USERS):
    Click your profile → "Copy member ID"

Setup:
    export SLACK_BOT_TOKEN="xoxb-..."
    export SLACK_APP_TOKEN="xapp-..."
    export ALLOWED_USERS="U12345678,U87654321"  # optional

Run:
    ./bin/scriptling examples/messaging/slack_bot.py
"""

import scriptling.messaging.slack as slack
import bot_handlers
import logging
import os

bot_token = os.environ.get("SLACK_BOT_TOKEN", "")
app_token = os.environ.get("SLACK_APP_TOKEN", "")

if not bot_token:
    logging.error("SLACK_BOT_TOKEN environment variable not set")
    exit(1)
if not app_token:
    logging.error("SLACK_APP_TOKEN environment variable not set")
    exit(1)

allowed_users = [u.strip() for u in os.environ.get("ALLOWED_USERS", "").split(",") if u.strip()]
client = slack.client(bot_token, app_token, allowed_users=allowed_users) if allowed_users else slack.client(bot_token, app_token)

# ── Standalone send (no event loop needed) ────────────────────────────────────
# client.send_message("C1234567890", "Hello from Scriptling!")
# client.send_message("C1234567890", {"title": "Alert", "body": "Something happened.", "color": "red"})

# ── Auth ───────────────────────────────────────────────────────────────────────
# For custom auth logic (overrides allowed_users):
# def handle_auth(ctx):
#     return ctx["user"]["id"] in ["U12345678"]
# client.auth(handle_auth)

# ── Register handlers ──────────────────────────────────────────────────────────

bot_handlers.register_common(client)

logging.info("Bot started. Connecting to Slack Socket Mode...")
client.run()
