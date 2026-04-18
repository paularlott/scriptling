package unicast

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.unicast"
	LibraryDesc = "UDP and TCP point-to-point messaging"
)

var (
	library     *object.Library
	libraryOnce sync.Once

	listeners = struct {
		sync.Mutex
		m   map[uint64]io.Closer
		seq uint64
	}{m: make(map[uint64]io.Closer)}
)

func trackListener(c io.Closer) uint64 {
	listeners.Lock()
	listeners.seq++
	id := listeners.seq
	listeners.m[id] = c
	listeners.Unlock()
	return id
}

func untrackListener(id uint64) {
	listeners.Lock()
	delete(listeners.m, id)
	listeners.Unlock()
}

func msgToBytes(msg object.Object) ([]byte, object.Object) {
	if dict, ok := msg.(*object.Dict); ok {
		jsonData, jsonErr := json.Marshal(conversion.ToGo(dict))
		if jsonErr != nil {
			return nil, errors.NewError("failed to encode JSON: %s", jsonErr.Error())
		}
		return jsonData, nil
	}
	if str, ok := msg.(*object.String); ok {
		return []byte(str.Value), nil
	}
	strVal, coerceErr := msg.CoerceString()
	if coerceErr != nil {
		return nil, errors.NewError("message must be string or dict")
	}
	return []byte(strVal), nil
}

// udpConn wraps a connected UDP socket (from net.Dialer.DialContext).
// Using net.Conn (not *net.UDPConn) avoids the EISCONN error that WriteToUDP
// raises on connected sockets on Linux.
type udpConn struct {
	mu         sync.Mutex
	recvMu     sync.Mutex
	conn       net.Conn
	closed     bool
	localAddr  string
	remoteAddr string
}

func (c *udpConn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	c.conn.Close()
}

func (c *udpConn) send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("connection is closed")
	}
	_, err := c.conn.Write(data)
	return err
}

func (c *udpConn) receive(timeout time.Duration) ([]byte, error) {
	c.recvMu.Lock()
	defer c.recvMu.Unlock()
	if timeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(timeout))
	} else {
		c.conn.SetReadDeadline(time.Time{})
	}
	buf := make([]byte, 65536)
	n, err := c.conn.Read(buf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, nil
		}
		return nil, err
	}
	return buf[:n], nil
}

func buildUDPConnObject(c *udpConn) *object.Builtin {
	return &object.Builtin{
		Attributes: map[string]object.Object{
			"send": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					data, dataErr := msgToBytes(args[0])
					if dataErr != nil {
						return dataErr
					}
					if sendErr := c.send(data); sendErr != nil {
						return errors.NewError("send failed: %s", sendErr.Error())
					}
					return &object.Null{}
				},
				HelpText: `send(message) - Send a message to the remote peer

Parameters:
  message (string or dict): Message to send. Dicts are automatically JSON encoded.`,
			},
			"receive": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					timeout := 30.0
					if t := kwargs.Get("timeout"); t != nil {
						if timeoutFloat, e := t.AsFloat(); e == nil {
							timeout = timeoutFloat
						}
					}
					data, err := c.receive(time.Duration(timeout * float64(time.Second)))
					if err != nil {
						return errors.NewError("receive failed: %s", err.Error())
					}
					if data == nil {
						return &object.Null{}
					}
					return object.NewStringDict(map[string]object.Object{
						"data":   &object.String{Value: string(data)},
						"source": &object.String{Value: c.remoteAddr},
					})
				},
				HelpText: `receive(timeout=30) - Receive a message

Parameters:
  timeout (number, optional): Timeout in seconds (default: 30)

Returns:
  dict with "data" and "source" keys, or None on timeout`,
			},
			"close": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					c.close()
					return &object.Null{}
				},
				HelpText: `close() - Close the connection`,
			},
			"connected": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					c.mu.Lock()
					defer c.mu.Unlock()
					return object.NewBoolean(!c.closed)
				},
				HelpText: `connected() - Check if connection is still open`,
			},
			"local_addr":  &object.String{Value: c.localAddr},
			"remote_addr": &object.String{Value: c.remoteAddr},
		},
		HelpText: "UDP connection object",
	}
}

type tcpConn struct {
	mu         sync.Mutex
	recvMu     sync.Mutex
	conn       net.Conn
	closed     bool
	localAddr  string
	remoteAddr string
}

func (c *tcpConn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	c.conn.Close()
}

func (c *tcpConn) send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("connection is closed")
	}
	_, err := c.conn.Write(data)
	return err
}

func (c *tcpConn) receive(timeout time.Duration) ([]byte, error) {
	c.recvMu.Lock()
	defer c.recvMu.Unlock()
	if timeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(timeout))
	} else {
		c.conn.SetReadDeadline(time.Time{})
	}
	buf := make([]byte, 65536)
	n, err := c.conn.Read(buf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, nil
		}
		return nil, err
	}
	return buf[:n], nil
}

