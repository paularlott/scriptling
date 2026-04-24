package extlibs

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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

// WebSocketRouteInfo stores information about a registered WebSocket route
type WebSocketRouteInfo struct {
	Handler string // "library.function" to call for each connection
}

// WebSocketServerConn wraps a server-side WebSocket connection
type WebSocketServerConn struct {
	mu         sync.Mutex
	conn       *websocket.Conn
	id         string
	remoteAddr string
	closed     bool
	closedCh   chan struct{}
}

// NewWebSocketServerConn creates a new server WebSocket connection wrapper
func NewWebSocketServerConn(conn *websocket.Conn, id string) *WebSocketServerConn {
	addr := ""
	if conn.RemoteAddr() != nil {
		addr = conn.RemoteAddr().String()
	}
	return &WebSocketServerConn{
		conn:       conn,
		id:         id,
		remoteAddr: addr,
		closedCh:   make(chan struct{}),
	}
}

// ID returns the connection ID
func (c *WebSocketServerConn) ID() string {
	return c.id
}

// RemoteAddr returns the remote address
func (c *WebSocketServerConn) RemoteAddr() string {
	return c.remoteAddr
}

// ReadWithTimeout reads a message with timeout. Returns messageType, data, error.
// On timeout, returns 0, nil, nil
func (c *WebSocketServerConn) ReadWithTimeout(timeout time.Duration) (int, []byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, nil, net.ErrClosed
	}

	if timeout > 0 {
		deadline := time.Now().Add(timeout)
		if err := c.conn.SetReadDeadline(deadline); err != nil {
			return 0, nil, err
		}
	} else {
		if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
			return 0, nil, err
		}
	}

	msgType, data, err := c.conn.ReadMessage()
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return 0, nil, nil
		}
		c.closed = true
		close(c.closedCh)
		return 0, nil, err
	}
	return msgType, data, nil
}

// WriteMessage sends a message
func (c *WebSocketServerConn) WriteMessage(msgType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return net.ErrClosed
	}
	return c.conn.WriteMessage(msgType, data)
}

// Close closes the connection
func (c *WebSocketServerConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	close(c.closedCh)
	return c.conn.Close()
}

// IsConnected returns whether the connection is still open
func (c *WebSocketServerConn) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed
}

// ClosedChan returns a channel that closes when the connection closes
func (c *WebSocketServerConn) ClosedChan() <-chan struct{} {
	return c.closedCh
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

// WebSocketClientClass is the class for WebSocket client objects passed to handlers
var WebSocketClientClass = &object.Class{
	Name: "WebSocketClient",
	Methods: map[string]object.Object{
		"connected": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				instance, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("connected() called on non-WebSocketClient object")
				}

				conn := getWSConnFromInstance(instance)
				if conn == nil {
					return object.NewBoolean(false)
				}
				return object.NewBoolean(conn.IsConnected())
			},
			HelpText: `connected() - Check if the WebSocket connection is still open

Returns True if connected, False otherwise.`,
		},
		"receive": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 1); err != nil {
					return err
				}
				instance, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("receive() called on non-WebSocketClient object")
				}

				timeout := 30.0
				if t := kwargs.Get("timeout"); t != nil {
					if timeoutFloat, e := t.AsFloat(); e == nil {
						timeout = timeoutFloat
					}
				}

				conn := getWSConnFromInstance(instance)
				if conn == nil {
					return &object.Null{}
				}

				msgType, data, err := conn.ReadWithTimeout(time.Duration(timeout * float64(time.Second)))
				if err != nil || data == nil {
					return &object.Null{}
				}

				if msgType == websocket.TextMessage {
					return &object.String{Value: string(data)}
				}
				// Binary message - return as list of bytes
				elements := make([]object.Object, len(data))
				for i, b := range data {
					elements[i] = &object.Integer{Value: int64(b)}
				}
				return &object.List{Elements: elements}
			},
			HelpText: `receive(timeout=30) - Receive a message from the WebSocket

Parameters:
  timeout (number, optional): Timeout in seconds (default: 30)

Returns:
  string for text messages, list of bytes for binary, or None on timeout/disconnect`,
		},
		"send": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 2); err != nil {
					return err
				}
				instance, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("send() called on non-WebSocketClient object")
				}

				msg := args[1]
				var data []byte

				// Check if it's a dict - convert to JSON
				if dict, ok := msg.(*object.Dict); ok {
					jsonData, jsonErr := json.Marshal(conversion.ToGo(dict))
					if jsonErr != nil {
						return errors.NewError("failed to encode JSON: %s", jsonErr.Error())
					}
					data = jsonData
				} else if str, ok := msg.(*object.String); ok {
					data = []byte(str.Value)
				} else {
					strVal, coerceErr := msg.CoerceString()
					if coerceErr != nil {
						return errors.NewError("message must be string or dict")
					}
					data = []byte(strVal)
				}

				conn := getWSConnFromInstance(instance)
				if conn == nil {
					return errors.NewError("connection closed")
				}

				if writeErr := conn.WriteMessage(websocket.TextMessage, data); writeErr != nil {
					return errors.NewError("send failed: %s", writeErr.Error())
				}
				return &object.Null{}
			},
			HelpText: `send(message) - Send a message to the WebSocket client

Parameters:
  message (string or dict): Message to send. Dicts are automatically JSON encoded.`,
		},
		"send_binary": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 2); err != nil {
					return err
				}
				instance, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("send_binary() called on non-WebSocketClient object")
				}

				list, ok := args[1].(*object.List)
				if !ok {
					return errors.NewError("send_binary requires a list of bytes")
				}

				data := make([]byte, len(list.Elements))
				for i, elem := range list.Elements {
					b, e := elem.AsInt()
					if e != nil || b < 0 || b > 255 {
						return errors.NewError("send_binary requires list of bytes (0-255)")
					}
					data[i] = byte(b)
				}

				conn := getWSConnFromInstance(instance)
				if conn == nil {
					return errors.NewError("connection closed")
				}

				if writeErr := conn.WriteMessage(websocket.BinaryMessage, data); writeErr != nil {
					return errors.NewError("send_binary failed: %s", writeErr.Error())
				}
				return &object.Null{}
			},
			HelpText: `send_binary(data) - Send binary data to the WebSocket client

Parameters:
  data (list): List of byte values (0-255)`,
		},
		"close": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				instance, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("close() called on non-WebSocketClient object")
				}

				conn := getWSConnFromInstance(instance)
				if conn != nil {
					conn.Close()
				}
				return &object.Null{}
			},
			HelpText: `close() - Close the WebSocket connection`,
		},
	},
}

