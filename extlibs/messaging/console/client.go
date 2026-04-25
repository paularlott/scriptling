package console

import (
	"context"
	"fmt"
	"strings"

	"github.com/paularlott/cli/tui"
	scriptconsole "github.com/paularlott/scriptling/extlibs/console"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
)

const consoleDest = "console"

type commandEntry struct {
	handler  shared.Handler
	helpText string
}

type consoleClient struct {
	t        *tui.TUI
	inst     *consoleInstance
	commands map[string]commandEntry
	onMessage  shared.Handler
	onCallback shared.Handler
}

// consoleInstance is a thin shared.Sender adapter so BuildCtxDict has a sender reference.
type consoleInstance struct {
	c *consoleClient
}

func (s *consoleInstance) Platform() string     { return "console" }
func (s *consoleInstance) Capabilities() []string {
	return []string{"rich_message", "rich_message.title", "rich_message.body", "rich_message.color", "typing", "keyboard", "keyboard.callback"}
}
func (s *consoleInstance) SendMessage(_ context.Context, _, text string, opts *shared.SendOptions) error {
	s.c.t.StopSpinner()
	s.c.t.AddMessage(tui.RoleAssistant, text)
	if opts != nil && opts.Keyboard != nil {
		s.c.openKeyboardMenu(text, opts.Keyboard)
	}
	return nil
}
func (s *consoleInstance) SendRichMessage(_ context.Context, _ string, msg *shared.RichMessage) error {
	s.c.t.StopSpinner()
	var sb strings.Builder
	if msg.Body != "" {
		body := msg.Body
		if msg.Color != "" {
			body = tui.Styled(parseColor(msg.Color), body)
		}
		sb.WriteString(body)
	}
	if msg.URL != "" {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(msg.URL)
	}
	content := sb.String()
	if msg.Title != "" {
		s.c.t.AddMessageAs(tui.RoleAssistant, msg.Title, content)
	} else {
		s.c.t.AddMessage(tui.RoleAssistant, content)
	}
	return nil
}

// parseColor converts a color name or hex string to a tui.Color.
func parseColor(color string) tui.Color {
	named := map[string]tui.Color{
		"red":    0xED4245,
		"green":  0x57F287,
		"blue":   0x5865F2,
		"yellow": 0xFEE75C,
		"orange": 0xE67E22,
		"purple": 0x9B59B6,
		"grey":   0x95A5A6,
		"gray":   0x95A5A6,
		"white":  0xFFFFFF,
		"black":  0x000000,
	}
	if v, ok := named[strings.ToLower(color)]; ok {
		return v
	}
	s := strings.TrimPrefix(color, "#")
	if len(s) == 6 {
		var v uint32
		fmt.Sscanf(s, "%x", &v)
		return tui.Color(v)
	}
	return 0
}
func (s *consoleInstance) EditMessage(_ context.Context, _, _, _ string) error  { return nil }
func (s *consoleInstance) DeleteMessage(_ context.Context, _, _ string) error   { return nil }
func (s *consoleInstance) SendTyping(_ context.Context, _ string) error {
	s.c.t.StartSpinner("Thinking...")
	return nil
}
func (s *consoleInstance) AckCallback(_ context.Context, _, _, _ string) error  { return nil }
func (s *consoleInstance) Download(_ context.Context, _ string) ([]byte, error) { return nil, nil }
func (s *consoleInstance) SendFile(_ context.Context, _, _, _, _ string, _ bool) error {
	return fmt.Errorf("console: send_file not supported")
}

func newClient(t *tui.TUI) *consoleClient {
	c := &consoleClient{
		t:        t,
		commands: make(map[string]commandEntry),
	}
	c.inst = &consoleInstance{c: c}
	return c
}

func (c *consoleClient) Platform() string       { return "console" }
func (c *consoleClient) Capabilities() []string { return c.inst.Capabilities() }

func (c *consoleClient) BotCommand(name, helpText string, h shared.Handler) {
	c.commands[strings.ToLower(name)] = commandEntry{handler: h, helpText: helpText}
	cmdName := strings.TrimPrefix(strings.ToLower(name), "/")
	fullName := "/" + cmdName
	c.t.AddCommand(&tui.Command{
		Name:        cmdName,
		Description: helpText,
		Handler: func(args string) {
			text := fullName
			if args != "" {
				text += " " + args
			}
			c.dispatch(context.Background(), text)
		},
	})
}
func (c *consoleClient) BotOnCallback(_ string, h shared.Handler) { c.onCallback = h }
func (c *consoleClient) BotOnMessage(h shared.Handler)             { c.onMessage = h }
func (c *consoleClient) BotOnFile(_ shared.Handler)                {} // not applicable
func (c *consoleClient) BotAuth(_ shared.Handler)                  {} // not applicable

