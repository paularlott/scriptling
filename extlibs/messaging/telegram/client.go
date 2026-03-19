package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
)

const apiBase = "https://api.telegram.org/bot"

// telegramClient implements shared.ScriptSender and embeds *shared.Bot for the routing table.
type telegramClient struct {
	*shared.Bot
	token      string
	httpClient *http.Client
	log        logger.Logger
}

func newClient(token string, log logger.Logger) *telegramClient {
	c := &telegramClient{
		token:      token,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		log:        log,
	}
	c.Bot = shared.NewBot(c)
	return c
}

// NewClient creates a Telegram client for direct Go use (no Scriptling required).
func NewClient(token string, log logger.Logger) *telegramClient {
	return newClient(token, log)
}

// Platform implements shared.Sender.
func (c *telegramClient) Platform() string { return "telegram" }

// Capabilities returns the list of features this Telegram client supports.
func (c *telegramClient) Capabilities() []string {
	return []string{
		"rich_message",
		"rich_message.title",
		"rich_message.body",
		"rich_message.image",
		"keyboard",
		"keyboard.callback",
		"keyboard.url",
		"typing",
		"edit_message",
		"delete_message",
		"send_file",
		"download",
	}
}

// BotCommand / BotOnCallback / BotOnMessage / BotOnFile / BotAuth / BotRun
// delegate to the embedded *shared.Bot.
func (c *telegramClient) BotCommand(name, helpText string, h shared.Handler) {
	c.Bot.Command(name, helpText, h)
}
func (c *telegramClient) BotOnCallback(prefix string, h shared.Handler) {
	c.Bot.OnCallback(prefix, h)
}
func (c *telegramClient) BotOnMessage(h shared.Handler) { c.Bot.OnMessage(h) }
func (c *telegramClient) BotOnFile(h shared.Handler)    { c.Bot.OnFile(h) }
func (c *telegramClient) BotAuth(h shared.Handler)      { c.Bot.Auth(h) }

// BotRun starts the HTTP long-polling loop and blocks until ctx is cancelled.
func (c *telegramClient) BotRun(ctx context.Context) error {
	c.log.Info("bot polling for updates")
	var offset int64
	for {
		if ctx.Err() != nil {
			return nil
		}
		updates, err := c.getUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
			}
			continue
		}
		for _, raw := range updates {
			u := parseUpdate(raw)
			offset = u.UpdateID + 1
			norm := normalise(u)
			if err := c.Bot.Dispatch(ctx, norm); err != nil {
				c.log.Error("dispatch error", "err", err)
			}
		}
	}
}

// normalise converts a rawUpdate to a shared.Update.
func normalise(u *rawUpdate) *shared.Update {
	dest := fmt.Sprintf("%d", u.ChatID)
	cmd, args := shared.ParseCommand(u.Text)
	n := &shared.Update{
		Dest:         dest,
		UserID:       u.UserID,
		UserName:     u.UserName,
		Text:         u.Text,
		Command:      cmd,
		Args:         args,
		IsCallback:   u.IsCallback,
		CallbackID:   u.CallbackID,
		CallbackData: u.CallbackData,
	}
	if u.File != nil {
		n.File = &shared.FileInfo{
			ID:       u.File.FileID,
			Name:     u.File.FileName,
			MimeType: u.File.MimeType,
			Size:     u.File.FileSize,
		}
	}
	return n
}

// ── shared.Sender implementation ─────────────────────────────────────────────

func (c *telegramClient) SendMessage(ctx context.Context, dest, text string, opts *shared.SendOptions) error {
	params := map[string]interface{}{"chat_id": dest, "text": text}
	if opts != nil {
		if opts.ParseMode != "" {
			params["parse_mode"] = opts.ParseMode
		}
		if opts.Keyboard != nil {
			rows := make([][]map[string]interface{}, 0, len(*opts.Keyboard))
			for _, row := range *opts.Keyboard {
				btns := make([]map[string]interface{}, 0, len(row))
				for _, b := range row {
					if b.URL != "" {
						btns = append(btns, map[string]interface{}{"text": b.Text, "url": b.URL})
					} else {
						btns = append(btns, map[string]interface{}{"text": b.Text, "callback_data": b.Data})
					}
				}
				rows = append(rows, btns)
			}
			params["reply_markup"] = map[string]interface{}{"inline_keyboard": rows}
		}
	}
	_, err := c.post(ctx, "sendMessage", params)
	return err
}

