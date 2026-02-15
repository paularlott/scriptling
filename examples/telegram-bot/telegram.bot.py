"""
Telegram Bot Command Framework

This module provides a command registration framework for building Telegram bots.
It handles command parsing, routing, and auto-generates /help.

Example:

    import telegram.bot

    bot = telegram.bot.New(token)

    def handle_start(cmd):
        cmd.reply("Welcome " + cmd.get_user_name() + "!")

    def handle_echo(cmd):
        if cmd.args:
            cmd.reply(" ".join(cmd.args))
        else:
            cmd.reply("Usage: /echo <text>")

    def handle_menu(cmd):
        keyboard = telegram.inline_keyboard([
            [{"text": "Option 1", "callback_data": "menu_opt1"},
             {"text": "Option 2", "callback_data": "menu_opt2"}]
        ])
        cmd.reply("Choose:", reply_markup=keyboard)

    def handle_menu_button(bot, callback_query):
        data = callback_query["data"]
        bot.answer_callback_query(callback_query["id"], text="Selected: " + data)

    bot.command("/start", handle_start, "Start the bot")
    bot.command("/echo", handle_echo, "Echo text back")
    bot.command("/menu", handle_menu, "Show menu", button_handler=handle_menu_button)

    bot.run()
"""

import logging
import os


class Command:
    """
    Command context passed to command handlers.

    Provides convenient access to the bot, update, and common operations.
    """

    def __init__(self, bot, update, command, args):
        self.bot = bot
        self.update = update
        self.command = command
        self.args = args
        self.cached_chat_id = None
        self.cached_message = None
        self.cached_user = None

    def get_chat_id(self):
        """Get the chat ID from the update."""
        if self.cached_chat_id is None:
            if "message" in self.update and self.update["message"]:
                self.cached_chat_id = self.update["message"]["chat"]["id"]
            elif "callback_query" in self.update and self.update["callback_query"]:
                self.cached_chat_id = self.update["callback_query"]["message"]["chat"]["id"]
            elif "edited_message" in self.update and self.update["edited_message"]:
                self.cached_chat_id = self.update["edited_message"]["chat"]["id"]
        return self.cached_chat_id

    def get_message(self):
        """Get the message from the update."""
        if self.cached_message is None:
            if "message" in self.update:
                self.cached_message = self.update["message"]
            elif "edited_message" in self.update:
                self.cached_message = self.update["edited_message"]
        return self.cached_message

    def get_user(self):
        """Get the user from the update."""
        if self.cached_user is None:
            msg = self.get_message()
            if msg:
                self.cached_user = msg.get("from", {})
            elif "callback_query" in self.update:
                self.cached_user = self.update["callback_query"].get("from", {})
        return self.cached_user

    def get_user_id(self):
        """Get the user ID."""
        u = self.get_user()
        return u.get("id") if u else None

    def get_user_name(self):
        """Get the user's first name."""
        u = self.get_user()
        return u.get("first_name", "there") if u else "there"

    def get_text(self):
        """Get the message text."""
        msg = self.get_message()
        return msg.get("text", "") if msg else ""

    def reply(self, text, **kwargs):
        """Send a reply to the chat."""
        return self.bot.send_message(self.get_chat_id(), text, **kwargs)

    def reply_markdown(self, text, **kwargs):
        """Send a markdown reply to the chat."""
        kwargs["parse_mode"] = "Markdown"
        return self.bot.send_message(self.get_chat_id(), text, **kwargs)

    def typing(self):
        """Send typing indicator."""
        return self.bot.send_chat_action(self.get_chat_id(), "typing")


