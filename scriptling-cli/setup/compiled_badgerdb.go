//go:build plugin_badgerdb

package setup

import (
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	badgerplugin "github.com/paularlott/scriptling/plugins/badgerdb"
)

func init() {
	plugin.RegisterCompiledIn("scriptling.badgerdb", badgerplugin.Description, func(policy *plugin.Policy) (*object.Library, string) {
		return badgerplugin.Build(&plugin.StaticPolicy{P: policy}), ""
	})
}
