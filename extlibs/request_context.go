package extlibs

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// requestContextKey is the context key carrying the Request instance a handler
// is serving. HTTP route handlers receive the request as their argument, but
// protocol handlers (MCP tools, JSON-RPC methods) have no request parameter:
// the server stashes the originating HTTP request on the Go context so they
// can query it with scriptling.mcp.tool.get_request() /
// runtime.jsonrpc.get_request().
type requestContextKey struct{}

// WithRequestContext attaches req to ctx so handler-side accessors can reach
// it. The request's context dict is replaced with a shallow copy first: a
// JSON-RPC batch fans one request out to several concurrent handlers, and the
// copy keeps them from sharing mutable state with each other or with anything
// that still holds the original dict.
func WithRequestContext(ctx context.Context, req *object.Instance) context.Context {
	if req != nil {
		if dict, ok := req.Field("context").(*object.Dict); ok {
			req.SetField("context", copyDict(dict))
		}
	}
	return context.WithValue(ctx, requestContextKey{}, req)
}

// RequestContextFrom returns the request stashed on ctx, or nil when there is
// none (e.g. the stdio transports, which have no HTTP request).
func RequestContextFrom(ctx context.Context) *object.Instance {
	req, _ := ctx.Value(requestContextKey{}).(*object.Instance)
	return req
}

// copyDict returns a shallow copy of dict: a new dict with the same key/value
// pairs. Values themselves are shared.
func copyDict(dict *object.Dict) *object.Dict {
	out := &object.Dict{Pairs: make(map[string]object.DictPair, len(dict.Pairs))}
	for k, pair := range dict.Pairs {
		out.Pairs[k] = pair
	}
	return out
}

// RequestContextBuiltins returns the get_request / request_context pair shared
// by scriptling.mcp.tool and scriptling.runtime.jsonrpc, so tool scripts and
// JSON-RPC handlers — which receive only their parameters — can reach the HTTP
// request they are being served for. Over the stdio transports there is no
// HTTP request: get_request() returns None and request_context() an empty dict.
func RequestContextBuiltins() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"get_request": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 0); err != nil {
					return err
				}
				if req := RequestContextFrom(ctx); req != nil {
					return req
				}
				return &object.Null{}
			},
			HelpText: `get_request() - Get the HTTP request this handler is being served for

Returns the same Request object the middleware saw (method, path, headers,
query, path_params, remote_addr and the context dict the middleware may have
populated), or None when there is no HTTP request — the stdio transports and
anywhere else outside a served request.`,
		},
		"request_context": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 0); err != nil {
					return err
				}
				if req := RequestContextFrom(ctx); req != nil {
					if dict, ok := req.Field("context").(*object.Dict); ok {
						// A per-call copy: a JSON-RPC batch dispatches its
						// elements concurrently, and handler writes must stay
						// local rather than racing through a shared dict.
						return copyDict(dict)
					}
				}
				return &object.Dict{Pairs: make(map[string]object.DictPair)}
			},
			HelpText: `request_context() - Get the context dict set by the middleware

Middleware can write to request.context (e.g. request.context["user"] = name
after authenticating); this returns a copy of that dict. Each call gets its
own copy, so writes from the handler are local and never visible to other
handlers of the same request (e.g. other elements of a JSON-RPC batch). It is
always a dict — empty when no middleware ran or set anything — so
request_context().get("user", "default") is always safe.`,
		},
	}
}
