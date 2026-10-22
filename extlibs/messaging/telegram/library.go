package telegram

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

var telegramClientClass = &object.Class{
	Name:    "TelegramClient",
	Methods: map[string]object.Object{},
}

func newClientInstance(c *telegramClient, builtins map[string]*object.Builtin) *object.Instance {
	inst := object.NewInstanceWithData(telegramClientClass, nil, c)
	shared.BindToInstance(inst, builtins)
	return inst
}

// NewLibrary creates the scriptling.messaging.telegram library. cfg is the
// optional network policy, which every request goes through.
func NewLibrary(log logger.Logger, cfg ...*netsecurity.Config) *object.Library {
	return NewLibraryConfigured(log, nil, cfg...)
}

// NewLibraryConfigured is NewLibrary with the local files scripts may send
// confined to allowedPaths (nil = unrestricted, empty = deny all), as the os
// library is.
func NewLibraryConfigured(log logger.Logger, allowedPaths []string, cfg ...*netsecurity.Config) *object.Library {
	builtins := shared.SharedBuiltins()
	files := shared.FilePolicy(allowedPaths)
	network := shared.NewNetwork(platformDomains, 60*time.Second, cfg...)

	// Telegram-specific: client constructor
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
				return errors.NewError("telegram.client: token must not be empty")
			}
			c := newClient(token, log)
			c.network = network
			c.files = files
			inst := newClientInstance(c, builtins)
			// allowed_users kwarg installs a default auth handler
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
		HelpText: `client(token, allowed_users=[]) - Create a Telegram bot client`,
	}

	builtins["keyboard"] = shared.KeyboardBuiltin

	return object.NewLibrary(LibraryName, builtins, map[string]object.Object{
		"TelegramClient": telegramClientClass,
	}, "Telegram Bot API client")
}

// Register registers the telegram library with a Scriptling instance.
func Register(registrar interface{ RegisterLibrary(*object.Library) }, log logger.Logger, cfg ...*netsecurity.Config) {
	RegisterConfigured(registrar, log, nil, cfg...)
}

// RegisterConfigured registers the telegram library with the local files
// scripts may send confined to allowedPaths (nil = unrestricted, empty = deny
// all).
func RegisterConfigured(registrar interface{ RegisterLibrary(*object.Library) }, log logger.Logger, allowedPaths []string, cfg ...*netsecurity.Config) {
	if log == nil {
		log = logger.NewNullLogger()
	}
	registrar.RegisterLibrary(NewLibraryConfigured(log.WithGroup("telegram"), allowedPaths, cfg...))
}
