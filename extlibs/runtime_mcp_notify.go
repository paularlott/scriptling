package extlibs

import (
	"context"
	"sync/atomic"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// mcpResourceNotifier is the running MCP server's NotifyResourceUpdated,
// set by the server when it starts serving MCP. Scripts reach it through
// scriptling.runtime.mcp.notify_resource_updated(); it is process-wide
// because the setup script, background tasks and handlers all run in their
// own interpreters.
var mcpResourceNotifier atomic.Pointer[func(uri string)]

// SetMCPResourceNotifier installs fn as the target of
// notify_resource_updated(); nil removes it, making the call a no-op.
func SetMCPResourceNotifier(fn func(uri string)) {
	if fn == nil {
		mcpResourceNotifier.Store(nil)
		return
	}
	mcpResourceNotifier.Store(&fn)
}

func notifyResourceUpdatedBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			uri, err := args[0].AsString()
			if err != nil || uri == "" {
				return errors.NewError("notify_resource_updated: uri must be a non-empty string")
			}
			if fn := mcpResourceNotifier.Load(); fn != nil {
				(*fn)(uri)
				return object.NewBoolean(true)
			}
			return object.NewBoolean(false)
		},
		HelpText: `notify_resource_updated(uri) - Tell subscribed MCP clients a resource changed

Sends notifications/resources/updated for uri to every client that subscribed
to it with resources/subscribe (clients that didn't subscribe get nothing), so
they can read it again. The server can't see a resource's content change by
itself: call this after writing the data a resource serves, from a tool,
an HTTP handler, a background task or the setup script.

Matching is by exact URI. For a resource template, pass the expanded URI the
client subscribed to (e.g. "notes://42"), not the template.

Parameters:
  uri (str): The resource URI that changed

Returns:
  bool: True if an MCP server is running to send it, False otherwise (for
  example when the script isn't being served as an MCP server)

Example:
  import scriptling.runtime.mcp as mcp

  def save_note(id, text):
      store[id] = text
      mcp.notify_resource_updated("notes://" + id)`,
	}
}
