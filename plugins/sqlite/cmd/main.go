// Command sqlite serves the sqlite database plugin over
// the Scriptling plugin protocol (stdio JSON-RPC).
package main

import (
	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/plugin"
	relational "github.com/paularlott/scriptling/plugins/internal/relational"
	sqliteplugin "github.com/paularlott/scriptling/plugins/sqlite"
)

func main() {
	server := plugin.NewServer("scriptling.sqlite", build.Version, sqliteplugin.Description)
	server.RegisterLibrary(sqliteplugin.Build(server))
	// The user-facing surface is script: Connection proxies to the plugin
	// object and hands out the host-side ORM kit, so builder chains cost no
	// round trips. Transaction wraps the remote transaction object begin()
	// returns for the same reason.
	server.Wrapper("Connection", relational.ConnectionScriptSource("scriptling.sqlite", relational.SQLiteSpec))
	server.Wrapper("Transaction", relational.TransactionScriptSource("scriptling.sqlite", relational.SQLiteSpec))
	for _, entry := range relational.ScriptKitEntries() {
		if entry.Class {
			server.RegisterScriptClass(entry.Name, entry.Source)
		} else {
			server.RegisterScriptFunc(entry.Name, entry.Source)
		}
	}
	server.Wrapper("connect", sqliteplugin.ConnectSource)
	if err := server.Run(); err != nil {
		panic(err)
	}
}
