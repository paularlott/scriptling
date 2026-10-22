package slack

import (
	"context"
	"fmt"
	"time"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/object"
)

var slackClientClass = &object.Class{
	Name:    "SlackClient",
	Methods: map[string]object.Object{},
}

func newClientInstance(c *slackClient, builtins map[string]*object.Builtin) *object.Instance {
	inst := object.NewInstanceWithData(slackClientClass, nil, c)
	shared.BindToInstance(inst, builtins)
	return inst
}

// NewLibrary creates the scriptling.messaging.slack library. An optional
// network policy governs URLs scripts pass to download() and answer_callback()
// that are not Slack's own hosts.
func NewLibrary(log logger.Logger, cfg ...*netsecurity.Config) *object.Library {
	return NewLibraryConfigured(log, nil, cfg...)
}

// NewLibraryConfigured is NewLibrary with file uploads confined to
// allowedPaths (nil = unrestricted, empty = deny all), as the os library is.
func NewLibraryConfigured(log logger.Logger, allowedPaths []string, cfg ...*netsecurity.Config) *object.Library {
	builtins := shared.SharedBuiltins()
	external := shared.NewExternalURLs(platformDomains, 30*time.Second, cfg...)
	files := shared.FilePolicy(allowedPaths)

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
			c.external = external
			c.files = files
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
			return object.NewString(channelID)
		},
		HelpText: `open_dm(client, user_id) - Open or retrieve a DM channel with a user, returns channel ID`,
	}

	return object.NewLibrary(LibraryName, builtins, map[string]object.Object{
		"SlackClient": slackClientClass,
	}, "Slack Bot API client")
}

// Register registers the slack library with a Scriptling instance.
func Register(registrar interface{ RegisterLibrary(*object.Library) }, log logger.Logger, cfg ...*netsecurity.Config) {
	RegisterConfigured(registrar, log, nil, cfg...)
}

// RegisterConfigured registers the slack library with file uploads confined
// to allowedPaths (nil = unrestricted, empty = deny all).
func RegisterConfigured(registrar interface{ RegisterLibrary(*object.Library) }, log logger.Logger, allowedPaths []string, cfg ...*netsecurity.Config) {
	if log == nil {
		log = logger.NewNullLogger()
	}
	registrar.RegisterLibrary(NewLibraryConfigured(log.WithGroup("slack"), allowedPaths, cfg...))
}