func buildTCPConnObject(c *tcpConn) *object.Builtin {
	return &object.Builtin{
		Attributes: map[string]object.Object{
			"send": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					data, dataErr := msgToBytes(args[0])
					if dataErr != nil {
						return dataErr
					}
					if sendErr := c.send(data); sendErr != nil {
						return errors.NewError("send failed: %s", sendErr.Error())
					}
					return &object.Null{}
				},
				HelpText: `send(message) - Send a message

Parameters:
  message (string or dict): Message to send. Dicts are automatically JSON encoded.`,
			},
			"receive": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					timeout := 30.0
					if t := kwargs.Get("timeout"); t != nil {
						if timeoutFloat, e := t.AsFloat(); e == nil {
							timeout = timeoutFloat
						}
					}
					data, err := c.receive(time.Duration(timeout * float64(time.Second)))
					if err != nil {
						return errors.NewError("receive failed: %s", err.Error())
					}
					if data == nil {
						return &object.Null{}
					}
					return object.NewStringDict(map[string]object.Object{
						"data":   &object.String{Value: string(data)},
						"source": &object.String{Value: c.remoteAddr},
					})
				},
				HelpText: `receive(timeout=30) - Receive a message

Parameters:
  timeout (number, optional): Timeout in seconds (default: 30)

Returns:
  dict with "data" and "source" keys, or None on timeout`,
			},
			"close": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					c.close()
					return &object.Null{}
				},
				HelpText: `close() - Close the connection`,
			},
			"connected": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					c.mu.Lock()
					defer c.mu.Unlock()
					return object.NewBoolean(!c.closed)
				},
				HelpText: `connected() - Check if connection is still open`,
			},
			"local_addr":  &object.String{Value: c.localAddr},
			"remote_addr": &object.String{Value: c.remoteAddr},
		},
		HelpText: "TCP connection object",
	}
}

