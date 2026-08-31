//go:build plugin_sqlite

package setup

import (
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	sqliteplugin "github.com/paularlott/scriptling/plugins/sqlite"
)

func init() {
	plugin.RegisterCompiledIn("scriptling.sqlite", sqliteplugin.Description, func(policy *plugin.Policy) (*object.Library, string) {
		lib := sqliteplugin.Build(&plugin.StaticPolicy{P: policy})
		return lib, sqliteplugin.ScriptModule()
	})
}