class CommandBot:
    """
    Command-based Telegram bot with automatic routing and help generation.

    Register commands with .command() and run with .run().
    """

    def __init__(self, token, allowed_users=None):
        """
        Initialize a new CommandBot.

        Parameters:
            token (str): Telegram bot token
            allowed_users (list, optional): List of allowed user IDs
        """
        import telegram
        self.bot = telegram.Bot(token, allowed_users=allowed_users)
        self.commands = {}
        self.callbacks = {}
        self.default_handler = None
        self.help_enabled = True

    def command(self, name, handler, help_text=None, button_handler=None):
        """
        Register a command handler.

        Parameters:
            name (str): Command name (e.g., "/start")
            handler (callable): Function to call, receives Command object
            help_text (str, optional): Help text for auto-generated /help
            button_handler (callable, optional): Handler for inline button callbacks.

        Returns:
            CommandBot: self for chaining
        """
        self.commands[name.lower()] = {
            "handler": handler,
            "help": help_text or name
        }

        if button_handler:
            prefix = name.lstrip("/") + "_"
            self.callbacks[prefix] = button_handler

        return self

    def callback(self, data_prefix, handler):
        """
        Register a callback handler for inline button presses.

        Parameters:
            data_prefix (str): Callback data prefix to match
            handler (callable): Function to call, receives (bot, callback_query)

        Returns:
            CommandBot: self for chaining
        """
        self.callbacks[data_prefix] = handler
        return self

    def default(self, handler):
        """
        Register a default handler for non-command messages.

        Parameters:
            handler (callable): Function to call, receives Command object

        Returns:
            CommandBot: self for chaining
        """
        self.default_handler = handler
        return self

    def no_help(self):
        """
        Disable auto-generated /help command.

        Returns:
            CommandBot: self for chaining
        """
        self.help_enabled = False
        return self

    def handle_update(self, update):
        """Process a single update."""
        if "callback_query" in update:
            callback = update["callback_query"]
            data = callback.get("data", "")

            for prefix, handler in self.callbacks.items():
                if data.startswith(prefix):
                    try:
                        handler(self, callback)
                    except Exception as e:
                        logging.error(f"Callback handler error: {e}")
                    return

            logging.warning(f"No handler for callback data: {data}")
            return

        if "message" not in update and "edited_message" not in update:
            return

        message = update.get("message") or update.get("edited_message")
        text = message.get("text", "")

        if text.startswith("/"):
            parts = text.split()
            cmd = parts[0].lower()
            args = parts[1:] if len(parts) > 1 else []

            if cmd == "/help" and self.help_enabled:
                self.send_help(message["chat"]["id"])
                return

            if cmd in self.commands:
                cmd_ctx = Command(self.bot, update, cmd, args)
                try:
                    self.commands[cmd]["handler"](cmd_ctx)
                except Exception as e:
                    logging.error(f"Command handler error for {cmd}: {e}")
                return

            chat_id = message["chat"]["id"]
            self.bot.send_message(chat_id, f"Unknown command: {cmd}\nUse /help for available commands.")
            return

        if self.default_handler is not None:
            cmd_ctx = Command(self.bot, update, "", [])
            try:
                self.default_handler(cmd_ctx)
            except Exception as e:
                logging.error(f"Default handler error: {e}")

    def send_help(self, chat_id):
        """Send auto-generated help message."""
        text = "*Available Commands:*\n\n"

        for cmd, info in self.commands.items():
            if cmd == "/help":
                continue
            text = text + cmd + " - " + info["help"] + "\n"

        if self.help_enabled:
            text = text + "/help - Show this help message\n"

        self.bot.send_message(chat_id, text, parse_mode="Markdown")

    def run(self, timeout=120):
        """
        Start the bot and begin polling for updates.

        This is a blocking call. Press Ctrl+C to stop.

        Parameters:
            timeout (int): Long polling timeout in seconds
        """
        me = self.bot.get_me()
        if me.get("ok"):
            bot_info = me.get("result", {})
            logging.info(f"Starting bot: @{bot_info.get('username', 'unknown')}")
        else:
            logging.warning("Could not get bot info")

        logging.info("Polling for updates. Press Ctrl+C to stop.")

        # Wrap bound method to match poll_updates signature: handler(bot, update)
        self.bot.poll_updates(lambda bot, update: self.handle_update(update), timeout=timeout)

    # Proxy common methods to underlying bot
    def send_message(self, chat_id, text, **kwargs):
        """Send a message."""
        return self.bot.send_message(chat_id, text, **kwargs)

    def send_photo(self, chat_id, photo, caption=None, **kwargs):
        """Send a photo."""
        return self.bot.send_photo(chat_id, photo, caption, **kwargs)

    def send_document(self, chat_id, document, caption=None, **kwargs):
        """Send a document."""
        return self.bot.send_document(chat_id, document, caption, **kwargs)

    def send_chat_action(self, chat_id, action):
        """Send a chat action."""
        return self.bot.send_chat_action(chat_id, action)

    def answer_callback_query(self, callback_query_id, text=None, show_alert=False):
        """Answer a callback query."""
        return self.bot.answer_callback_query(callback_query_id, text, show_alert)

    def edit_message_text(self, chat_id, message_id, text, **kwargs):
        """Edit message text."""
        return self.bot.edit_message_text(chat_id, message_id, text, **kwargs)

    def edit_message_reply_markup(self, chat_id, message_id, reply_markup):
        """Edit message reply markup."""
        return self.bot.edit_message_reply_markup(chat_id, message_id, reply_markup)


def New(token, allowed_users=None):
    """
    Create a new CommandBot instance.

    Parameters:
        token (str): Telegram bot token
        allowed_users (list, optional): List of allowed user IDs

    Returns:
        CommandBot: New bot instance
    """
    return CommandBot(token, allowed_users)


def Create(token, allowed_users=None):
    """
    Create a new CommandBot instance (alias for New).

    Parameters:
        token (str): Telegram bot token
        allowed_users (list, optional): List of allowed user IDs

    Returns:
        CommandBot: New bot instance
    """
    return New(token, allowed_users)
