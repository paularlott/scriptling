"""
Telegram Bot API library for Scriptling.

This library provides a simple interface for creating Telegram bots.
Supports both polling mode (getUpdates) and webhook mode.

Example usage:

    # Polling mode (simple, no web server needed)
    import telegram
    import os

    bot = telegram.Bot(os.environ["TELEGRAM_TOKEN"])

    def handle_update(bot, update):
        if "message" not in update:
            return
        chat_id = update["message"]["chat"]["id"]
        text = update["message"]["text"]
        bot.send_message(chat_id, f"Echo: {text}")

    bot.poll_updates(handle_update)

    # Webhook mode (requires public HTTPS URL)
    # In webhook handler:
    import scriptling.http
    import scriptling.kv

    def webhook(request):
        token = scriptling.kv.get("telegram_token")
        bot = telegram.Bot(token)
        update = request.json()
        if "message" in update:
            chat_id = update["message"]["chat"]["id"]
            bot.send_message(chat_id, "Got it!")
        return scriptling.http.json(200, {"status": "ok"})
"""

import logging
import requests


class Bot:
    """
    Telegram Bot class for interacting with the Telegram Bot API.

    The Bot class is stateless - it just holds the token and makes HTTP calls.
    Creating a Bot instance per request is cheap - it's just a token wrapper.
    """

    def __init__(self, token, allowed_users=None):
        """
        Initialize a new Bot instance.

        Parameters:
            token (str): Your Telegram bot token from @BotFather
            allowed_users (list, optional): List of allowed user IDs.
                                            If None or empty, all users are allowed.
        """
        self.token = token
        self.base_url = f"https://api.telegram.org/bot{token}"
        self.allowed_users = allowed_users if allowed_users else []

    def is_user_allowed(self, user_id):
        """
        Check if a user ID is in the allowed list.

        Parameters:
            user_id (int): The user ID to check

        Returns:
            bool: True if allowed (or no filter set), False otherwise
        """
        if not self.allowed_users:
            return True
        return user_id in self.allowed_users

    def _get_user_id_from_update(self, update):
        """
        Extract the user ID from an update.

        Parameters:
            update (dict): The update object

        Returns:
            int or None: The user ID, or None if not found
        """
        # Check message
        if "message" in update and update["message"]:
            return update["message"].get("from", {}).get("id")
        # Check callback query
        if "callback_query" in update and update["callback_query"]:
            return update["callback_query"].get("from", {}).get("id")
        # Check edited message
        if "edited_message" in update and update["edited_message"]:
            return update["edited_message"].get("from", {}).get("id")
        # Check channel post
        if "channel_post" in update and update["channel_post"]:
            return update["channel_post"].get("from", {}).get("id")
        return None

    def _get_user_info_from_update(self, update):
        """
        Extract user info (id and name) from an update.

        Parameters:
            update (dict): The update object

        Returns:
            dict: {"id": user_id, "name": display_name} or None if not found
        """
        user = None
        # Check message
        if "message" in update and update["message"]:
            user = update["message"].get("from", {})
        # Check callback query
        elif "callback_query" in update and update["callback_query"]:
            user = update["callback_query"].get("from", {})
        # Check edited message
        elif "edited_message" in update and update["edited_message"]:
            user = update["edited_message"].get("from", {})
        # Check channel post
        elif "channel_post" in update and update["channel_post"]:
            user = update["channel_post"].get("from", {})

        if not user:
            return None

        user_id = user.get("id")
        # Build display name
        name_parts = []
        if user.get("first_name"):
            name_parts.append(user.get("first_name"))
        if user.get("last_name"):
            name_parts.append(user.get("last_name"))
        if user.get("username"):
            name_parts.append("@" + user.get("username"))

        display_name = " ".join(name_parts) if name_parts else str(user_id)
        return {"id": user_id, "name": display_name}

    def _api_call(self, method, params=None):
        """
        Make an API call to Telegram.

        Parameters:
            method (str): The API method name
            params (dict): Parameters for the API call

        Returns:
            dict: The API response
        """
        if params is None:
            params = {}

        response = requests.post(
            f"{self.base_url}/{method}",
            json=params,
            timeout=30
        )

        return response.json()

    def get_me(self):
        """
        Get information about the bot.

        Returns:
            dict: Bot information including id, is_bot, first_name, username, etc.

        Example:
            info = bot.get_me()
            print(info["result"]["username"])
        """
        return self._api_call("getMe")

    def send_message(self, chat_id, text, **kwargs):
        """
        Send a text message to a chat.

        Parameters:
            chat_id (int or str): Unique identifier for the target chat
            text (str): Text of the message to send
            **kwargs: Optional parameters:
                - parse_mode (str): "Markdown", "MarkdownV2", or "HTML"
                - disable_notification (bool): Send silently
                - reply_to_message_id (int): Reply to a specific message
                - reply_markup (dict): Inline keyboard, etc.

        Returns:
            dict: API response with sent message info

        Example:
            bot.send_message(chat_id, "Hello, World!")
            bot.send_message(chat_id, "*Bold*", parse_mode="Markdown")
        """
        params = {"chat_id": chat_id, "text": text}
        params.update(kwargs)
        return self._api_call("sendMessage", params)

    def edit_message_text(self, chat_id, message_id, text, **kwargs):
        """
        Edit text of a previously sent message.

        Parameters:
            chat_id (int or str): Unique identifier for the target chat
            message_id (int): Identifier of the message to edit
            text (str): New text of the message
            **kwargs: Optional parameters (parse_mode, etc.)

        Returns:
            dict: API response

        Example:
            bot.edit_message_text(chat_id, msg_id, "Updated text")
        """
        params = {"chat_id": chat_id, "message_id": message_id, "text": text}
        params.update(kwargs)
        return self._api_call("editMessageText", params)

    def delete_message(self, chat_id, message_id):
        """
        Delete a message.

        Parameters:
            chat_id (int or str): Unique identifier for the target chat
            message_id (int): Identifier of the message to delete

        Returns:
            dict: API response

        Example:
            bot.delete_message(chat_id, msg_id)
        """
        params = {"chat_id": chat_id, "message_id": message_id}
        return self._api_call("deleteMessage", params)

    def send_photo(self, chat_id, photo, caption=None, **kwargs):
        """
        Send a photo.

        Parameters:
            chat_id (int or str): Unique identifier for the target chat
            photo (str): File_id, URL, or file to send
            caption (str, optional): Photo caption
            **kwargs: Optional parameters

        Returns:
            dict: API response

        Example:
            bot.send_photo(chat_id, "https://example.com/photo.jpg")
            bot.send_photo(chat_id, file_id, caption="My photo")
        """
        params = {"chat_id": chat_id, "photo": photo}
        if caption:
            params["caption"] = caption
        params.update(kwargs)
        return self._api_call("sendPhoto", params)

    def send_document(self, chat_id, document, caption=None, **kwargs):
        """
        Send a document.

        Parameters:
            chat_id (int or str): Unique identifier for the target chat
            document (str): File_id, URL, or file to send
            caption (str, optional): Document caption
            **kwargs: Optional parameters

        Returns:
            dict: API response

        Example:
            bot.send_document(chat_id, "https://example.com/file.pdf")
        """
        params = {"chat_id": chat_id, "document": document}
        if caption:
            params["caption"] = caption
        params.update(kwargs)
        return self._api_call("sendDocument", params)

    def send_chat_action(self, chat_id, action):
        """
        Send a chat action (typing, uploading, etc.).

        Parameters:
            chat_id (int or str): Unique identifier for the target chat
            action (str): Type of action: "typing", "upload_photo", "record_video",
                         "upload_video", "record_voice", "upload_voice",
                         "upload_document", "choose_sticker", "find_location",
                         "record_video_note", "upload_video_note"

        Returns:
            dict: API response

        Example:
            bot.send_chat_action(chat_id, "typing")
        """
        params = {"chat_id": chat_id, "action": action}
        return self._api_call("sendChatAction", params)

    def answer_callback_query(self, callback_query_id, text=None, show_alert=False):
        """
        Answer a callback query from an inline button press.

        Parameters:
            callback_query_id (str): Unique identifier for the query
            text (str, optional): Text to show the user
            show_alert (bool): If True, show as popup instead of toast

        Returns:
            dict: API response

        Example:
            bot.answer_callback_query(query_id, "Button clicked!")
        """
        params = {"callback_query_id": callback_query_id}
        if text:
            params["text"] = text
        if show_alert:
            params["show_alert"] = True
        return self._api_call("answerCallbackQuery", params)

    def edit_message_reply_markup(self, chat_id, message_id, reply_markup):
        """
        Edit only the reply markup of a message.

        Parameters:
            chat_id (int or str): Unique identifier for the target chat
            message_id (int): Identifier of the message to edit
            reply_markup (dict): New inline keyboard

        Returns:
            dict: API response
        """
        params = {
            "chat_id": chat_id,
            "message_id": message_id,
            "reply_markup": reply_markup
        }
        return self._api_call("editMessageReplyMarkup", params)

    def set_webhook(self, url, secret_token=None, **kwargs):
        """
        Set a webhook for receiving updates.

        Parameters:
            url (str): HTTPS URL to receive updates
            secret_token (str, optional): Secret token for X-Telegram-Bot-Api-Secret-Token header
            **kwargs: Optional parameters (allowed_updates, etc.)

        Returns:
            dict: API response

        Example:
            bot.set_webhook("https://example.com/webhook", secret_token="my_secret")
        """
        params = {"url": url}
        if secret_token:
            params["secret_token"] = secret_token
        params.update(kwargs)
        return self._api_call("setWebhook", params)

    def delete_webhook(self):
        """
        Remove the webhook.

        Returns:
            dict: API response

        Example:
            bot.delete_webhook()
        """
        return self._api_call("deleteWebhook")

    def get_webhook_info(self):
        """
        Get current webhook status.

        Returns:
            dict: Webhook information

        Example:
            info = bot.get_webhook_info()
            print(info["result"]["url"])
        """
        return self._api_call("getWebhookInfo")

    def get_updates(self, offset=None, timeout=30, **kwargs):
        """
        Get new updates using long polling.

        Parameters:
            offset (int, optional): Identifier of the first update to be returned
            timeout (int): Timeout in seconds for long polling
            **kwargs: Optional parameters (limit, allowed_updates, etc.)

        Returns:
            dict: API response with list of updates

        Example:
            updates = bot.get_updates(offset=update_id + 1)
        """
        params = {"timeout": timeout}
        if offset:
            params["offset"] = offset
        params.update(kwargs)
        return self._api_call("getUpdates", params)

    def poll_updates(self, handler, timeout=30):
        """
        Start polling for updates and call handler for each one.

        This is a blocking call that runs indefinitely.
        Press Ctrl+C to stop.

        Parameters:
            handler (callable): Function to call for each update.
                               Signature: handler(bot, update)
            timeout (int): Long polling timeout in seconds

        Example:
            def handle_update(bot, update):
                chat_id = update["message"]["chat"]["id"]
                bot.send_message(chat_id, "Hello!")

            bot.poll_updates(handle_update)
        """
        offset = 0
        while True:
            try:
                result = self.get_updates(offset=offset, timeout=timeout)
                if result.get("ok"):
                    updates = result.get("result", [])
                    for update in updates:
                        offset = update["update_id"] + 1
                        # Check if user is allowed (skip if not)
                        user_info = self._get_user_info_from_update(update)
                        if user_info is not None and not self.is_user_allowed(user_info["id"]):
                            logging.warning(f"Rejected update from unauthorized user: {user_info['name']} ({user_info['id']})")
                            continue
                        try:
                            handler(self, update)
                        except Exception as e:
                            logging.error(f"Handler error: {e}")
            except Exception as e:
                # Silently ignore timeouts - this is normal for long polling
                if "timeout" in str(e).lower():
                    pass
                else:
                    logging.error(f"Poll error: {e}")