// SendRichMessage translates a RichMessage to Telegram:
// - Title + body combined as a single MarkdownV2 message (title bold, body plain)
// - Image sent as a separate photo with caption if present
// - Color and URL are not supported by Telegram and are silently ignored
func (c *telegramClient) SendRichMessage(ctx context.Context, dest string, msg *shared.RichMessage) error {
	// Build text: bold title + body
	var text string
	if msg.Title != "" && msg.Body != "" {
		text = "*" + escapeTelegramMarkdown(msg.Title) + "*\n" + msg.Body
	} else if msg.Title != "" {
		text = "*" + escapeTelegramMarkdown(msg.Title) + "*"
	} else {
		text = msg.Body
	}
	if msg.Image != "" {
		// Send image with text as caption
		data, isURL, fileID, err := resolveMedia(msg.Image, false)
		if err != nil {
			return err
		}
		params := map[string]interface{}{"chat_id": dest}
		if text != "" {
			params["caption"] = text
			params["parse_mode"] = "Markdown"
		}
		if isURL {
			params["photo"] = msg.Image
			_, err = c.post(ctx, "sendPhoto", params)
		} else if fileID != "" {
			params["photo"] = fileID
			_, err = c.post(ctx, "sendPhoto", params)
		} else {
			strFields := map[string]string{"chat_id": dest}
			if text != "" {
				strFields["caption"] = text
				strFields["parse_mode"] = "Markdown"
			}
			_, err = c.postMultipart(ctx, "sendPhoto", strFields, "photo", "photo.jpg", data)
		}
		return err
	}
	if text != "" {
		_, err := c.post(ctx, "sendMessage", map[string]interface{}{
			"chat_id":    dest,
			"text":       text,
			"parse_mode": "Markdown",
		})
		return err
	}
	return nil
}

// escapeTelegramMarkdown escapes special chars for Telegram Markdown (v1) bold.
func escapeTelegramMarkdown(s string) string {
	return strings.ReplaceAll(s, "*", "\\*")
}

func (c *telegramClient) EditMessage(ctx context.Context, dest, msgID, text string) error {
	_, err := c.post(ctx, "editMessageText", map[string]interface{}{
		"chat_id":    dest,
		"message_id": msgID,
		"text":       text,
	})
	return err
}

func (c *telegramClient) DeleteMessage(ctx context.Context, dest, msgID string) error {
	_, err := c.post(ctx, "deleteMessage", map[string]interface{}{
		"chat_id":    dest,
		"message_id": msgID,
	})
	return err
}

// isImageFile reports whether the file should be sent as a photo based on extension.
func isImageFile(fileName, source string) bool {
	name := fileName
	if name == "" {
		name = source
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	}
	return false
}

func (c *telegramClient) SendFile(ctx context.Context, dest, source, fileName, caption string, isB64 bool) error {
	if isImageFile(fileName, source) {
		_, err := c.sendPhoto(ctx, dest, source, caption, isB64)
		return err
	}
	_, err := c.sendDocument(ctx, dest, source, fileName, caption, isB64)
	return err
}

func (c *telegramClient) SendTyping(ctx context.Context, dest string) error {
	_, err := c.post(ctx, "sendChatAction", map[string]interface{}{
		"chat_id": dest,
		"action":  "typing",
	})
	if err != nil {
		c.log.Error("sendChatAction failed", "err", err)
	}
	return err
}

func (c *telegramClient) AckCallback(ctx context.Context, id, token, text string) error {
	// token is unused on Telegram
	params := map[string]interface{}{"callback_query_id": id}
	if text != "" {
		params["text"] = text
	}
	_, err := c.post(ctx, "answerCallbackQuery", params)
	return err
}

