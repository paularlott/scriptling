package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
)

// slackClient implements shared.ScriptSender and embeds *shared.Bot.
type slackClient struct {
	*shared.Bot
	botToken   string // xoxb-... — used for API calls
	appToken   string // xapp-... — used for Socket Mode
	httpClient *http.Client
	log        logger.Logger
	gateway    *socketGateway
}

func newClient(botToken, appToken string, log logger.Logger) *slackClient {
	c := &slackClient{
		botToken:   botToken,
		appToken:   appToken,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		log:        log,
	}
	c.Bot = shared.NewBot(c)
	c.gateway = newSocketGateway(c)
	return c
}

// NewClient creates a Slack client for direct Go use.
func NewClient(botToken, appToken string, log logger.Logger) *slackClient {
	return newClient(botToken, appToken, log)
}

func (c *slackClient) Platform() string { return "slack" }

func (c *slackClient) Capabilities() []string {
	return []string{
		"rich_message",
		"rich_message.title",
		"rich_message.body",
		"rich_message.color",
		"rich_message.image",
		"rich_message.url",
		"keyboard",
		"keyboard.callback",
		"keyboard.url",
		"edit_message",
		"delete_message",
		"send_file",
		"download",
	}
}

func (c *slackClient) BotCommand(name, helpText string, h shared.Handler) {
	c.Bot.Command(name, helpText, h)
}
func (c *slackClient) BotOnCallback(prefix string, h shared.Handler) {
	c.Bot.OnCallback(prefix, h)
}
func (c *slackClient) BotOnMessage(h shared.Handler) { c.Bot.OnMessage(h) }
func (c *slackClient) BotOnFile(h shared.Handler)    { c.Bot.OnFile(h) }
func (c *slackClient) BotAuth(h shared.Handler)      { c.Bot.Auth(h) }

func (c *slackClient) BotRun(ctx context.Context) error {
	c.log.Info("connecting to Slack Socket Mode gateway")
	updates := make(chan *rawUpdate, 64)
	c.gateway.updates = updates

	gwCtx, gwCancel := context.WithCancel(ctx)
	defer gwCancel()
	go c.gateway.connect(gwCtx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case u, ok := <-updates:
			if !ok {
				return nil
			}
			norm := normalise(c, u)
			if err := c.Bot.Dispatch(ctx, norm); err != nil {
				c.log.Error("dispatch error", "err", err)
			}
		}
	}
}

func normalise(c *slackClient, u *rawUpdate) *shared.Update {
	// Resolve display name if we only have a user ID
	userName := u.UserName
	if userName == "" && u.UserID != "" {
		userName = c.resolveUserName(u.UserID)
	}
	cmd, args := shared.ParseCommand(u.Text)
	n := &shared.Update{
		Dest:          u.ChannelID,
		MessageID:     u.MessageTS,
		UserID:        u.UserID,
		UserName:      userName,
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

func (c *slackClient) SendMessage(ctx context.Context, dest, text string, opts *shared.SendOptions) error {
	body := map[string]interface{}{
		"channel": dest,
		"text":    text,
	}
	if opts != nil && opts.Keyboard != nil {
		body["blocks"] = keyboardToBlocks(text, opts.Keyboard)
		// When using blocks, text is the fallback for notifications
	}
	_, err := c.api(ctx, "chat.postMessage", body)
	return err
}

// SendRichMessage translates a RichMessage to a Slack attachment.
// Slack attachments support color, title, text, image_url, and title_link natively.
func (c *slackClient) SendRichMessage(ctx context.Context, dest string, msg *shared.RichMessage) error {
	attachment := map[string]interface{}{}
	if msg.Color != "" {
		attachment["color"] = normaliseSlackColor(msg.Color)
	}
	if msg.Title != "" {
		attachment["title"] = msg.Title
	}
	if msg.URL != "" {
		attachment["title_link"] = msg.URL
	}
	if msg.Body != "" {
		attachment["text"] = msg.Body
	}
	if msg.Image != "" {
		attachment["image_url"] = msg.Image
	}
	_, err := c.api(ctx, "chat.postMessage", map[string]interface{}{
		"channel":     dest,
		"text":        msg.Title, // fallback notification text
		"attachments": []interface{}{attachment},
	})
	return err
}

// normaliseSlackColor converts named colors or hex to Slack attachment color format.
func normaliseSlackColor(color string) string {
	named := map[string]string{
		"red":     "#ED4245",
		"green":   "#57F287",
		"blue":    "#5865F2",
		"yellow":  "#FEE75C",
		"orange":  "#E67E22",
		"purple":  "#9B59B6",
		"grey":    "#95A5A6",
		"gray":    "#95A5A6",
		"good":    "good",
		"warning": "warning",
		"danger":  "danger",
	}
	if v, ok := named[strings.ToLower(color)]; ok {
		return v
	}
	if strings.HasPrefix(color, "#") {
		return color
	}
	return "#" + color
}

// keyboardToBlocks converts a Keyboard to Slack Block Kit button sections.
func keyboardToBlocks(text string, kb *shared.Keyboard) []interface{} {
	blocks := []interface{}{}
	if text != "" {
		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{"type": "mrkdwn", "text": text},
		})
	}
	for _, row := range *kb {
		elements := make([]interface{}, 0, len(row))
		for _, b := range row {
			if b.URL != "" {
				elements = append(elements, map[string]interface{}{
					"type": "button",
					"text": map[string]interface{}{"type": "plain_text", "text": b.Text},
					"url":  b.URL,
				})
			} else {
				elements = append(elements, map[string]interface{}{
					"type":      "button",
					"text":      map[string]interface{}{"type": "plain_text", "text": b.Text},
					"action_id": b.Data,
					"value":     b.Data,
				})
			}
		}
		if len(elements) > 0 {
			blocks = append(blocks, map[string]interface{}{
				"type":     "actions",
				"elements": elements,
			})
		}
	}
	return blocks
}

