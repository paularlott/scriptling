package console

import (
	"context"

	scriptconsole "github.com/paularlott/scriptling/extlibs/console"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
	"github.com/paularlott/scriptling/object"
)

const nativeClientKey = "__con_client__"

type clientWrapper struct {
	c *consoleClient
}

func (w *clientWrapper) Type() object.ObjectType                           { return object.BUILTIN_OBJ }
func (w *clientWrapper) Inspect() string                                   { return "<ConsoleClient>" }
func (w *clientWrapper) AsString() (string, object.Object)                 { return "<ConsoleClient>", nil }
func (w *clientWrapper) AsInt() (int64, object.Object)                     { return 0, nil }
func (w *clientWrapper) AsFloat() (float64, object.Object)                 { return 0, nil }
func (w *clientWrapper) AsBool() (bool, object.Object)                     { return true, nil }
func (w *clientWrapper) AsList() ([]object.Object, object.Object)          { return nil, nil }
func (w *clientWrapper) AsDict() (map[string]object.Object, object.Object) { return nil, nil }
func (w *clientWrapper) CoerceString() (string, object.Object)             { return "<ConsoleClient>", nil }
func (w *clientWrapper) CoerceInt() (int64, object.Object)                 { return 0, nil }
func (w *clientWrapper) CoerceFloat() (float64, object.Object)             { return 0, nil }

func (w *clientWrapper) Platform() string          { return w.c.Platform() }
func (w *clientWrapper) Capabilities() []string    { return w.c.Capabilities() }
func (w *clientWrapper) BotCapabilities() []string { return w.c.Capabilities() }
func (w *clientWrapper) SendMessage(ctx context.Context, dest, text string, opts *shared.SendOptions) error {
	return w.c.SendMessage(ctx, dest, text, opts)
}
func (w *clientWrapper) SendRichMessage(ctx context.Context, dest string, msg *shared.RichMessage) error {
	return w.c.SendRichMessage(ctx, dest, msg)
}
func (w *clientWrapper) EditMessage(ctx context.Context, dest, msgID, text string) error {
	return w.c.EditMessage(ctx, dest, msgID, text)
}
func (w *clientWrapper) DeleteMessage(ctx context.Context, dest, msgID string) error {
	return w.c.DeleteMessage(ctx, dest, msgID)
}
func (w *clientWrapper) SendFile(ctx context.Context, dest, source, fileName, caption string, isB64 bool) error {
	return w.c.SendFile(ctx, dest, source, fileName, caption, isB64)
}
func (w *clientWrapper) SendTyping(ctx context.Context, dest string) error {
	return w.c.SendTyping(ctx, dest)
}
func (w *clientWrapper) AckCallback(ctx context.Context, id, token, text string) error {
	return w.c.AckCallback(ctx, id, token, text)
}
func (w *clientWrapper) Download(ctx context.Context, ref string) ([]byte, error) {
	return w.c.Download(ctx, ref)
}
func (w *clientWrapper) BotCommand(name, helpText string, h shared.Handler) {
	w.c.BotCommand(name, helpText, h)
}
func (w *clientWrapper) BotOnCallback(prefix string, h shared.Handler) {
	w.c.BotOnCallback(prefix, h)
}
func (w *clientWrapper) BotOnMessage(h shared.Handler) { w.c.BotOnMessage(h) }
func (w *clientWrapper) BotOnFile(h shared.Handler)    { w.c.BotOnFile(h) }
func (w *clientWrapper) BotAuth(h shared.Handler)      { w.c.BotAuth(h) }
func (w *clientWrapper) BotRun(ctx context.Context) error {
	return w.c.BotRun(ctx)
}

var consoleClientClass = &object.Class{
	Name:    "ConsoleClient",
	Methods: map[string]object.Object{},
}

func newClientInstance(c *consoleClient, builtins map[string]*object.Builtin) *object.Instance {
	inst := &object.Instance{
		Class:  consoleClientClass,
		Fields: map[string]object.Object{},
	}
	inst.Fields[nativeClientKey] = &clientWrapper{c: c}
	shared.BindToInstance(inst, builtins)
	return inst
}

// NewLibrary creates the scriptling.messaging.console library.
func NewLibrary() *object.Library {
	builtins := shared.SharedBuiltins(nativeClientKey)

	builtins["client"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			t := scriptconsole.TUI()
			c := newClient(t)
			return newClientInstance(c, builtins)
		},
		HelpText: `client() - Create a console messaging bot client`,
	}

	return object.NewLibrary(LibraryName, builtins, map[string]object.Object{
		"ConsoleClient": consoleClientClass,
	}, "Console messaging bot client")
}

// Register registers the messaging console library with a Scriptling instance.
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(NewLibrary())
}
