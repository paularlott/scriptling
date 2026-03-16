package ai

import (
	mcpai "github.com/paularlott/mcp/ai"
	"github.com/paularlott/scriptling/object"
)

// AIClientFromObject extracts the underlying ai.Client from a Scriptling
// object.Instance created by ai.Client(...). Returns nil if the object is
// not a valid AI client instance.
func AIClientFromObject(obj object.Object) mcpai.Client {
	instance, ok := obj.(*object.Instance)
	if !ok {
		return nil
	}
	ci, err := getClientInstance(instance)
	if err != nil || ci == nil {
		return nil
	}
	return ci.client
}
