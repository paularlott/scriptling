// Command badgerdb serves the BadgerDB plugin over the
// Scriptling plugin protocol (stdio JSON-RPC).
package main

import (
	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/plugin"
	badgerplugin "github.com/paularlott/scriptling/plugins/badgerdb"
)

func main() {
	server := plugin.NewServer("scriptling.badgerdb", build.Version, badgerplugin.Description)
	server.RegisterLibrary(badgerplugin.Build(server))
	// open() is a script wrapper in plugin mode: a Go function cannot return
	// an instance over the wire, so it constructs Client through the object
	// protocol instead.
	server.Wrapper("open", badgerplugin.OpenSource)
	if err := server.Run(); err != nil {
		panic(err)
	}
}
