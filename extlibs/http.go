package extlibs

import (
	"context"
	"net/url"
	"strings"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

const HTTPLibraryName = "scriptling.http"

// RouteInfo stores information about a registered route
type RouteInfo struct {
	Methods  []string // HTTP methods (GET, POST, etc.)
	Handler  string   // "library.function" string reference
	Static   bool     // true if this is a static file route
	StaticDir string  // directory for static files (if Static is true)
}

// HTTPRoutes stores all registered routes and middleware
// This is populated during setup script execution and used by the server
var HTTPRoutes = struct {
	Routes     map[string]*RouteInfo // path -> RouteInfo
	Middleware string                // "library.function" string reference
}{
	Routes: make(map[string]*RouteInfo),
}

// ResetHTTPRoutes clears all routes (for testing or re-initialization)
func ResetHTTPRoutes() {
	HTTPRoutes.Routes = make(map[string]*RouteInfo)
	HTTPRoutes.Middleware = ""
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
	// Convert headers to Dict
	headerPairs := make(map[string]object.DictPair)
	for k, v := range headers {
		headerPairs[strings.ToLower(k)] = object.DictPair{
			Key:   &object.String{Value: strings.ToLower(k)},
			Value: &object.String{Value: v},
		}
	}

	// Convert query to Dict
	queryPairs := make(map[string]object.DictPair)
	for k, v := range query {
		queryPairs[k] = object.DictPair{
			Key:   &object.String{Value: k},
			Value: &object.String{Value: v},
		}
	}

	return &object.Instance{
		Class: RequestClass,
		Fields: map[string]object.Object{
			"method":  &object.String{Value: method},
			"path":    &object.String{Value: path},
			"body":    &object.String{Value: body},
			"headers": &object.Dict{Pairs: headerPairs},
			"query":   &object.Dict{Pairs: queryPairs},
		},
	}
}

func RegisterHTTPLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(HTTPLibrary)
}

