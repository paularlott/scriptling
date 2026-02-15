package extlibs

import (
	"context"
	"net/url"
	"strings"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// httpResponse creates a standard HTTP response dict with status, headers, and body.
// The headers parameter should be a map of header names to values that will be converted to object.String.
func httpResponse(statusCode int64, headers map[string]string, body object.Object) object.Object {
	headerDict := make(map[string]object.Object, len(headers))
	for k, v := range headers {
		headerDict[k] = &object.String{Value: v}
	}

	return object.NewStringDict(map[string]object.Object{
		"status":  object.NewInteger(statusCode),
		"headers": object.NewStringDict(headerDict),
		"body":    body,
	})
}

// RouteInfo stores information about a registered route
type RouteInfo struct {
	Methods   []string
	Handler   string
	Static    bool
	StaticDir string
}

// RequestClass is the class for Request objects passed to handlers
var RequestClass = &object.Class{
	Name: "Request",
	Methods: map[string]object.Object{
		"json": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				instance, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("json() called on non-Request object")
				}

				body, err := instance.Fields["body"].AsString()
				if err != nil {
					return err
				}

				if body == "" {
					return &object.Null{}
				}

				return conversion.MustParseJSON(body)
			},
			HelpText: `json() - Parse request body as JSON

Returns the parsed JSON as a dict or list, or None if body is empty.`,
		},
	},
}

// CreateRequestInstance creates a new Request instance with the given data
func CreateRequestInstance(method, path, body string, headers map[string]string, query map[string]string) *object.Instance {
	headerDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
	for k, v := range headers {
		lk := strings.ToLower(k)
		headerDict.SetByString(lk, &object.String{Value: v})
	}

	queryDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
	for k, v := range query {
		queryDict.SetByString(k, &object.String{Value: v})
	}

	return &object.Instance{
		Class: RequestClass,
		Fields: map[string]object.Object{
			"method":  &object.String{Value: method},
			"path":    &object.String{Value: path},
			"body":    &object.String{Value: body},
			"headers": headerDict,
			"query":   queryDict,
		},
	}
}

