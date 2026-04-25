package console

import (
	"context"

	scriptconsole "github.com/paularlott/scriptling/extlibs/console"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
	"github.com/paularlott/scriptling/object"
)

var consoleClientClass = &object.Class{
	Name:    "ConsoleClient",
	Methods: map[string]object.Object{},
}

func newClientInstance(c *consoleClient, builtins map[string]*object.Builtin) *object.Instance {
	inst := &object.Instance{
		Class:      consoleClientClass,
		Fields:     map[string]object.Object{},
		NativeData: c,
	}
	shared.BindToInstance(inst, builtins)
	return inst
}

// NewLibrary creates the scriptling.messaging.console library.
func NewLibrary() *object.Library {
	builtins := shared.SharedBuiltins()

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
