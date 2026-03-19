package slack

import (
	"context"
	"fmt"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
	"github.com/paularlott/scriptling/object"
)

const nativeClientKey = "__sl_client__"

type clientWrapper struct {
	c *slackClient
}

func (w *clientWrapper) Type() object.ObjectType                           { return object.BUILTIN_OBJ }
func (w *clientWrapper) Inspect() string                                   { return "<SlackClient>" }
func (w *clientWrapper) AsString() (string, object.Object)                 { return "<SlackClient>", nil }
func (w *clientWrapper) AsInt() (int64, object.Object)                     { return 0, nil }
func (w *clientWrapper) AsFloat() (float64, object.Object)                 { return 0, nil }
func (w *clientWrapper) AsBool() (bool, object.Object)                     { return true, nil }
func (w *clientWrapper) AsList() ([]object.Object, object.Object)          { return nil, nil }
func (w *clientWrapper) AsDict() (map[string]object.Object, object.Object) { return nil, nil }
func (w *clientWrapper) CoerceString() (string, object.Object)             { return "<SlackClient>", nil }
func (w *clientWrapper) CoerceInt() (int64, object.Object)                 { return 0, nil }
func (w *clientWrapper) CoerceFloat() (float64, object.Object)             { return 0, nil }

func (w *clientWrapper) Platform() string { return w.c.Platform() }
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

var slackClientClass = &object.Class{
	Name:    "SlackClient",
	Methods: map[string]object.Object{},
}

func newClientInstance(c *slackClient, builtins map[string]*object.Builtin) *object.Instance {
	inst := &object.Instance{
		Class:  slackClientClass,
		Fields: map[string]object.Object{},
	}
	inst.Fields[nativeClientKey] = &clientWrapper{c: c}
	shared.BindToInstance(inst, builtins)
	return inst
}

// NewLibrary creates the scriptling.messaging.slack library.
func NewLibrary(log logger.Logger) *object.Library {
	builtins := shared.SharedBuiltins(nativeClientKey)

	builtins["client"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}
			botToken, errObj := args[0].AsString()
			if errObj != nil {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			appToken, errObj := args[1].AsString()
			if errObj != nil {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}
			if botToken == "" {
				return errors.NewError("slack.client: bot_token must not be empty")
			}
			if appToken == "" {
				return errors.NewError("slack.client: app_token must not be empty")
			}
			c := newClient(botToken, appToken, log)
			inst := newClientInstance(c, builtins)
			if rawList, errObj := kwargs.GetList("allowed_users", nil); errObj == nil && rawList != nil && len(rawList) > 0 {
				allowed := make(map[string]bool, len(rawList))
				for _, item := range rawList {
					if s, err := item.CoerceString(); err == nil {
						allowed[s] = true
					}
				}
				c.BotAuth(func(goCtx context.Context, cx *shared.Ctx) error {
					if allowed[cx.Update.UserID] {
						return nil
					}
					return fmt.Errorf("denied")
				})
			}
			return inst
		},
		HelpText: `client(bot_token, app_token, allowed_users=[]) - Create a Slack bot client`,
	}

	builtins["keyboard"] = shared.KeyboardBuiltin

	// Slack-specific: open a DM channel with a user by their user ID
	builtins["open_dm"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			inst, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewError("open_dm: invalid client")
			}
			w, ok := inst.Fields[nativeClientKey].(*clientWrapper)
			if !ok {
				return errors.NewError("open_dm: invalid client")
			}
			userID, _ := args[1].AsString()
			channelID, err := w.c.openDM(ctx, userID)
			if err != nil {
				return errors.NewError("open_dm: %s", err.Error())
			}
			return &object.String{Value: channelID}
		},
		HelpText: `open_dm(client, user_id) - Open or retrieve a DM channel with a user, returns channel ID`,
	}

	return object.NewLibrary(LibraryName, builtins, map[string]object.Object{
		"SlackClient": slackClientClass,
	}, "Slack Bot API client")
}

// Register registers the slack library with a Scriptling instance.
func Register(registrar interface{ RegisterLibrary(*object.Library) }, log logger.Logger) {
	if log == nil {
		log = logger.NewNullLogger()
	}
	registrar.RegisterLibrary(NewLibrary(log.WithGroup("slack")))
}