func buildLibrary() *object.Library {
	return object.NewLibrary(LibraryName, map[string]*object.Builtin{
		"connect": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 2); err != nil {
					return err
				}

				host, err := args[0].AsString()
				if err != nil {
					return err
				}

				port, portErr := args[1].AsInt()
				if portErr != nil {
					return errors.NewError("port must be an integer")
				}

				protocol := "udp"
				if p := kwargs.Get("protocol"); p != nil {
					if pv, e := p.AsString(); e == nil {
						protocol = pv
					}
				}

				timeout := 10.0
				if t := kwargs.Get("timeout"); t != nil {
					if timeoutFloat, e := t.AsFloat(); e == nil {
						timeout = timeoutFloat
					}
				}

				addr := fmt.Sprintf("%s:%d", host, port)

				switch protocol {
				case "udp":
					dialer := &net.Dialer{Timeout: time.Duration(timeout * float64(time.Second))}
					conn, dialErr := dialer.DialContext(ctx, "udp", addr)
					if dialErr != nil {
						return errors.NewError("connect failed: %s", dialErr.Error())
					}

					uc := &udpConn{
						conn:       conn,
						localAddr:  conn.LocalAddr().String(),
						remoteAddr: conn.RemoteAddr().String(),
					}
					return buildUDPConnObject(uc)

				case "tcp":
					dialer := &net.Dialer{Timeout: time.Duration(timeout * float64(time.Second))}
					conn, dialErr := dialer.DialContext(ctx, "tcp", addr)
					if dialErr != nil {
						return errors.NewError("connect failed: %s", dialErr.Error())
					}

					tc := &tcpConn{
						conn:       conn,
						localAddr:  conn.LocalAddr().String(),
						remoteAddr: conn.RemoteAddr().String(),
					}
					return buildTCPConnObject(tc)

				default:
					return errors.NewError("unsupported protocol: %s (use 'udp' or 'tcp')", protocol)
				}
			},
			HelpText: `connect(host, port, protocol="udp", timeout=10) - Connect to a remote host

Parameters:
  host (string): Remote host address
  port (int): Remote port number
  protocol (string): "udp" or "tcp" (default: "udp")
  timeout (number): Connection timeout in seconds (default: 10)

Returns:
  Connection object with methods: send(), receive(), close(), connected()
  Properties: local_addr, remote_addr

Example:
  import scriptling.unicast as uc
  conn = uc.connect("192.168.1.1", 8080, protocol="tcp")
  conn.send("Hello!")
  msg = conn.receive(timeout=5)
  conn.close()`,
		},
		"listen": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 1); err != nil {
					return err
				}

				host, hostErr := args[0].AsString()
				if hostErr != nil {
					return hostErr
				}

				port := int64(0)
				if len(args) > 1 {
					if p, e := args[1].AsInt(); e == nil {
						port = p
					}
				}

				protocol := "tcp"
				if p := kwargs.Get("protocol"); p != nil {
					if pv, e := p.AsString(); e == nil {
						protocol = pv
					}
				}

				switch protocol {
				case "udp":
					addr, addrErr := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
					if addrErr != nil {
						return errors.NewError("invalid address: %s", addrErr.Error())
					}

					conn, listenErr := net.ListenUDP("udp", addr)
					if listenErr != nil {
						return errors.NewError("listen failed: %s", listenErr.Error())
					}

					localAddr := conn.LocalAddr().String()
					listenerID := trackListener(conn)

					return &object.Builtin{
						Attributes: map[string]object.Object{
							"receive": &object.Builtin{
								Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
									timeout := 30.0
									if t := kwargs.Get("timeout"); t != nil {
										if timeoutFloat, e := t.AsFloat(); e == nil {
											timeout = timeoutFloat
										}
									}
									if timeout > 0 {
										conn.SetReadDeadline(time.Now().Add(time.Duration(timeout * float64(time.Second))))
									} else {
										conn.SetReadDeadline(time.Time{})
									}
									buf := make([]byte, 65536)
									n, src, err := conn.ReadFromUDP(buf)
									if err != nil {
										if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
											return &object.Null{}
										}
										return errors.NewError("receive failed: %s", err.Error())
									}

									return object.NewStringDict(map[string]object.Object{
										"data":   &object.String{Value: string(buf[:n])},
										"source": &object.String{Value: src.String()},
									})
								},
								HelpText: `receive(timeout=30) - Receive a message from any sender

Returns:
  dict with "data" and "source" keys, or None on timeout`,
							},
							"send_to": &object.Builtin{
								Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
									if err := errors.MinArgs(args, 2); err != nil {
										return err
									}
									addrStr, addrErr := args[0].AsString()
									if addrErr != nil {
										return addrErr
									}
									data, dataErr := msgToBytes(args[1])
									if dataErr != nil {
										return dataErr
									}

									raddr, resolveErr := net.ResolveUDPAddr("udp", addrStr)
									if resolveErr != nil {
										return errors.NewError("invalid address: %s", resolveErr.Error())
									}

									if _, writeErr := conn.WriteToUDP(data, raddr); writeErr != nil {
										return errors.NewError("send failed: %s", writeErr.Error())
									}
									return &object.Null{}
								},
								HelpText: `send_to(address, message) - Send a message to a specific address

Parameters:
  address (string): Target address (e.g., "192.168.1.1:8080")
  message (string or dict): Message to send`,
							},
							"close": &object.Builtin{
								Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
									untrackListener(listenerID)
									conn.Close()
									return &object.Null{}
								},
								HelpText: `close() - Close the listener`,
							},
							"addr": &object.String{Value: localAddr},
						},
						HelpText: "UDP listener object",
					}

				case "tcp":
					listener, listenErr := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
					if listenErr != nil {
						return errors.NewError("listen failed: %s", listenErr.Error())
					}

					listenerAddr := listener.Addr().String()
					listenerID := trackListener(listener)

					return &object.Builtin{
						Attributes: map[string]object.Object{
							"accept": &object.Builtin{
								Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
									timeout := 30.0
									if t := kwargs.Get("timeout"); t != nil {
										if timeoutFloat, e := t.AsFloat(); e == nil {
											timeout = timeoutFloat
										}
									}
									if timeout > 0 {
										listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Duration(timeout * float64(time.Second))))
									}

									conn, acceptErr := listener.Accept()
									if acceptErr != nil {
										if netErr, ok := acceptErr.(net.Error); ok && netErr.Timeout() {
											return &object.Null{}
										}
										return errors.NewError("accept failed: %s", acceptErr.Error())
									}

									tc := &tcpConn{
										conn:       conn,
										localAddr:  conn.LocalAddr().String(),
										remoteAddr: conn.RemoteAddr().String(),
									}
									return buildTCPConnObject(tc)
								},
								HelpText: `accept(timeout=30) - Accept an incoming TCP connection

Parameters:
  timeout (number, optional): Timeout in seconds (default: 30)

Returns:
  TCP connection object or None on timeout`,
							},
							"close": &object.Builtin{
								Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
									untrackListener(listenerID)
									listener.Close()
									return &object.Null{}
								},
								HelpText: `close() - Close the listener`,
							},
							"addr": &object.String{Value: listenerAddr},
						},
						HelpText: "TCP listener object",
					}

				default:
					return errors.NewError("unsupported protocol: %s (use 'udp' or 'tcp')", protocol)
				}
			},
			HelpText: `listen(host, port, protocol="tcp") - Listen for incoming connections

Parameters:
  host (string): Bind address (use "0.0.0.0" to bind all interfaces)
  port (int): Port number to listen on
  protocol (string): "udp" or "tcp" (default: "tcp")

For TCP: returns a listener with accept(), close(), addr
For UDP: returns a listener with receive(), send_to(), close(), addr

Example:
  import scriptling.unicast as uc
  server = uc.listen("0.0.0.0", 8080)
  conn = server.accept(timeout=60)
  if conn:
      msg = conn.receive()
      conn.send("Echo: " + msg["data"])
      conn.close()
  server.close()`,
		},
	}, nil, LibraryDesc)
}

func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
		extlibs.RegisterCleanup(func() {
			listeners.Lock()
			for _, c := range listeners.m {
				c.Close()
			}
			listeners.m = make(map[uint64]io.Closer)
			listeners.Unlock()
		})
	})
	registrar.RegisterLibrary(library)
}
