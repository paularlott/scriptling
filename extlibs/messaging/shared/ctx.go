package shared

import "context"

// Ctx is passed to every handler. It wraps the Update and provides reply helpers
// so handlers don't need to manage dest/token themselves.
type Ctx struct {
	Update *Update
	Sender Sender
}

// Reply sends a text message back to the source of this update.
func (c *Ctx) Reply(goCtx context.Context, text string, opts *SendOptions) error {
	return c.Sender.SendMessage(goCtx, c.Update.Dest, text, opts)
}

// ReplyRich sends a rich message back to the source of this update.
func (c *Ctx) ReplyRich(goCtx context.Context, msg *RichMessage) error {
	return c.Sender.SendRichMessage(goCtx, c.Update.Dest, msg)
}

// Answer acknowledges a callback (button press).
// On Telegram: calls answerCallbackQuery (toast notification).
// On Discord: responds to the interaction (visible reply if text non-empty, silent ack if empty).
func (c *Ctx) Answer(goCtx context.Context, text string) error {
	return c.Sender.AckCallback(goCtx, c.Update.CallbackID, c.Update.CallbackToken, text)
}

// Typing sends a typing indicator.
func (c *Ctx) Typing(goCtx context.Context) error {
	return c.Sender.SendTyping(goCtx, c.Update.Dest)
}

// Download fetches the file in this update and returns raw bytes.
func (c *Ctx) Download(goCtx context.Context) ([]byte, error) {
	if c.Update.File != nil {
		ref := c.Update.File.URL
		if ref == "" {
			ref = c.Update.File.ID
		}
		return c.Sender.Download(goCtx, ref)
	}
	return nil, nil
}
