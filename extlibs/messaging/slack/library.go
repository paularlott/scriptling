package slack

import (
	"context"
	"fmt"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
	"github.com/paularlott/scriptling/object"
)

var slackClientClass = &object.Class{
	Name:    "SlackClient",
	Methods: map[string]object.Object{},
}

func newClientInstance(c *slackClient, builtins map[string]*object.Builtin) *object.Instance {
	inst := &object.Instance{
		Class:      slackClientClass,
		Fields:     map[string]object.Object{},
		NativeData: c,
	}
	shared.BindToInstance(inst, builtins)
	return inst
}

// NewLibrary creates the scriptling.messaging.slack library.
func NewLibrary(log logger.Logger) *object.Library {
	builtins := shared.SharedBuiltins()

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
			c, ok := inst.NativeData.(*slackClient)
			if !ok {
				return errors.NewError("open_dm: invalid client")
			}
			userID, _ := args[1].AsString()
			channelID, err := c.openDM(ctx, userID)
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
