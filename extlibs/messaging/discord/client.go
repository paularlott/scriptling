package discord

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
	"strings"
	"time"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
)

// discordClient implements shared.ScriptSender and embeds *shared.Bot.
type discordClient struct {
	*shared.Bot
	token      string
	httpClient *http.Client
	botUserID  string
	log        logger.Logger
	gateway    *gateway
}

func newClient(token string, log logger.Logger) *discordClient {
	c := &discordClient{
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		log:        log,
	}
	c.Bot = shared.NewBot(c)
	c.gateway = newGateway(c)
	return c
}

// NewClient creates a Discord client for direct Go use (no Scriptling required).
func NewClient(token string, log logger.Logger) *discordClient {
	return newClient(token, log)
}

// Platform implements shared.Sender.
func (c *discordClient) Platform() string { return "discord" }

// Capabilities returns the list of features this Discord client supports.
func (c *discordClient) Capabilities() []string {
	return []string{
		"rich_message",
		"rich_message.title",
		"rich_message.body",
		"rich_message.image",
		"rich_message.color",
		"rich_message.url",
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

func (c *discordClient) BotCommand(name, helpText string, h shared.Handler) {
	c.Bot.Command(name, helpText, h)
}
func (c *discordClient) BotOnCallback(prefix string, h shared.Handler) {
	c.Bot.OnCallback(prefix, h)
}
func (c *discordClient) BotOnMessage(h shared.Handler) { c.Bot.OnMessage(h) }
func (c *discordClient) BotOnFile(h shared.Handler)    { c.Bot.OnFile(h) }
func (c *discordClient) BotAuth(h shared.Handler)      { c.Bot.Auth(h) }

// BotRun starts the WebSocket gateway and blocks until ctx is cancelled.
func (c *discordClient) BotRun(ctx context.Context) error {
	c.log.Info("connecting to Discord gateway")
	gwCtx, gwCancel := context.WithCancel(ctx)
	defer gwCancel()

	updates := make(chan *rawUpdate, 64)
	c.gateway.updates = updates
	go c.gateway.connect(gwCtx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case u, ok := <-updates:
			if !ok {
				return nil
			}
			norm := normalise(u)
			if err := c.Bot.Dispatch(ctx, norm); err != nil {
				c.log.Error("dispatch error", "err", err)
			}
		}
	}
}

// normalise converts a rawUpdate to a shared.Update.
func normalise(u *rawUpdate) *shared.Update {
	cmd, args := shared.ParseCommand(u.Text)
	n := &shared.Update{
		Dest:          u.ChannelID,
		MessageID:     u.MessageID,
		UserID:        u.UserID,
		UserName:      u.UserName,
		Text:          u.Text,
		Command:       cmd,
		Args:          args,
		IsCallback:    u.IsCallback,
		CallbackID:    u.CallbackID,
		CallbackToken: u.CallbackToken,
		CallbackData:  u.CallbackData,
	}
	if u.File != nil {
		n.File = &shared.FileInfo{
			ID:       u.File.ID,
			Name:     u.File.Name,
			MimeType: u.File.MimeType,
			Size:     u.File.Size,
			URL:      u.File.URL,
		}
	}
	return n
}

// ── shared.Sender implementation ─────────────────────────────────────────────

func (c *discordClient) SendMessage(ctx context.Context, dest, text string, opts *shared.SendOptions) error {
	body := map[string]interface{}{"content": text}
	if opts != nil && opts.Keyboard != nil {
		btns := make([]map[string]interface{}, 0)
		for _, row := range *opts.Keyboard {
			for _, b := range row {
				if b.URL != "" {
					btns = append(btns, map[string]interface{}{"type": 2, "style": 5, "label": b.Text, "url": b.URL})
				} else {
					btns = append(btns, map[string]interface{}{"type": 2, "style": 1, "label": b.Text, "custom_id": b.Data})
				}
			}
		}
		body["components"] = []map[string]interface{}{{"type": 1, "components": btns}}
	}
	_, err := c.request(ctx, "POST", "/channels/"+dest+"/messages", body)
	return err
}

// SendRichMessage translates a RichMessage to a Discord embed.
// Title, body (description), color, image, and url are all supported natively.
func (c *discordClient) SendRichMessage(ctx context.Context, dest string, msg *shared.RichMessage) error {
	embed := map[string]interface{}{}
	if msg.Title != "" {
		embed["title"] = msg.Title
	}
	if msg.Body != "" {
		embed["description"] = msg.Body
	}
	if msg.URL != "" {
		embed["url"] = msg.URL
	}
	if msg.Color != "" {
		embed["color"] = parseDiscordColor(msg.Color)
	}
	if msg.Image != "" {
		embed["image"] = map[string]interface{}{"url": msg.Image}
	}
	_, err := c.request(ctx, "POST", "/channels/"+dest+"/messages",
		map[string]interface{}{"embeds": []interface{}{embed}})
	return err
}

// parseDiscordColor converts a color name or hex string to a Discord integer color.
func parseDiscordColor(color string) int {
	named := map[string]int{
		"red":    0xED4245,
		"green":  0x57F287,
		"blue":   0x5865F2,
		"yellow": 0xFEE75C,
		"orange": 0xE67E22,
		"purple": 0x9B59B6,
		"grey":   0x95A5A6,
		"gray":   0x95A5A6,
		"white":  0xFFFFFE,
		"black":  0x000000,
	}
	if v, ok := named[strings.ToLower(color)]; ok {
		return v
	}
	// Parse hex: "#ff0000" or "ff0000"
	s := strings.TrimPrefix(color, "#")
	if len(s) == 6 {
		var v int
		fmt.Sscanf(s, "%x", &v)
		return v
	}
	return 0
}

func (c *discordClient) EditMessage(ctx context.Context, dest, msgID, text string) error {
	_, err := c.request(ctx, "PATCH", "/channels/"+dest+"/messages/"+msgID,
		map[string]interface{}{"content": text})
	return err
}

func (c *discordClient) DeleteMessage(ctx context.Context, dest, msgID string) error {
	_, err := c.request(ctx, "DELETE", "/channels/"+dest+"/messages/"+msgID, nil)
	return err
}

func (c *discordClient) SendFile(ctx context.Context, dest, source, fileName, caption string, isB64 bool) error {
	_, err := c.sendFile(ctx, dest, source, fileName, caption, isB64)
	return err
}

func (c *discordClient) SendTyping(ctx context.Context, dest string) error {
	_, err := c.request(ctx, "POST", "/channels/"+dest+"/typing", nil)
	return err
}

func (c *discordClient) AckCallback(ctx context.Context, id, token, text string) error {
	if text == "" {
		return c.ackInteraction(ctx, id, token)
	}
	return c.respondToInteraction(ctx, id, token, text)
}

func (c *discordClient) Download(ctx context.Context, ref string) ([]byte, error) {
	return c.downloadAttachment(ctx, ref)
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func (c *discordClient) authHeader() string { return "Bot " + c.token }

func (c *discordClient) request(ctx context.Context, method, path string, body interface{}) (map[string]interface{}, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 204 {
		return nil, nil
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if code, ok := result["code"].(float64); ok && code != 0 {
		msg, _ := result["message"].(string)
		return nil, fmt.Errorf("discord API error %d: %s", int(code), msg)
	}
	return result, nil
}

func (c *discordClient) respondToInteraction(ctx context.Context, id, token, content string) error {
	body := map[string]interface{}{
		"type": 4,
		"data": map[string]interface{}{"content": content},
	}
	url := apiBase + "/interactions/" + id + "/" + token + "/callback"
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *discordClient) ackInteraction(ctx context.Context, id, token string) error {
	body := map[string]interface{}{"type": 6}
	url := apiBase + "/interactions/" + id + "/" + token + "/callback"
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func resolveMedia(source string, isBase64 bool) (data []byte, isURL bool, err error) {
	if isBase64 {
		data, err = base64.StdEncoding.DecodeString(source)
		return
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return nil, true, nil
	}
	if _, statErr := os.Stat(source); statErr == nil {
		data, err = os.ReadFile(source)
		return
	}
	return nil, false, fmt.Errorf("discord: cannot resolve media source %q", source)
}

func (c *discordClient) sendFile(ctx context.Context, channelID, source, fileName, content string, isBase64 bool) (map[string]interface{}, error) {
	data, isURL, err := resolveMedia(source, isBase64)
	if err != nil {
		return nil, err
	}
	if isURL {
		msg := source
		if content != "" {
			msg = content + "\n" + source
		}
		return nil, c.SendMessage(ctx, channelID, msg, nil)
	}
	if fileName == "" {
		fileName = "file"
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	payload, _ := json.Marshal(map[string]interface{}{"content": content})
	pw, _ := w.CreateFormField("payload_json")
	pw.Write(payload)
	fw, err := w.CreateFormFile("files[0]", fileName)
	if err != nil {
		return nil, err
	}
	fw.Write(data)
	w.Close()
	req, err := http.NewRequestWithContext(ctx, "POST", apiBase+"/channels/"+channelID+"/messages", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (c *discordClient) downloadAttachment(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *discordClient) getGatewayURL(ctx context.Context) (string, error) {
	result, err := c.request(ctx, "GET", "/gateway/bot", nil)
	if err != nil {
		return gatewayURL, nil
	}
	if u, ok := result["url"].(string); ok {
		return u + "?v=10&encoding=json", nil
	}
	return gatewayURL, nil
}

func parseEvent(eventType string, data map[string]interface{}) *rawUpdate {
	u := &rawUpdate{}
	switch eventType {
	case "MESSAGE_CREATE":
		u.MessageID, _ = data["id"].(string)
		u.ChannelID, _ = data["channel_id"].(string)
		u.Text, _ = data["content"].(string)
		if author, ok := data["author"].(map[string]interface{}); ok {
			u.UserID, _ = author["id"].(string)
			u.UserName = buildName(author)
		}
		if atts, ok := data["attachments"].([]interface{}); ok {
			for _, a := range atts {
				if am, ok := a.(map[string]interface{}); ok {
					fi := fileInfo{}
					fi.ID, _ = am["id"].(string)
					fi.Name, _ = am["filename"].(string)
					fi.URL, _ = am["url"].(string)
					fi.MimeType, _ = am["content_type"].(string)
					if sz, ok := am["size"].(float64); ok {
						fi.Size = int64(sz)
					}
					u.Attachments = append(u.Attachments, fi)
				}
			}
		}
		if len(u.Attachments) > 0 {
			u.File = &u.Attachments[0]
		}
	case "INTERACTION_CREATE":
		u.IsCallback = true
		u.CallbackID, _ = data["id"].(string)
		u.CallbackToken, _ = data["token"].(string)
		u.ChannelID, _ = data["channel_id"].(string)
		if member, ok := data["member"].(map[string]interface{}); ok {
			if usr, ok := member["user"].(map[string]interface{}); ok {
				u.UserID, _ = usr["id"].(string)
				u.UserName = buildName(usr)
			}
		}
		if usr, ok := data["user"].(map[string]interface{}); ok {
			u.UserID, _ = usr["id"].(string)
			u.UserName = buildName(usr)
		}
		if d, ok := data["data"].(map[string]interface{}); ok {
			u.CallbackData, _ = d["custom_id"].(string)
		}
	}
	return u
}

func buildName(user map[string]interface{}) string {
	if gn, ok := user["global_name"].(string); ok && gn != "" {
		return gn
	}
	if un, ok := user["username"].(string); ok && un != "" {
		return un
	}
	if id, ok := user["id"].(string); ok {
		return id
	}
	return ""
}