func (c *telegramClient) Download(ctx context.Context, ref string) ([]byte, error) {
	return c.downloadFile(ctx, ref)
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func (c *telegramClient) apiURL(method string) string {
	return apiBase + c.token + "/" + method
}

func (c *telegramClient) post(ctx context.Context, method string, params map[string]interface{}) (map[string]interface{}, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL(method), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if ok, _ := result["ok"].(bool); !ok {
		desc, _ := result["description"].(string)
		if desc == "" {
			desc = "telegram API error"
		}
		return nil, fmt.Errorf("%s: %s", method, desc)
	}
	return result, nil
}

func (c *telegramClient) postMultipart(ctx context.Context, method string, fields map[string]string, fileField, fileName string, fileData []byte) (map[string]interface{}, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	part, err := w.CreateFormFile(fileField, fileName)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, err
	}
	w.Close()
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL(method), &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func resolveMedia(source string, isBase64 bool) (data []byte, isURL bool, fileID string, err error) {
	if isBase64 {
		data, err = base64.StdEncoding.DecodeString(source)
		return
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return nil, true, "", nil
	}
	if _, statErr := os.Stat(source); statErr == nil {
		data, err = os.ReadFile(source)
		return
	}
	return nil, false, source, nil
}

func (c *telegramClient) sendDocument(ctx context.Context, dest, source, fileName, caption string, isBase64 bool) (map[string]interface{}, error) {
	data, isURL, fileID, err := resolveMedia(source, isBase64)
	if err != nil {
		return nil, err
	}
	if fileName == "" {
		fileName = "file"
	}
	if isURL {
		params := map[string]interface{}{"chat_id": dest, "document": source}
		if caption != "" {
			params["caption"] = caption
		}
		return c.post(ctx, "sendDocument", params)
	}
	if fileID != "" {
		params := map[string]interface{}{"chat_id": dest, "document": fileID}
		if caption != "" {
			params["caption"] = caption
		}
		return c.post(ctx, "sendDocument", params)
	}
	fields := map[string]string{"chat_id": dest}
	if caption != "" {
		fields["caption"] = caption
	}
	return c.postMultipart(ctx, "sendDocument", fields, "document", fileName, data)
}

func (c *telegramClient) sendPhoto(ctx context.Context, dest, source, caption string, isBase64 bool) (map[string]interface{}, error) {
	data, isURL, fileID, err := resolveMedia(source, isBase64)
	if err != nil {
		return nil, err
	}
	if isURL {
		params := map[string]interface{}{"chat_id": dest, "photo": source}
		if caption != "" {
			params["caption"] = caption
		}
		return c.post(ctx, "sendPhoto", params)
	}
	if fileID != "" {
		params := map[string]interface{}{"chat_id": dest, "photo": fileID}
		if caption != "" {
			params["caption"] = caption
		}
		return c.post(ctx, "sendPhoto", params)
	}
	fields := map[string]string{"chat_id": dest}
	if caption != "" {
		fields["caption"] = caption
	}
	return c.postMultipart(ctx, "sendPhoto", fields, "photo", "photo.jpg", data)
}

func (c *telegramClient) getFile(ctx context.Context, fileID string) (string, error) {
	result, err := c.post(ctx, "getFile", map[string]interface{}{"file_id": fileID})
	if err != nil {
		return "", err
	}
	ok, _ := result["ok"].(bool)
	if !ok {
		return "", fmt.Errorf("getFile failed")
	}
	res, _ := result["result"].(map[string]interface{})
	path, _ := res["file_path"].(string)
	return path, nil
}

func (c *telegramClient) downloadFile(ctx context.Context, fileID string) ([]byte, error) {
	path, err := c.getFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", c.token, path)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *telegramClient) getUpdates(ctx context.Context, offset int64, timeout int) ([]map[string]interface{}, error) {
	result, err := c.post(ctx, "getUpdates", map[string]interface{}{"timeout": timeout, "offset": offset})
	if err != nil {
		return nil, err
	}
	ok, _ := result["ok"].(bool)
	if !ok {
		return nil, fmt.Errorf("getUpdates failed")
	}
	raw, _ := result["result"].([]interface{})
	updates := make([]map[string]interface{}, 0, len(raw))
	for _, u := range raw {
		if m, ok := u.(map[string]interface{}); ok {
			updates = append(updates, m)
		}
	}
	return updates, nil
}

func parseUpdate(raw map[string]interface{}) *rawUpdate {
	u := &rawUpdate{}

	if id, ok := raw["update_id"].(float64); ok {
		u.UpdateID = int64(id)
	}

	if cb, ok := raw["callback_query"].(map[string]interface{}); ok {
		u.IsCallback = true
		u.CallbackID, _ = cb["id"].(string)
		u.CallbackData, _ = cb["data"].(string)
		if from, ok := cb["from"].(map[string]interface{}); ok {
			if id, ok := from["id"].(float64); ok {
				u.UserID = fmt.Sprintf("%d", int64(id))
			}
			u.UserName = buildName(from)
		}
		if msg, ok := cb["message"].(map[string]interface{}); ok {
			if chat, ok := msg["chat"].(map[string]interface{}); ok {
				if id, ok := chat["id"].(float64); ok {
					u.ChatID = int64(id)
				}
			}
		}
		return u
	}

	msg, ok := raw["message"].(map[string]interface{})
	if !ok {
		msg, ok = raw["edited_message"].(map[string]interface{})
	}
	if !ok {
		return u
	}

	if chat, ok := msg["chat"].(map[string]interface{}); ok {
		if id, ok := chat["id"].(float64); ok {
			u.ChatID = int64(id)
		}
	}
	if from, ok := msg["from"].(map[string]interface{}); ok {
		if id, ok := from["id"].(float64); ok {
			u.UserID = fmt.Sprintf("%d", int64(id))
		}
		u.UserName = buildName(from)
	}
	u.Text, _ = msg["text"].(string)

	if doc, ok := msg["document"].(map[string]interface{}); ok {
		fi := &fileInfo{}
		fi.FileID, _ = doc["file_id"].(string)
		fi.FileName, _ = doc["file_name"].(string)
		fi.MimeType, _ = doc["mime_type"].(string)
		if sz, ok := doc["file_size"].(float64); ok {
			fi.FileSize = int64(sz)
		}
		u.File = fi
	}

	return u
}

func buildName(from map[string]interface{}) string {
	var parts []string
	if fn, ok := from["first_name"].(string); ok && fn != "" {
		parts = append(parts, fn)
	}
	if ln, ok := from["last_name"].(string); ok && ln != "" {
		parts = append(parts, ln)
	}
	if len(parts) == 0 {
		if id, ok := from["id"].(float64); ok {
			return fmt.Sprintf("%d", int64(id))
		}
	}
	return strings.Join(parts, " ")
}
