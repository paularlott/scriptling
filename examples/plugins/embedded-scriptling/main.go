package main

import (
	"context"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
)

func main() {
	env := scriptling.New()
	if err := env.RegisterScriptFunc("decorate", `
def decorate(name):
    return "[" + name + "]"
`); err != nil {
		panic(err)
	}

	server := plugin.NewServer("embedded", "1.0.0", "Embedded Scriptling plugin example")
	server.FunctionBuiltin("decorate", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		name, err := args[0].AsString()
		if err != nil {
			return err
		}
		result, callErr := env.CallFunction("decorate", name)
		if callErr != nil {
			return object.NewString(callErr.Error())
		}
		return result
	})

	if err := server.Run(); err != nil {
		panic(err)
	}
}
