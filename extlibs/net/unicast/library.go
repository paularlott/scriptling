package unicast

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/net/internal"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.net.unicast"
	LibraryDesc = "UDP and TCP point-to-point messaging"
)

var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 65536)
		return &b
	},
}

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

// netConn is shared by both UDP (connected) and TCP connections.
// mu protects closed/conn for send and close; recvMu serializes receives
// without blocking concurrent sends (always acquired before mu if both needed).
// For TCP, messages are length-prefixed (4-byte big-endian) to provide
// message framing over the stream protocol.
type netConn struct {
	mu         sync.Mutex
	recvMu     sync.Mutex // always acquired before mu if both needed
	conn       net.Conn
	closed     bool
	tcp        bool // true = apply length-prefix framing
	localAddr  string
	remoteAddr string
}

func (c *netConn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	c.conn.Close()
}

func (c *netConn) send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("connection is closed")
	}
	if c.tcp {
		var hdr [4]byte
		binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))
		if _, err := c.conn.Write(hdr[:]); err != nil {
			return err
		}
	}
	_, err := c.conn.Write(data)
	return err
}

func (c *netConn) receive(timeout time.Duration) ([]byte, error) {
	c.recvMu.Lock()
	defer c.recvMu.Unlock()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("connection is closed")
	}
	conn := c.conn
	c.mu.Unlock()
	if timeout > 0 {
		if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return nil, err
		}
	} else {
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			return nil, err
		}
	}
	if c.tcp {
		var hdr [4]byte
		if _, err := io.ReadFull(conn, hdr[:]); err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return nil, nil
			}
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil, fmt.Errorf("connection closed by peer")
			}
			return nil, err
		}
		size := binary.BigEndian.Uint32(hdr[:])
		if size > 65536 {
			return nil, fmt.Errorf("message too large: %d bytes", size)
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(conn, body); err != nil {
			return nil, err
		}
		return body, nil
	}
	buf := *bufPool.Get().(*[]byte)
	defer bufPool.Put(&buf)
	n, err := conn.Read(buf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, nil
		}
		return nil, err
	}
	result := make([]byte, n)
	copy(result, buf[:n])
	return result, nil
}

