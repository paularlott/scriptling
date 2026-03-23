package extlibs

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// WebSocketClientConn wraps a websocket.Conn with thread-safe access
type WebSocketClientConn struct {
	mu         sync.Mutex
	conn       *websocket.Conn
	closed     bool
	closedCh   chan struct{}
	remoteAddr string
}

// NewWebSocketClientConn creates a new wrapped WebSocket connection
func NewWebSocketClientConn(conn *websocket.Conn) *WebSocketClientConn {
	addr := ""
	if conn.RemoteAddr() != nil {
		addr = conn.RemoteAddr().String()
	}
	return &WebSocketClientConn{
		conn:       conn,
		closedCh:   make(chan struct{}),
		remoteAddr: addr,
	}
}

// ReadWithTimeout reads a message with a timeout
// Returns messageType, data, error. On timeout, returns 0, nil, nil
func (c *WebSocketClientConn) ReadWithTimeout(timeout time.Duration) (int, []byte, error) {
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
		// Clear deadline
		if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
			return 0, nil, err
		}
	}

	msgType, data, err := c.conn.ReadMessage()
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return 0, nil, nil // Timeout returns nil
		}
		// Connection closed or error
		c.closed = true
		close(c.closedCh)
		return 0, nil, err
	}
	return msgType, data, nil
}

// WriteMessage sends a message
func (c *WebSocketClientConn) WriteMessage(msgType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return net.ErrClosed
	}
	return c.conn.WriteMessage(msgType, data)
}

// Close closes the connection
func (c *WebSocketClientConn) Close() error {
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
func (c *WebSocketClientConn) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed
}

// RemoteAddr returns the remote address
func (c *WebSocketClientConn) RemoteAddr() string {
	return c.remoteAddr
}

// ClosedChan returns a channel that closes when the connection closes
func (c *WebSocketClientConn) ClosedChan() <-chan struct{} {
	return c.closedCh
}

// RegisterWebSocketLibrary registers the WebSocket client library
func RegisterWebSocketLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(WebSocketLibrary)
}

