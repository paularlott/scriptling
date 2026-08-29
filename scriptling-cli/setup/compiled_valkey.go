//go:build plugin_valkey

package setup

import (
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	valkeyplugin "github.com/paularlott/scriptling/plugins/valkey"
)

func init() {
	plugin.RegisterCompiledIn("scriptling.valkey", valkeyplugin.Description, func(policy *plugin.Policy) (*object.Library, string) {
		return valkeyplugin.Build(&plugin.StaticPolicy{P: policy}), ""
	})
}