var HTTPSubLibrary = object.NewLibrary("http", map[string]*object.Builtin{
	"get": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			path, err := args[0].AsString()
			if err != nil {
				return err
			}

			handler, err := args[1].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			RuntimeState.Routes[path] = &RouteInfo{
				Methods: []string{"GET"},
				Handler: handler,
			}
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `get(path, handler) - Register a GET route

Parameters:
  path (string): URL path for the route (e.g., "/api/users")
  handler (string): Handler function as "library.function" string

Example:
  runtime.http.get("/health", "handlers.health_check")`,
	},

	"post": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			path, err := args[0].AsString()
			if err != nil {
				return err
			}

			handler, err := args[1].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			RuntimeState.Routes[path] = &RouteInfo{
				Methods: []string{"POST"},
				Handler: handler,
			}
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `post(path, handler) - Register a POST route

Parameters:
  path (string): URL path for the route
  handler (string): Handler function as "library.function" string

Example:
  runtime.http.post("/webhook", "handlers.webhook")`,
	},

	"put": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			path, err := args[0].AsString()
			if err != nil {
				return err
			}

			handler, err := args[1].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			RuntimeState.Routes[path] = &RouteInfo{
				Methods: []string{"PUT"},
				Handler: handler,
			}
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `put(path, handler) - Register a PUT route

Parameters:
  path (string): URL path for the route
  handler (string): Handler function as "library.function" string

Example:
  runtime.http.put("/resource", "handlers.update_resource")`,
	},

	"delete": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			path, err := args[0].AsString()
			if err != nil {
				return err
			}

			handler, err := args[1].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			RuntimeState.Routes[path] = &RouteInfo{
				Methods: []string{"DELETE"},
				Handler: handler,
			}
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `delete(path, handler) - Register a DELETE route

Parameters:
  path (string): URL path for the route
  handler (string): Handler function as "library.function" string

Example:
  runtime.http.delete("/resource", "handlers.delete_resource")`,
	},

	"route": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			path, err := args[0].AsString()
			if err != nil {
				return err
			}

			handler, err := args[1].AsString()
			if err != nil {
				return err
			}

			var methods []string
			if m := kwargs.Get("methods"); m != nil {
				if list, e := m.AsList(); e == nil {
					for _, item := range list {
						if method, e := item.AsString(); e == nil {
							methods = append(methods, strings.ToUpper(method))
						}
					}
				}
			}
			if len(methods) == 0 {
				methods = []string{"GET", "POST", "PUT", "DELETE"}
			}

			RuntimeState.Lock()
			RuntimeState.Routes[path] = &RouteInfo{
				Methods: methods,
				Handler: handler,
			}
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `route(path, handler, methods=["GET", "POST", "PUT", "DELETE"]) - Register a route for multiple methods

Parameters:
  path (string): URL path for the route
  handler (string): Handler function as "library.function" string
  methods (list): List of HTTP methods to accept

Example:
  runtime.http.route("/api", "handlers.api", methods=["GET", "POST"])`,
	},

	"middleware": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			handler, err := args[0].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			RuntimeState.Middleware = handler
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `middleware(handler) - Register middleware for all routes

Parameters:
  handler (string): Middleware function as "library.function" string

The middleware receives the request object and should return:
  - None to continue to the handler
  - A response dict to short-circuit (block the request)

Example:
  runtime.http.middleware("auth.check_request")`,
	},

	"static": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			path, err := args[0].AsString()
			if err != nil {
				return err
			}

			directory, err := args[1].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			RuntimeState.Routes[path] = &RouteInfo{
				Methods:   []string{"GET"},
				Static:    true,
				StaticDir: directory,
			}
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `static(path, directory) - Register a static file serving route

Parameters:
  path (string): URL path prefix for static files (e.g., "/assets")
  directory (string): Local directory to serve files from

Example:
  runtime.http.static("/assets", "./public")`,
	},

	"json": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			statusCode := int64(200)
			var data object.Object = &object.Null{}

			if len(args) >= 2 {
				if code, err := args[0].AsInt(); err == nil {
					statusCode = code
				}
				data = args[1]
			} else {
				data = args[0]
			}

			if c := kwargs.Get("status"); c != nil {
				if code, e := c.AsInt(); e == nil {
					statusCode = code
				}
			}

			return httpResponse(statusCode, map[string]string{
				"Content-Type": "application/json",
			}, data)
		},
		HelpText: `json(status_code, data) - Create a JSON response

Parameters:
  status_code (int): HTTP status code (e.g., 200, 404, 500)
  data: Data to serialize as JSON

Returns:
  dict: Response object for the server

Example:
  return runtime.http.json(200, {"status": "ok"})
  return runtime.http.json(404, {"error": "Not found"})`,
	},

	"redirect": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			location, err := args[0].AsString()
			if err != nil {
				return err
			}

			statusCode := int64(302)
			if len(args) > 1 {
				if code, e := args[1].AsInt(); e == nil {
					statusCode = code
				}
			}
			if c := kwargs.Get("status"); c != nil {
				if code, e := c.AsInt(); e == nil {
					statusCode = code
				}
			}

			return httpResponse(statusCode, map[string]string{
				"Location": location,
			}, &object.String{Value: ""})
		},
		HelpText: `redirect(location, status=302) - Create a redirect response

Parameters:
  location (string): URL to redirect to
  status (int, optional): HTTP status code (default: 302)

Returns:
  dict: Response object for the server

Example:
  return runtime.http.redirect("/new-location")
  return runtime.http.redirect("/permanent", status=301)`,
	},

	"html": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			statusCode := int64(200)
			var htmlContent object.Object = &object.String{Value: ""}

			if len(args) >= 2 {
				if code, err := args[0].AsInt(); err == nil {
					statusCode = code
				}
				htmlContent = args[1]
			} else {
				htmlContent = args[0]
			}

			if c := kwargs.Get("status"); c != nil {
				if code, e := c.AsInt(); e == nil {
					statusCode = code
				}
			}

			return httpResponse(statusCode, map[string]string{
				"Content-Type": "text/html; charset=utf-8",
			}, htmlContent)
		},
		HelpText: `html(status_code, content) - Create an HTML response

Parameters:
  status_code (int): HTTP status code
  content (string): HTML content to return

Returns:
  dict: Response object for the server

Example:
  return runtime.http.html(200, "<h1>Hello World</h1>")`,
	},

	"text": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			statusCode := int64(200)
			var textContent object.Object = &object.String{Value: ""}

			if len(args) >= 2 {
				if code, err := args[0].AsInt(); err == nil {
					statusCode = code
				}
				textContent = args[1]
			} else {
				textContent = args[0]
			}

			if c := kwargs.Get("status"); c != nil {
				if code, e := c.AsInt(); e == nil {
					statusCode = code
				}
			}

			return httpResponse(statusCode, map[string]string{
				"Content-Type": "text/plain; charset=utf-8",
			}, textContent)
		},
		HelpText: `text(status_code, content) - Create a plain text response

Parameters:
  status_code (int): HTTP status code
  content (string): Text content to return

Returns:
  dict: Response object for the server

Example:
  return runtime.http.text(200, "Hello World")`,
	},

	"parse_query": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			queryString, err := args[0].AsString()
			if err != nil {
				return err
			}

			values, parseErr := url.ParseQuery(queryString)
			if parseErr != nil {
				return errors.NewError("failed to parse query string: %s", parseErr.Error())
			}

			pairs := make(map[string]object.DictPair)
			for key, vals := range values {
				keyObj := &object.String{Value: key}
				dk := object.DictKey(keyObj)
				if len(vals) == 1 {
					pairs[dk] = object.DictPair{
						Key:   keyObj,
						Value: &object.String{Value: vals[0]},
					}
				} else {
					elements := make([]object.Object, len(vals))
					for i, v := range vals {
						elements[i] = &object.String{Value: v}
					}
					pairs[dk] = object.DictPair{
						Key:   keyObj,
						Value: &object.List{Elements: elements},
					}
				}
			}

			return &object.Dict{Pairs: pairs}
		},
		HelpText: `parse_query(query_string) - Parse a URL query string

Parameters:
  query_string (string): Query string to parse (with or without leading ?)

Returns:
  dict: Parsed key-value pairs

Example:
  params = runtime.http.parse_query("name=John&age=30")`,
	},
}, map[string]object.Object{
	"Request": RequestClass,
}, "HTTP server route registration and response helpers")