func buildConnObject(c *netConn, helpText string) *object.Builtin {
	return &object.Builtin{
		Attributes: map[string]object.Object{
			"send": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					data, dataErr := internal.MsgToBytes(args[0])
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
  dict with "data" and "source" keys, or None on timeout

Note: TCP messages are limited to 64KB.`,
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
		HelpText: helpText,
	}
}

// udpListener wraps a *net.UDPConn for use as a server-side UDP listener.
// recvMu serializes receive calls; mu guards closed state for idempotent close.
type udpListener struct {
	mu         sync.Mutex
	recvMu     sync.Mutex
	conn       *net.UDPConn
	closed     bool
	listenerID uint64
	localAddr  string
}

func (l *udpListener) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	untrackListener(l.listenerID)
	l.conn.Close()
}

func (l *udpListener) receive(timeout time.Duration) ([]byte, *net.UDPAddr, error) {
	l.recvMu.Lock()
	defer l.recvMu.Unlock()
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, nil, fmt.Errorf("listener is closed")
	}
	conn := l.conn
	l.mu.Unlock()
	if timeout > 0 {
		if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return nil, nil, err
		}
	} else {
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			return nil, nil, err
		}
	}
	buf := *bufPool.Get().(*[]byte)
	defer bufPool.Put(&buf)
	n, src, err := conn.ReadFromUDP(buf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	result := make([]byte, n)
	copy(result, buf[:n])
	return result, src, nil
}

func (l *udpListener) sendTo(addr *net.UDPAddr, data []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return fmt.Errorf("listener is closed")
	}
	_, err := l.conn.WriteToUDP(data, addr)
	return err
}

func buildUDPListenerObject(l *udpListener) *object.Builtin {
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
					data, src, err := l.receive(time.Duration(timeout * float64(time.Second)))
					if err != nil {
						return errors.NewError("receive failed: %s", err.Error())
					}
					if data == nil {
						return &object.Null{}
					}
					return object.NewStringDict(map[string]object.Object{
						"data":   &object.String{Value: string(data)},
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
					data, dataErr := internal.MsgToBytes(args[1])
					if dataErr != nil {
						return dataErr
					}
					raddr, resolveErr := net.ResolveUDPAddr("udp", addrStr)
					if resolveErr != nil {
						return errors.NewError("invalid address: %s", resolveErr.Error())
					}
					if err := l.sendTo(raddr, data); err != nil {
						return errors.NewError("send failed: %s", err.Error())
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
					l.close()
					return &object.Null{}
				},
				HelpText: `close() - Close the listener`,
			},
			"addr": &object.String{Value: l.localAddr},
		},
		HelpText: "UDP listener object",
	}
}

// tcpListener wraps a *net.TCPListener with idempotent close, mirroring udpListener.
type tcpListener struct {
	mu         sync.Mutex
	listener   *net.TCPListener
	closed     bool
	listenerID uint64
	listenerAddr string
}

func (l *tcpListener) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	untrackListener(l.listenerID)
	l.listener.Close()
}

func (l *tcpListener) accept(timeout time.Duration) (net.Conn, error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, fmt.Errorf("listener is closed")
	}
	l.mu.Unlock()
	if timeout > 0 {
		l.listener.SetDeadline(time.Now().Add(timeout))
	}
	conn, err := l.listener.Accept()
	l.listener.SetDeadline(time.Time{})
	return conn, err
}

func buildTCPListenerObject(l *tcpListener) *object.Builtin {
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
					conn, acceptErr := l.accept(time.Duration(timeout * float64(time.Second)))
					if acceptErr != nil {
						if netErr, ok := acceptErr.(net.Error); ok && netErr.Timeout() {
							return &object.Null{}
						}
						return errors.NewError("accept failed: %s", acceptErr.Error())
					}
					c := &netConn{
						conn:       conn,
						tcp:        true,
						localAddr:  conn.LocalAddr().String(),
						remoteAddr: conn.RemoteAddr().String(),
					}
					return buildConnObject(c, "TCP connection object")
				},
				HelpText: `accept(timeout=30) - Accept an incoming TCP connection

Parameters:
  timeout (number, optional): Timeout in seconds (default: 30)

Returns:
  TCP connection object or None on timeout`,
			},
			"close": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					l.close()
					return &object.Null{}
				},
				HelpText: `close() - Close the listener`,
			},
			"addr": &object.String{Value: l.listenerAddr},
		},
		HelpText: "TCP listener object",
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
					c := &netConn{
						conn:       conn,
						localAddr:  conn.LocalAddr().String(),
						remoteAddr: conn.RemoteAddr().String(),
					}
					return buildConnObject(c, "UDP connection object")

				case "tcp":
					dialer := &net.Dialer{Timeout: time.Duration(timeout * float64(time.Second))}
					conn, dialErr := dialer.DialContext(ctx, "tcp", addr)
					if dialErr != nil {
						return errors.NewError("connect failed: %s", dialErr.Error())
					}
					c := &netConn{
						conn:       conn,
						tcp:        true,
						localAddr:  conn.LocalAddr().String(),
						remoteAddr: conn.RemoteAddr().String(),
					}
					return buildConnObject(c, "TCP connection object")

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
  import scriptling.net.unicast as uc
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

					l := &udpListener{
						conn:      conn,
						localAddr: conn.LocalAddr().String(),
					}
					l.listenerID = trackListener(conn)
					return buildUDPListenerObject(l)

				case "tcp":
					listener, listenErr := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
					if listenErr != nil {
						return errors.NewError("listen failed: %s", listenErr.Error())
					}
					l := &tcpListener{
						listener:     listener.(*net.TCPListener),
						listenerAddr: listener.Addr().String(),
					}
					l.listenerID = trackListener(listener)
					return buildTCPListenerObject(l)

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
  import scriptling.net.unicast as uc
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
