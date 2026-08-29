// Command sql serves the MySQL/MariaDB/PostgreSQL plugin over
// the Scriptling plugin protocol (stdio JSON-RPC).
package main

import (
	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/plugin"
	relational "github.com/paularlott/scriptling/plugins/internal/relational"
	sqlplugin "github.com/paularlott/scriptling/plugins/sql"
)

func main() {
	server := plugin.NewServer("scriptling.sql", build.Version, sqlplugin.Description)
	server.RegisterLibrary(sqlplugin.Build(server))
	// The user-facing surface is script: Connection proxies to the plugin
	// object and hands out the host-side ORM kit, so builder chains cost no
	// round trips.
	server.Wrapper("Connection", relational.ConnectionScriptSourceMultiDriver("scriptling.sql"))
	for _, entry := range relational.ScriptKitEntries() {
		if entry.Class {
			server.RegisterScriptClass(entry.Name, entry.Source)
		} else {
			server.RegisterScriptFunc(entry.Name, entry.Source)
		}
	}
	server.Wrapper("connect", sqlplugin.ConnectSource)
	if err := server.Run(); err != nil {
		panic(err)
	}
}
