//go:build plugin_sql

package setup

import (
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	sqlplugin "github.com/paularlott/scriptling/plugins/sql"
)

func init() {
	plugin.RegisterCompiledIn("scriptling.sql", sqlplugin.Description, func(policy *plugin.Policy) (*object.Library, string) {
		lib := sqlplugin.Build(&plugin.StaticPolicy{P: policy})
		return lib, sqlplugin.ScriptModule()
	})
}
