package mcp

import (
	"context"
	stderrors "errors"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// progressBuiltin is scriptling.mcp.tool.progress(): it sends
// notifications/progress for the MCP request the calling handler is serving,
// through mcp.ReportProgress. It is a no-op when the client did not ask for
// progress (no progressToken), outside an MCP request, and once the request
// has been answered or cancelled.
func progressBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 3 {
				return errors.NewError("progress() takes 1 to 3 positional arguments (progress, total, message), got %d", len(args))
			}
			get := func(i int, name string) object.Object {
				if i < len(args) {
					return args[i]
				}
				return kwargs.Get(name)
			}

			progress, errObj := progressNumber(args[0], "progress")
			if errObj != nil {
				return errObj
			}
			total := 0.0
			if t := get(1, "total"); t != nil {
				if _, isNull := t.(*object.Null); !isNull {
					if total, errObj = progressNumber(t, "total"); errObj != nil {
						return errObj
					}
					if total <= 0 {
						return errors.NewValueError("progress(): total must be greater than 0, got %v", total)
					}
				}
			}
			message := ""
			if m := get(2, "message"); m != nil {
				switch v := m.(type) {
				case *object.Null:
				case *object.String:
					message = v.StringValue()
				default:
					return errors.NewError("progress(): message must be a string, got %s", m.Type())
				}
			}

			switch err := mcplib.ReportProgress(ctx, progress, total, message); {
			case err == nil, stderrors.Is(err, mcplib.ErrRequestCompleted):
				return &object.Null{}
			case stderrors.Is(err, mcplib.ErrProgressNotIncreasing):
				return errors.NewValueError("progress(): progress must increase with each call (got %v)", progress)
			default:
				return errors.NewError("progress(): %v", err)
			}
		},
		HelpText: `progress(progress, total=None, message=None) - Report progress to the MCP client

Sends a notifications/progress message for the request this tool, prompt or
resource handler is serving, so a client can show how far a long-running call
has got. It does nothing when the client didn't ask for progress (it sent no
progressToken), and nothing after the request has been answered.

Parameters:
  progress (int | float): Work done so far; must increase with every call
  total (int | float, optional): Total amount of work, when known
  message (str, optional): Human-readable status

Calls closer together than the server's rate limit (10ms) are dropped, except
the one that reaches total.

Example:
  import scriptling.mcp.tool as tool

  for i, item in enumerate(items):
      process(item)
      tool.progress(i + 1, total=len(items), message="Processed " + item)`,
	}
}

func progressNumber(v object.Object, name string) (float64, object.Object) {
	switch n := v.(type) {
	case *object.Integer, *object.Float:
		f, _ := n.AsFloat()
		return f, nil
	default:
		return 0, errors.NewError("progress(): %s must be a number, got %s", name, v.Type())
	}
}
