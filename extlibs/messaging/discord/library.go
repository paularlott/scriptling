package discord

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

var discordClientClass = &object.Class{
	Name:    "DiscordClient",
	Methods: map[string]object.Object{},
}

func newClientInstance(c *discordClient, builtins map[string]*object.Builtin) *object.Instance {
	inst := object.NewInstanceWithData(discordClientClass, nil, c)
	shared.BindToInstance(inst, builtins)
	return inst
}

// NewLibrary creates the scriptling.messaging.discord library. An optional
// network policy governs URLs scripts pass to download() that are not
// Discord's own hosts.
func NewLibrary(log logger.Logger, cfg ...*netsecurity.Config) *object.Library {
	builtins := shared.SharedBuiltins()
	external := shared.NewExternalURLs(platformDomains, 30*time.Second, cfg...)

	// Discord-specific: client constructor
	builtins["client"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}
			token, errObj := args[0].AsString()
			if errObj != nil {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			if token == "" {
				return errors.NewError("discord.client: token must not be empty")
			}
			c := newClient(token, log)
			c.external = external
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
		HelpText: `client(token, allowed_users=[]) - Create a Discord bot client`,
	}

	builtins["keyboard"] = shared.KeyboardBuiltin

	return object.NewLibrary(LibraryName, builtins, map[string]object.Object{
		"DiscordClient": discordClientClass,
	}, "Discord Bot API client")
}

// Register registers the discord library with a Scriptling instance.
func Register(registrar interface{ RegisterLibrary(*object.Library) }, log logger.Logger, cfg ...*netsecurity.Config) {
	if log == nil {
		log = logger.NewNullLogger()
	}
	registrar.RegisterLibrary(NewLibrary(log.WithGroup("discord"), cfg...))
}