func (c *slackClient) EditMessage(ctx context.Context, dest, msgID, text string) error {
	_, err := c.api(ctx, "chat.update", map[string]interface{}{
		"channel": dest,
		"ts":      msgID,
		"text":    text,
	})
	return err
}

func (c *slackClient) DeleteMessage(ctx context.Context, dest, msgID string) error {
	_, err := c.api(ctx, "chat.delete", map[string]interface{}{
		"channel": dest,
		"ts":      msgID,
	})
	return err
}

func (c *slackClient) SendFile(ctx context.Context, dest, source, fileName, caption string, isB64 bool) error {
	return c.uploadFile(ctx, dest, source, fileName, caption, isB64)
}

func (c *slackClient) SendTyping(ctx context.Context, dest string) error {
	// Slack doesn't have a typing indicator API for bots — silently ignore
	return nil
}

func (c *slackClient) AckCallback(ctx context.Context, id, token, text string) error {
	// token is the response_url for Slack interactions
	if token == "" {
		return nil
	}
	body := map[string]interface{}{}
	if text != "" {
		body["text"] = text
	} else {
		body["delete_original"] = false
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", token, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *slackClient) Download(ctx context.Context, ref string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", ref, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.botToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func (c *slackClient) api(ctx context.Context, method string, body interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", apiBase+method, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.botToken)
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
		errStr, _ := result["error"].(string)
		if errStr == "" {
			errStr = "slack API error"
		}
		return nil, fmt.Errorf("%s: %s", method, errStr)
	}
	return result, nil
}

// openSocketConnection calls apps.connections.open to get a WSS URL.
func (c *slackClient) openSocketConnection(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", apiBase+"apps.connections.open", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.appToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if ok, _ := result["ok"].(bool); !ok {
		errStr, _ := result["error"].(string)
		return "", fmt.Errorf("apps.connections.open: %s", errStr)
	}
	url, _ := result["url"].(string)
	return url, nil
}

// resolveUserName fetches the display name for a user ID via users.info.
func (c *slackClient) resolveUserName(userID string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET",
		apiBase+"users.info?user="+userID, nil)
	if err != nil {
		return userID
	}
	req.Header.Set("Authorization", "Bearer "+c.botToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return userID
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return userID
	}
	if user, ok := result["user"].(map[string]interface{}); ok {
		if profile, ok := user["profile"].(map[string]interface{}); ok {
			if name, ok := profile["display_name"].(string); ok && name != "" {
				return name
			}
			if name, ok := profile["real_name"].(string); ok && name != "" {
				return name
			}
		}
	}
	return userID
}

// openDM calls conversations.open to get or create a DM channel with a user.
// Returns the channel ID to use as dest for SendMessage.
func (c *slackClient) openDM(ctx context.Context, userID string) (string, error) {
	result, err := c.api(ctx, "conversations.open", map[string]interface{}{"users": userID})
	if err != nil {
		return "", err
	}
	ch, _ := result["channel"].(map[string]interface{})
	id, _ := ch["id"].(string)
	if id == "" {
		return "", fmt.Errorf("conversations.open: no channel id returned")
	}
	return id, nil
}

func (c *slackClient) uploadFile(ctx context.Context, channelID, source, fileName, caption string, isBase64 bool) error {
	var data []byte
	var isURL bool

	if isBase64 {
		var err error
		data, err = decodeBase64(source)
		if err != nil {
			return err
		}
	} else if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		isURL = true
	} else {
		var err error
		data, err = readFile(source)
		if err != nil {
			return err
		}
	}

	if isURL {
		// For URLs, post as a message with the URL
		text := source
		if caption != "" {
			text = caption + "\n" + source
		}
		return c.SendMessage(ctx, channelID, text, nil)
	}

	if fileName == "" {
		fileName = "file"
	}

	// Use files.getUploadURLExternal + files.completeUploadExternal (new API)
	// Step 1: get upload URL
	uploadReq, err := http.NewRequestWithContext(ctx, "POST",
		apiBase+"files.getUploadURLExternal",
		strings.NewReader(fmt.Sprintf("filename=%s&length=%d", fileName, len(data))),
	)
	if err != nil {
		return err
	}
	uploadReq.Header.Set("Authorization", "Bearer "+c.botToken)
	uploadReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	uploadResp, err := c.httpClient.Do(uploadReq)
	if err != nil {
		return err
	}
	defer uploadResp.Body.Close()
	var uploadResult map[string]interface{}
	json.NewDecoder(uploadResp.Body).Decode(&uploadResult)
	if ok, _ := uploadResult["ok"].(bool); !ok {
		errStr, _ := uploadResult["error"].(string)
		return fmt.Errorf("files.getUploadURLExternal: %s", errStr)
	}
	uploadURL, _ := uploadResult["upload_url"].(string)
	fileID, _ := uploadResult["file_id"].(string)

	// Step 2: upload file bytes
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return err
	}
	fw.Write(data)
	w.Close()
	putReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return err
	}
	putReq.Header.Set("Content-Type", w.FormDataContentType())
	putResp, err := c.httpClient.Do(putReq)
	if err != nil {
		return err
	}
	putResp.Body.Close()

	// Step 3: complete upload
	completeBody := map[string]interface{}{
		"files":      []interface{}{map[string]interface{}{"id": fileID}},
		"channel_id": channelID,
	}
	if caption != "" {
		completeBody["initial_comment"] = caption
	}
	_, err = c.api(ctx, "files.completeUploadExternal", completeBody)
	return err
}