# Helper functions for building inline keyboards
def inline_keyboard(buttons):
    """
    Build an inline keyboard markup.

    Parameters:
        buttons (list of list of dict): 2D array of button definitions
            Each button: {"text": "Label", "callback_data": "value"}
            Or for URLs: {"text": "Label", "url": "https://..."}

    Returns:
        dict: Reply markup dict for use with send_message, etc.

    Example:
        keyboard = telegram.inline_keyboard([
            [{"text": "Yes", "callback_data": "yes"}, {"text": "No", "callback_data": "no"}]
        ])
        bot.send_message(chat_id, "Choose:", reply_markup=keyboard)
    """
    return {"inline_keyboard": buttons}


def reply_keyboard(buttons, resize=True, one_time=False):
    """
    Build a reply keyboard markup.

    Parameters:
        buttons (list of list of str): 2D array of button labels
        resize (bool): Resize keyboard to fit buttons
        one_time (bool): Hide keyboard after use

    Returns:
        dict: Reply markup dict

    Example:
        keyboard = telegram.reply_keyboard([["Option 1", "Option 2"]])
        bot.send_message(chat_id, "Choose:", reply_markup=keyboard)
    """
    keyboard = [[{"text": btn} for btn in row] for row in buttons]
    return {
        "keyboard": keyboard,
        "resize_keyboard": resize,
        "one_time_keyboard": one_time
    }


def remove_reply_keyboard():
    """
    Build a markup to remove the reply keyboard.

    Returns:
        dict: Reply markup dict
    """
    return {"remove_keyboard": True}
