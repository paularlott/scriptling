"""
Console Bot Example

Runs the bot handlers in a local TUI console — no tokens or network required.
Useful for testing handlers before deploying to Telegram/Discord/Slack.

Run:
    ./bin/scriptling examples/messaging/console_bot.py
"""

import scriptling.console as console
import scriptling.messaging.console as messaging_console
import bot_handlers

client = messaging_console.client()

bot_handlers.register_common(client)

client.run()