func (c *consoleClient) SendMessage(ctx context.Context, dest, text string, opts *shared.SendOptions) error {
	return c.inst.SendMessage(ctx, dest, text, opts)
}
func (c *consoleClient) SendRichMessage(ctx context.Context, dest string, msg *shared.RichMessage) error {
	return c.inst.SendRichMessage(ctx, dest, msg)
}
func (c *consoleClient) EditMessage(ctx context.Context, dest, msgID, text string) error {
	return c.inst.EditMessage(ctx, dest, msgID, text)
}
func (c *consoleClient) DeleteMessage(ctx context.Context, dest, msgID string) error {
	return c.inst.DeleteMessage(ctx, dest, msgID)
}
func (c *consoleClient) SendFile(ctx context.Context, dest, source, fileName, caption string, isB64 bool) error {
	return c.inst.SendFile(ctx, dest, source, fileName, caption, isB64)
}
func (c *consoleClient) SendTyping(ctx context.Context, dest string) error {
	return c.inst.SendTyping(ctx, dest)
}
func (c *consoleClient) AckCallback(ctx context.Context, id, token, text string) error {
	return c.inst.AckCallback(ctx, id, token, text)
}
func (c *consoleClient) Download(ctx context.Context, ref string) ([]byte, error) {
	return c.inst.Download(ctx, ref)
}

func (c *consoleClient) openKeyboardMenu(title string, kb *shared.Keyboard) {
	var items []*tui.MenuItem
	for _, row := range *kb {
		for _, btn := range row {
			data := btn.Data
			label := btn.Text
			if btn.URL != "" {
				label += " ↗"
				data = btn.URL
			}
			items = append(items, &tui.MenuItem{
				Label: label,
				Value: data,
				OnSelect: func(item *tui.MenuItem, _ string) {
					c.t.CloseMenu()
					c.dispatchCallback(item.Value)
				},
			})
		}
	}
	c.t.OpenMenu(&tui.Menu{Title: title, Items: items})
}

func (c *consoleClient) dispatchCallback(data string) {
	if c.onCallback == nil {
		return
	}
	u := &shared.Update{
		Dest:         consoleDest,
		UserID:       "user",
		UserName:     "user",
		IsCallback:   true,
		CallbackData: data,
	}
	cx := &shared.Ctx{Update: u, Sender: c.inst}
	_ = c.onCallback(context.Background(), cx)
}

func (c *consoleClient) helpText() string {
	var sb strings.Builder
	for name, e := range c.commands {
		sb.WriteString(name)
		if e.helpText != "" {
			sb.WriteString(" - ")
			sb.WriteString(e.helpText)
		}
		sb.WriteByte('\n')
	}
	sb.WriteString("/help - Show this message")
	return sb.String()
}

func (c *consoleClient) dispatch(ctx context.Context, text string) {
	cmd, args := shared.ParseCommand(text)
	u := &shared.Update{
		Dest:     consoleDest,
		UserID:   "user",
		UserName: "user",
		Text:     text,
		Command:  cmd,
		Args:     args,
	}
	cx := &shared.Ctx{Update: u, Sender: c.inst}

	if cmd == "/help" {
		_ = c.inst.SendMessage(ctx, consoleDest, c.helpText(), nil)
		return
	}
	if cmd != "" {
		if e, ok := c.commands[cmd]; ok {
			_ = e.handler(ctx, cx)
		} else {
			_ = c.inst.SendMessage(ctx, consoleDest, "Unknown command: "+cmd+"\nUse /help for available commands.", nil)
		}
		return
	}
	if c.onMessage != nil {
		_ = c.onMessage(ctx, cx)
	}
}

func (c *consoleClient) BotRun(ctx context.Context) error {
	c.t.AddCommand(&tui.Command{
		Name:        "help",
		Description: "Show available commands",
		Handler: func(_ string) {
			c.dispatch(context.Background(), "/help")
		},
	})
	scriptconsole.SetSubmit(func(submitCtx context.Context, text string) {
		c.dispatch(submitCtx, text)
	})
	return c.t.Run(ctx)
}
