package extlibs

import (
	"context"
	"sync"

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

// requestRegistrationsKey carries the per-request MCP registrations a
// middleware makes (scriptling.runtime.mcp.register_request_tool /
// register_request_resource / register_request_prompt). The holder is a
// pointer, so appends by the middleware's builtins are visible to the server
// after the middleware passes, even though contexts themselves are immutable.
type requestRegistrationsKey struct{}

// RequestRegistrations collects the request-scoped MCP entries registered for
// the request being served. Entries are the raw registration dicts as the
// middleware produced them; the server converts them into per-request MCP
// providers once the middleware has passed. Nil tool/resource/prompt entry
// kinds are simply not exposed for that request.
type RequestRegistrations struct {
	mu        sync.Mutex
	Tools     []*object.Dict
	Resources []*object.Dict
	Prompts   []*object.Dict
}

// AddTool records a register_request_tool registration dict.
func (r *RequestRegistrations) AddTool(entry *object.Dict) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Tools = append(r.Tools, entry)
}

// AddResource records a register_request_resource registration dict.
func (r *RequestRegistrations) AddResource(entry *object.Dict) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Resources = append(r.Resources, entry)
}

// AddPrompt records a register_request_prompt registration dict.
func (r *RequestRegistrations) AddPrompt(entry *object.Dict) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Prompts = append(r.Prompts, entry)
}

// Empty reports whether nothing was registered for this request.
func (r *RequestRegistrations) Empty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Tools) == 0 && len(r.Resources) == 0 && len(r.Prompts) == 0
}

// RegistrationsFrom returns the registrations attached to ctx, or nil when the
// context is not one of the server's stashed request contexts.
func RegistrationsFrom(ctx context.Context) *RequestRegistrations {
	regs, _ := ctx.Value(requestRegistrationsKey{}).(*RequestRegistrations)
	return regs
}

// newTransportBuiltin builds the transport() function shared by the
// scriptling.runtime.mcp and scriptling.runtime.jsonrpc sub-libraries: it
// answers "how am I being served" so one setup script can work in every mode —
// over stdio the middleware never runs, so per-request registrations have to
// be made unconditionally (or statically) there instead.
func newTransportBuiltin(help string) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			// A stashed request means this call is being served over HTTP,
			// whatever the process-wide mode says.
			if RequestContextFrom(ctx) != nil {
				return object.NewString("http")
			}
			RuntimeState.RLock()
			t := RuntimeState.Transport
			RuntimeState.RUnlock()
			if t != "" {
				return object.NewString(t)
			}
			return &object.Null{}
		},
		HelpText: help,
	}
}

// WithRequestContext attaches req to ctx so handler-side accessors can reach
// it. The request's context dict is replaced with a deep copy first: a
// JSON-RPC batch fans one request out to several concurrent handlers, and the
// copy keeps them from sharing mutable state with each other or with anything
// that still holds the original dict.
func WithRequestContext(ctx context.Context, req *object.Instance) context.Context {
	if req != nil {
		if dict, ok := req.Field("context").(*object.Dict); ok {
			req.SetField("context", copyDict(dict))
		}
	}
	ctx = context.WithValue(ctx, requestContextKey{}, req)
	return context.WithValue(ctx, requestRegistrationsKey{}, &RequestRegistrations{})
}

// RequestContextFrom returns the request stashed on ctx, or nil when there is
// none (e.g. the stdio transports, which have no HTTP request).
func RequestContextFrom(ctx context.Context) *object.Instance {
	req, _ := ctx.Value(requestContextKey{}).(*object.Instance)
	return req
}

// copyDict returns a deep copy of dict, so a handler mutating a nested value
// writes to its own copy rather than racing every other handler of the same
// request.
func copyDict(dict *object.Dict) *object.Dict {
	return copyValue(dict).(*object.Dict)
}

// copyValue copies the mutable containers recursively: dicts and lists can be
// written by a handler, so no two handlers may share one. Everything else is
// immutable from scriptling's view (strings, numbers) or deliberately shared
// (instances, which may carry resources), and passes through as-is.
func copyValue(v object.Object) object.Object {
	switch t := v.(type) {
	case *object.Dict:
		out := &object.Dict{Pairs: make(map[string]object.DictPair, len(t.Pairs))}
		for k, pair := range t.Pairs {
			out.Pairs[k] = object.DictPair{Key: pair.Key, Value: copyValue(pair.Value)}
		}
		return out
	case *object.List:
		elements := make([]object.Object, len(t.Elements))
		for i, element := range t.Elements {
			elements[i] = copyValue(element)
		}
		return &object.List{Elements: elements}
	default:
		return v
	}
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