var HTTPLibrary = object.NewLibrary(HTTPLibraryName, map[string]*object.Builtin{
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

			HTTPRoutes.Routes[path] = &RouteInfo{
				Methods: []string{"GET"},
				Handler: handler,
			}

			return &object.Null{}
		},
		HelpText: `get(path, handler) - Register a GET route

Parameters:
  path (string): URL path for the route (e.g., "/api/users")
  handler (string): Handler function as "library.function" string

Example:
  scriptling.http.get("/health", "handlers.health_check")`,
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

			HTTPRoutes.Routes[path] = &RouteInfo{
				Methods: []string{"POST"},
				Handler: handler,
			}

			return &object.Null{}
		},
		HelpText: `post(path, handler) - Register a POST route

Parameters:
  path (string): URL path for the route
  handler (string): Handler function as "library.function" string

Example:
  scriptling.http.post("/webhook", "handlers.webhook")`,
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

			HTTPRoutes.Routes[path] = &RouteInfo{
				Methods: []string{"PUT"},
				Handler: handler,
			}

			return &object.Null{}
		},
		HelpText: `put(path, handler) - Register a PUT route

Parameters:
  path (string): URL path for the route
  handler (string): Handler function as "library.function" string

Example:
  scriptling.http.put("/resource", "handlers.update_resource")`,
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

			HTTPRoutes.Routes[path] = &RouteInfo{
				Methods: []string{"DELETE"},
				Handler: handler,
			}

			return &object.Null{}
		},
		HelpText: `delete(path, handler) - Register a DELETE route

Parameters:
  path (string): URL path for the route
  handler (string): Handler function as "library.function" string

Example:
  scriptling.http.delete("/resource", "handlers.delete_resource")`,
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

			// Get methods from kwargs
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

			HTTPRoutes.Routes[path] = &RouteInfo{
				Methods: methods,
				Handler: handler,
			}

			return &object.Null{}
		},
		HelpText: `route(path, handler, methods=["GET", "POST", "PUT", "DELETE"]) - Register a route for multiple methods

Parameters:
  path (string): URL path for the route
  handler (string): Handler function as "library.function" string
  methods (list): List of HTTP methods to accept

Example:
  scriptling.http.route("/api", "handlers.api", methods=["GET", "POST"])`,
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

			HTTPRoutes.Middleware = handler

			return &object.Null{}
		},
		HelpText: `middleware(handler) - Register middleware for all routes

Parameters:
  handler (string): Middleware function as "library.function" string

The middleware receives the request object and should return:
  - None to continue to the handler
  - A response dict to short-circuit (block the request)

Example:
  scriptling.http.middleware("auth.check_request")`,
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

			HTTPRoutes.Routes[path] = &RouteInfo{
				Methods:   []string{"GET"},
				Static:    true,
				StaticDir: directory,
			}

			return &object.Null{}
		},
		HelpText: `static(path, directory) - Register a static file serving route

Parameters:
  path (string): URL path prefix for static files (e.g., "/assets")
  directory (string): Local directory to serve files from

Example:
  scriptling.http.static("/assets", "./public")`,
	},

	"json": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			// Get status code
			statusCode := int64(200)
			var data object.Object = &object.Null{}

			if len(args) >= 2 {
				// json(status_code, data)
				if code, err := args[0].AsInt(); err == nil {
					statusCode = code
				}
				data = args[1]
			} else {
				// json(data) - status 200
				data = args[0]
			}

			// Check for kwargs
			if c := kwargs.Get("status"); c != nil {
				if code, e := c.AsInt(); e == nil {
					statusCode = code
				}
			}

			// Return a response dict
			return &object.Dict{Pairs: map[string]object.DictPair{
				"status":  {Key: &object.String{Value: "status"}, Value: object.NewInteger(statusCode)},
				"headers": {Key: &object.String{Value: "headers"}, Value: &object.Dict{Pairs: map[string]object.DictPair{
					"Content-Type": {Key: &object.String{Value: "Content-Type"}, Value: &object.String{Value: "application/json"}},
				}}},
				"body": {Key: &object.String{Value: "body"}, Value: data},
			}}
		},
		HelpText: `json(status_code, data) - Create a JSON response

Parameters:
  status_code (int): HTTP status code (e.g., 200, 404, 500)
  data: Data to serialize as JSON

Returns:
  dict: Response object for the server

Example:
  return scriptling.http.json(200, {"status": "ok"})
  return scriptling.http.json(404, {"error": "Not found"})`,
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

			// Get status code (default 302)
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

			return &object.Dict{Pairs: map[string]object.DictPair{
				"status":  {Key: &object.String{Value: "status"}, Value: object.NewInteger(statusCode)},
				"headers": {Key: &object.String{Value: "headers"}, Value: &object.Dict{Pairs: map[string]object.DictPair{
					"Location": {Key: &object.String{Value: "Location"}, Value: &object.String{Value: location}},
				}}},
				"body": {Key: &object.String{Value: "body"}, Value: &object.String{Value: ""}},
			}}
		},
		HelpText: `redirect(location, status=302) - Create a redirect response

Parameters:
  location (string): URL to redirect to
  status (int, optional): HTTP status code (default: 302)

Returns:
  dict: Response object for the server

Example:
  return scriptling.http.redirect("/new-location")
  return scriptling.http.redirect("/permanent", status=301)`,
	},

	"html": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			// Get status code
			statusCode := int64(200)
			var htmlContent object.Object = &object.String{Value: ""}

			if len(args) >= 2 {
				// html(status_code, content)
				if code, err := args[0].AsInt(); err == nil {
					statusCode = code
				}
				htmlContent = args[1]
			} else {
				// html(content) - status 200
				htmlContent = args[0]
			}

			if c := kwargs.Get("status"); c != nil {
				if code, e := c.AsInt(); e == nil {
					statusCode = code
				}
			}

			return &object.Dict{Pairs: map[string]object.DictPair{
				"status":  {Key: &object.String{Value: "status"}, Value: object.NewInteger(statusCode)},
				"headers": {Key: &object.String{Value: "headers"}, Value: &object.Dict{Pairs: map[string]object.DictPair{
					"Content-Type": {Key: &object.String{Value: "Content-Type"}, Value: &object.String{Value: "text/html; charset=utf-8"}},
				}}},
				"body": {Key: &object.String{Value: "body"}, Value: htmlContent},
			}}
		},
		HelpText: `html(status_code, content) - Create an HTML response

Parameters:
  status_code (int): HTTP status code
  content (string): HTML content to return

Returns:
  dict: Response object for the server

Example:
  return scriptling.http.html(200, "<h1>Hello World</h1>")`,
	},

	"text": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			// Get status code
			statusCode := int64(200)
			var textContent object.Object = &object.String{Value: ""}

			if len(args) >= 2 {
				// text(status_code, content)
				if code, err := args[0].AsInt(); err == nil {
					statusCode = code
				}
				textContent = args[1]
			} else {
				// text(content) - status 200
				textContent = args[0]
			}

			if c := kwargs.Get("status"); c != nil {
				if code, e := c.AsInt(); e == nil {
					statusCode = code
				}
			}

			return &object.Dict{Pairs: map[string]object.DictPair{
				"status":  {Key: &object.String{Value: "status"}, Value: object.NewInteger(statusCode)},
				"headers": {Key: &object.String{Value: "headers"}, Value: &object.Dict{Pairs: map[string]object.DictPair{
					"Content-Type": {Key: &object.String{Value: "Content-Type"}, Value: &object.String{Value: "text/plain; charset=utf-8"}},
				}}},
				"body": {Key: &object.String{Value: "body"}, Value: textContent},
			}}
		},
		HelpText: `text(status_code, content) - Create a plain text response

Parameters:
  status_code (int): HTTP status code
  content (string): Text content to return

Returns:
  dict: Response object for the server

Example:
  return scriptling.http.text(200, "Hello World")`,
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

			// Parse query string
			values, parseErr := url.ParseQuery(queryString)
			if parseErr != nil {
				return errors.NewError("failed to parse query string: %s", parseErr.Error())
			}

			// Convert to dict
			pairs := make(map[string]object.DictPair)
			for key, vals := range values {
				if len(vals) == 1 {
					pairs[key] = object.DictPair{
						Key:   &object.String{Value: key},
						Value: &object.String{Value: vals[0]},
					}
				} else {
					// Multiple values - store as list
					elements := make([]object.Object, len(vals))
					for i, v := range vals {
						elements[i] = &object.String{Value: v}
					}
					pairs[key] = object.DictPair{
						Key:   &object.String{Value: key},
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
  params = scriptling.http.parse_query("name=John&age=30")`,
	},
}, map[string]object.Object{
	"Request": RequestClass,
}, "HTTP server route registration library")