// WebSocketLibrary is the WebSocket client library
var WebSocketLibrary = object.NewLibrary(WebSocketLibraryName, map[string]*object.Builtin{
	"connect": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			url, err := args[0].AsString()
			if err != nil {
				return err
			}

			// Parse timeout from kwargs
			timeout := 10
			if t := kwargs.Get("timeout"); t != nil {
				if timeoutInt, e := t.AsInt(); e == nil {
					timeout = int(timeoutInt)
				}
			}

			// Parse headers from kwargs
			headers := make(map[string]string)
			if h := kwargs.Get("headers"); h != nil {
				if hDict, e := h.AsDict(); e == nil {
					for key, val := range hDict {
						if strVal, e := val.AsString(); e == nil {
							headers[key] = strVal
						}
					}
				}
			}

			// Build header
			header := make(map[string][]string)
			for k, v := range headers {
				header[k] = []string{v}
			}

			// Dial with context and timeout
			dialCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			defer cancel()

			conn, _, dialErr := websocket.DefaultDialer.DialContext(dialCtx, url, header)
			if dialErr != nil {
				return errors.NewError("websocket connect failed: %s", dialErr.Error())
			}

			// Wrap connection
			wsConn := NewWebSocketClientConn(conn)

			// Return connection object with methods
			return &object.Builtin{
				Attributes: map[string]object.Object{
					"send": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							if err := errors.MinArgs(args, 1); err != nil {
								return err
							}

							msg := args[0]
							var data []byte
							var msgType = websocket.TextMessage

							// Check if it's a dict - convert to JSON
							if dict, ok := msg.(*object.Dict); ok {
								jsonData, err := json.Marshal(conversion.ToGo(dict))
								if err != nil {
									return errors.NewError("failed to encode JSON: %s", err.Error())
								}
								data = jsonData
							} else if str, ok := msg.(*object.String); ok {
								data = []byte(str.Value)
							} else {
								// Try to coerce to string
								strVal, err := msg.CoerceString()
								if err != nil {
									return errors.NewError("message must be string or dict")
								}
								data = []byte(strVal)
							}

							if err := wsConn.WriteMessage(msgType, data); err != nil {
								return errors.NewError("send failed: %s", err.Error())
							}
							return &object.Null{}
						},
						HelpText: `send(message) - Send a text message

Parameters:
  message (string or dict): Message to send. Dicts are automatically JSON encoded.`,
					},
					"send_binary": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							if err := errors.MinArgs(args, 1); err != nil {
								return err
							}

							var data []byte

							// Check for bytes (list of ints)
							if list, ok := args[0].(*object.List); ok {
								data = make([]byte, len(list.Elements))
								for i, elem := range list.Elements {
									if b, e := elem.AsInt(); e == nil && b >= 0 && b <= 255 {
										data[i] = byte(b)
									} else {
										return errors.NewError("send_binary requires list of bytes (0-255)")
									}
								}
							} else {
								return errors.NewError("send_binary requires a list of bytes")
							}

							if err := wsConn.WriteMessage(websocket.BinaryMessage, data); err != nil {
								return errors.NewError("send_binary failed: %s", err.Error())
							}
							return &object.Null{}
						},
						HelpText: `send_binary(data) - Send binary data

Parameters:
  data (list): List of byte values (0-255)`,
					},
					"receive": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							timeout := 30
							if t := kwargs.Get("timeout"); t != nil {
								if timeoutInt, e := t.AsInt(); e == nil {
									timeout = int(timeoutInt)
								}
							}

							msgType, data, err := wsConn.ReadWithTimeout(time.Duration(timeout) * time.Second)
							if err != nil {
								// Connection error - return None
								return &object.Null{}
							}
							if data == nil {
								// Timeout - return None
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
						HelpText: `receive(timeout=30) - Receive a message

Parameters:
  timeout (int, optional): Timeout in seconds (default: 30)

Returns:
  string for text messages, list of bytes for binary, or None on timeout/disconnect`,
					},
					"connected": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							return &object.Boolean{Value: wsConn.IsConnected()}
						},
						HelpText: `connected() - Check if connection is still open

Returns:
  True if connected, False otherwise`,
					},
					"close": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							if err := wsConn.Close(); err != nil {
								return errors.NewError("close failed: %s", err.Error())
							}
							return &object.Null{}
						},
						HelpText: `close() - Close the WebSocket connection`,
					},
					"remote_addr": &object.String{Value: wsConn.RemoteAddr()},
				},
				HelpText: "WebSocket connection object",
			}
		},
		HelpText: `connect(url, timeout=10, headers={}) - Connect to a WebSocket server

Parameters:
  url (string): WebSocket URL (ws:// or wss://)
  timeout (int, optional): Connection timeout in seconds (default: 10)
  headers (dict, optional): HTTP headers to send with the handshake

Returns:
  Connection object with methods: send(), send_binary(), receive(), connected(), close()

Example:
  import scriptling.websocket as ws
  conn = ws.connect("wss://echo.websocket.org")
  conn.send("Hello")
  msg = conn.receive(timeout=5)
  conn.close()`,
	},
	"send": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return errors.NewError("send() requires a connection object - use ws.connect() first, then conn.send()")
		},
		HelpText: `send(conn, message) - Send a message on a connection

Note: This is a module-level function. Prefer using conn.send(message) instead.`,
	},
	"send_binary": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return errors.NewError("send_binary() requires a connection object - use ws.connect() first, then conn.send_binary()")
		},
		HelpText: `send_binary(conn, data) - Send binary data on a connection

Note: This is a module-level function. Prefer using conn.send_binary(data) instead.`,
	},
	"receive": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return errors.NewError("receive() requires a connection object - use ws.connect() first, then conn.receive()")
		},
		HelpText: `receive(conn, timeout=30) - Receive a message from a connection

Note: This is a module-level function. Prefer using conn.receive(timeout) instead.`,
	},
	"connected": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return errors.NewError("connected() requires a connection object - use ws.connect() first, then conn.connected()")
		},
		HelpText: `connected(conn) - Check if a connection is still open

Note: This is a module-level function. Prefer using conn.connected() instead.`,
	},
	"close": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return errors.NewError("close() requires a connection object - use ws.connect() first, then conn.close()")
		},
		HelpText: `close(conn) - Close a connection

Note: This is a module-level function. Prefer using conn.close() instead.`,
	},
	"is_text": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}
			_, ok := args[0].(*object.String)
			return &object.Boolean{Value: ok}
		},
		HelpText: `is_text(message) - Check if a received message is a text message

Parameters:
  message: A message returned from receive()

Returns:
  True if the message is a text message (string), False otherwise

Example:
  msg = conn.receive()
  if ws.is_text(msg):
      print(f"Text: {msg}")`,
	},
	"is_binary": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}
			_, ok := args[0].(*object.List)
			return &object.Boolean{Value: ok}
		},
		HelpText: `is_binary(message) - Check if a received message is a binary message

Parameters:
  message: A message returned from receive()

Returns:
  True if the message is binary (list of bytes), False otherwise

Example:
  msg = conn.receive()
  if ws.is_binary(msg):
      print(f"Binary: {len(msg)} bytes")`,
	},
}, nil, "WebSocket client library")
