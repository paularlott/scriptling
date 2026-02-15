"""
Simple Telegram Bot (Polling Mode)

This bot uses long polling to receive updates from Telegram.
No web server or public URL required - just run and go!

Features:
- Echo bot that repeats messages
- /start command with welcome message
- /help command showing available commands (auto-generated)
- /buttons command with navigation inline keyboard
- /actions command with action inline keyboard
- /count command demonstrating state persistence
"""

import logging
import os
import telegram
import telegram.bot
import scriptling.runtime as runtime


def handle_start(cmd):
    """Handle /start command."""
    cmd.reply(
        f"Hello {cmd.get_user_name()}! Welcome to the Echo Bot!\n\n"
        "Send me any message and I'll echo it back.\n"
        "Use /help to see available commands."
    )


def handle_me(cmd):
    """Handle /me command - show user details."""
    user = cmd.get_user()
    chat_id = cmd.get_chat_id()

    text = "*Your Details:*\n\n"
    text = text + "User ID: `" + str(user.get("id", "unknown")) + "`\n"
    text = text + "Chat ID: `" + str(chat_id) + "`\n"
    text = text + "First Name: " + user.get("first_name", "unknown") + "\n"

    if user.get("last_name"):
        text = text + "Last Name: " + user.get("last_name") + "\n"
    if user.get("username"):
        text = text + "Username: @" + user.get("username") + "\n"
    if user.get("language_code"):
        text = text + "Language: " + user.get("language_code") + "\n"

    cmd.reply_markdown(text)


def handle_buttons(cmd):
    """Handle /buttons command - show inline keyboard with navigation options."""
    keyboard = telegram.inline_keyboard([
        [
            {"text": "Next", "callback_data": "buttons_next"},
            {"text": "Previous", "callback_data": "buttons_prev"},
        ],
        [
            {"text": "Visit Scriptling", "url": "https://github.com/paularlott/scriptling"},
        ],
    ])
    cmd.reply("Navigate:", reply_markup=keyboard)


def handle_buttons_callback(bot, callback_query):
    """Handle button presses from /buttons command."""
    data = callback_query["data"]
    chat_id = callback_query["message"]["chat"]["id"]

    # Map callback data to actions
    if data == "buttons_next":
        bot.answer_callback_query(callback_query["id"], text="Going next!")
        bot.send_message(chat_id, "You selected: Next")
    elif data == "buttons_prev":
        bot.answer_callback_query(callback_query["id"], text="Going back!")
        bot.send_message(chat_id, "You selected: Previous")


def handle_actions(cmd):
    """Handle /actions command - show inline keyboard with action buttons."""
    keyboard = telegram.inline_keyboard([
        [
            {"text": "Delete", "callback_data": "actions_delete"},
            {"text": "Edit", "callback_data": "actions_edit"},
        ],
        [
            {"text": "Share", "callback_data": "actions_share"},
            {"text": "Cancel", "callback_data": "actions_cancel"},
        ],
    ])
    cmd.reply("Choose an action:", reply_markup=keyboard)


def handle_actions_callback(bot, callback_query):
    """Handle button presses from /actions command."""
    data = callback_query["data"]
    chat_id = callback_query["message"]["chat"]["id"]

    # Map callback data to actions
    action = data.replace("actions_", "")
    bot.answer_callback_query(callback_query["id"], text=f"Action: {action}")
    bot.send_message(chat_id, f"You selected: {action.capitalize()}")


def handle_count(cmd):
    """Handle /count command - demonstrate persistent state."""
    key = f"count:{cmd.get_chat_id()}"
    count = runtime.kv.incr(key)
    cmd.reply(f"You've used /count {count} time(s)!")


def handle_echo(cmd):
    """Handle /echo command - echo custom text."""
    if cmd.args:
        cmd.reply(" ".join(cmd.args))
    else:
        cmd.reply("Usage: /echo <text>")


def handle_default(cmd):
    """Handle non-command messages - echo back."""
    if cmd.get_text():
        cmd.typing()
        cmd.reply(f"Echo: {cmd.get_text()}")


def main():
    """Main entry point - setup and start bot."""
    # Get bot token from environment
    token = os.environ.get("TELEGRAM_TOKEN", "")
    if not token:
        logging.error("TELEGRAM_TOKEN environment variable not set")
        logging.error("Get a token from @BotFather on Telegram")
        exit(1)

    # Load allowed users from environment (comma-separated list of user IDs)
    allowed_users = []
    allowed_str = os.environ.get("TELEGRAM_ALLOWED_USERS", "").strip()
    if allowed_str:
        for part in allowed_str.split(","):
            part = part.strip()
            if part:
                try:
                    allowed_users.append(int(part))
                except ValueError:
                    logging.warning(f"Invalid user ID in TELEGRAM_ALLOWED_USERS: {part}")
        if allowed_users:
            logging.info(f"Allowed users filter: {len(allowed_users)} user(s)")

    # Create bot instance using the command framework
    bot = telegram.bot.New(token, allowed_users=allowed_users)

    # Register commands
    bot.command("/start", handle_start, "Start the bot")
    bot.command("/me", handle_me, "Show your user details")
    bot.command("/buttons", handle_buttons, "Show navigation buttons", button_handler=handle_buttons_callback)
    bot.command("/actions", handle_actions, "Show action buttons", button_handler=handle_actions_callback)
    bot.command("/count", handle_count, "Count your count messages")
    bot.command("/echo", handle_echo, "Echo custom text")

    # Register default handler for non-command messages
    bot.default(handle_default)

    # Start polling (blocking)
    bot.run()


# Run the bot
main()