// getWSConnFromInstance extracts the WebSocketServerConn from an instance
func getWSConnFromInstance(instance *object.Instance) *WebSocketServerConn {
	if instance.Fields == nil {
		return nil
	}
	connIDObj, ok := instance.Fields["__conn_id__"]
	if !ok {
		return nil
	}
	connID, ok := connIDObj.(*object.String)
	if !ok {
		return nil
	}

	RuntimeState.RLock()
	conn := RuntimeState.WebSocketConnections[connID.Value]
	RuntimeState.RUnlock()
	return conn
}

// CreateWebSocketClientInstance creates a new WebSocketClient instance
func CreateWebSocketClientInstance(conn *WebSocketServerConn) *object.Instance {
	// Store connection in global map
	RuntimeState.Lock()
	RuntimeState.WebSocketConnections[conn.ID()] = conn
	RuntimeState.Unlock()

	return &object.Instance{
		Class: WebSocketClientClass,
		Fields: map[string]object.Object{
			"remote_addr": &object.String{Value: conn.RemoteAddr()},
			"__conn_id__": &object.String{Value: conn.ID()},
		},
	}
}

var HTTPSubLibrary = object.NewLibrary(RuntimeHTTPLibraryName, map[string]*object.Builtin{
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
			RuntimeState.Routes["GET "+path] = &RouteInfo{
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
			RuntimeState.Routes["POST "+path] = &RouteInfo{
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
			RuntimeState.Routes["PUT "+path] = &RouteInfo{
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
			RuntimeState.Routes["DELETE "+path] = &RouteInfo{
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
			for _, method := range methods {
				RuntimeState.Routes[method+" "+path] = &RouteInfo{
					Methods: []string{method},
					Handler: handler,
				}
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
			RuntimeState.Routes["GET "+path] = &RouteInfo{
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

	"not_found": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			handler, err := args[0].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			RuntimeState.NotFoundHandler = handler
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `not_found(handler) - Register a handler for 404 Not Found responses

Parameters:
  handler (string): Handler function as "library.function" string

The handler receives the request object and should return a response.
It is called when no route matches the request path, or when a static
asset is not found.

Example:
  runtime.http.not_found("handlers.not_found")`,
	},

	"websocket": {
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
			RuntimeState.WebSocketRoutes[path] = &WebSocketRouteInfo{
				Handler: handler,
			}
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `websocket(path, handler) - Register a WebSocket route

Parameters:
  path (string): URL path for the WebSocket endpoint (e.g., "/ws")
  handler (string): Handler function as "library.function" string

The handler receives a WebSocketClient object and runs for the connection lifetime.
The handler should loop while client.connected() and use client.receive()/client.send().

Example:
  runtime.http.websocket("/chat", "handlers.chat_handler")

  # In handlers.py:
  def chat_handler(client):
      client.send("Welcome!")
      while client.connected():
          msg = client.receive(timeout=60)
          if msg:
              client.send(f"Echo: {msg}")`,
	},
}, map[string]object.Object{
	"Request":        RequestClass,
	"WebSocketClient": WebSocketClientClass,
}, "HTTP server route registration and response helpers")
