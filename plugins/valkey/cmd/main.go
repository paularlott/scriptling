// Command valkey serves the valkey/redis plugin over the
// Scriptling plugin protocol (stdio JSON-RPC).
package main

import (
	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/plugin"
	valkeyplugin "github.com/paularlott/scriptling/plugins/valkey"
)

func main() {
	server := plugin.NewServer("scriptling.valkey", build.Version, valkeyplugin.Description)
	server.RegisterLibrary(valkeyplugin.Build(server))
	// connect() is a script wrapper in plugin mode: a Go function cannot
	// return an instance over the wire, so it constructs Client through the
	// object protocol instead.
	server.Wrapper("connect", valkeyplugin.ConnectSource)
	if err := server.Run(); err != nil {
		panic(err)
	}
}
