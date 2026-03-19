package shared

import (
	"context"
	"strings"
)

// Sender is the interface the shared bot calls back into the platform client for.
type Sender interface {
	SendMessage(ctx context.Context, dest, text string, opts *SendOptions) error
	SendRichMessage(ctx context.Context, dest string, msg *RichMessage) error
	EditMessage(ctx context.Context, dest, msgID, text string) error
	DeleteMessage(ctx context.Context, dest, msgID string) error
	SendFile(ctx context.Context, dest, source, fileName, caption string, isB64 bool) error
	SendTyping(ctx context.Context, dest string) error
	AckCallback(ctx context.Context, id, token, text string) error
	Download(ctx context.Context, ref string) ([]byte, error)
	Platform() string
	Capabilities() []string
}

// SendOptions carries optional send parameters.
type SendOptions struct {
	ParseMode string
	Keyboard  *Keyboard
}

// Handler is a Go-level update handler.
type Handler func(ctx context.Context, c *Ctx) error

type commandEntry struct {
	handler  Handler
	helpText string
}

type callbackEntry struct {
	prefix  string
	handler Handler
}

// Bot holds the routing table and dispatch logic shared across all platform clients.
// Platform clients embed *Bot and call dispatch after normalising a raw event.
type Bot struct {
	sender      Sender
	commands    map[string]commandEntry
	callbacks   []callbackEntry
	onMessage   Handler
	onFile      Handler
	authHandler Handler
}

// NewBot creates a Bot bound to a Sender.
func NewBot(s Sender) *Bot {
	return &Bot{
		sender:   s,
		commands: make(map[string]commandEntry),
	}
}

// Command registers a command handler. name must start with "/".
func (b *Bot) Command(name, helpText string, h Handler) {
	b.commands[strings.ToLower(name)] = commandEntry{handler: h, helpText: helpText}
}

// OnCallback registers a handler for callback (button) events whose data has the given prefix.
// Use "" to match all callbacks.
func (b *Bot) OnCallback(prefix string, h Handler) {
	b.callbacks = append(b.callbacks, callbackEntry{prefix: prefix, handler: h})
}

// OnMessage registers the default handler for plain (non-command) text messages.
func (b *Bot) OnMessage(h Handler) { b.onMessage = h }

// OnFile registers a handler for file attachments.
func (b *Bot) OnFile(h Handler) { b.onFile = h }

// Auth registers an auth handler. Return nil error to allow, non-nil to deny.
func (b *Bot) Auth(h Handler) { b.authHandler = h }

// HelpText returns a formatted list of registered commands.
func (b *Bot) HelpText() string {
	if len(b.commands) == 0 {
		return "No commands registered."
	}
	var sb strings.Builder
	for name, e := range b.commands {
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

// Dispatch routes a normalised Update to the appropriate registered handler.
// Called by the platform client after normalising a raw event.
func (b *Bot) Dispatch(goCtx context.Context, u *Update) error {
	c := &Ctx{Update: u, Sender: b.sender}

	// Auth check
	if b.authHandler != nil {
		if err := b.authHandler(goCtx, c); err != nil {
			return nil // denied — silently drop
		}
	}

	// Callback (button press)
	if u.IsCallback {
		for _, e := range b.callbacks {
			if e.prefix == "" || strings.HasPrefix(u.CallbackData, e.prefix) {
				return e.handler(goCtx, c)
			}
		}
		return nil
	}

	// File
	if u.File != nil && b.onFile != nil {
		return b.onFile(goCtx, c)
	}

	// Command
	if u.Command != "" {
		if u.Command == "/help" {
			return b.sender.SendMessage(goCtx, u.Dest, b.HelpText(), nil)
		}
		if e, ok := b.commands[u.Command]; ok {
			return e.handler(goCtx, c)
		}
		return b.sender.SendMessage(goCtx, u.Dest,
			"Unknown command: "+u.Command+"\nUse /help for available commands.", nil)
	}

	// Default message
	if b.onMessage != nil {
		return b.onMessage(goCtx, c)
	}
	return nil
}

// ParseCommand splits text into command + args if it starts with "/".
func ParseCommand(text string) (cmd string, args []string) {
	if !strings.HasPrefix(text, "/") {
		return "", nil
	}
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", nil
	}
	// Strip @botname suffix if present (e.g. /start@mybot)
	cmd = strings.ToLower(strings.SplitN(parts[0], "@", 2)[0])
	if len(parts) > 1 {
		args = parts[1:]
	}
	return cmd, args
}
