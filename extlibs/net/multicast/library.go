package multicast

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.net.multicast"
	LibraryDesc = "UDP multicast group messaging"
)

type multicastGroup struct {
	mu        sync.Mutex
	recvMu    sync.Mutex
	conn      *net.UDPConn
	addr      *net.UDPAddr
	iface     *net.Interface
	closed    bool
	groupAddr string
	port      int
	localAddr string
}

var (
	library     *object.Library
	libraryOnce sync.Once
	groups      = struct {
		sync.Mutex
		m map[string]*multicastGroup
	}{m: make(map[string]*multicastGroup)}
)

func (g *multicastGroup) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return
	}
	g.closed = true
	if g.conn != nil {
		g.conn.Close()
	}
}

func (g *multicastGroup) send(data []byte) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return fmt.Errorf("group is closed")
	}
	_, err := g.conn.WriteToUDP(data, g.addr)
	return err
}

func (g *multicastGroup) receive(timeout time.Duration) ([]byte, *net.UDPAddr, error) {
	g.recvMu.Lock()
	defer g.recvMu.Unlock()
	if timeout > 0 {
		if err := g.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return nil, nil, err
		}
	} else {
		if err := g.conn.SetReadDeadline(time.Time{}); err != nil {
			return nil, nil, err
		}
	}
	buf := make([]byte, 65536)
	n, src, err := g.conn.ReadFromUDP(buf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	return buf[:n], src, nil
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

func buildGroupObject(g *multicastGroup) *object.Builtin {
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
					if sendErr := g.send(data); sendErr != nil {
						return errors.NewError("send failed: %s", sendErr.Error())
					}
					return &object.Null{}
				},
				HelpText: `send(message) - Send a message to the multicast group

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

					data, src, err := g.receive(time.Duration(timeout * float64(time.Second)))
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
				HelpText: `receive(timeout=30) - Receive a message from the multicast group

Parameters:
  timeout (number, optional): Timeout in seconds (default: 30)

Returns:
  dict with "data" and "source" keys, or None on timeout`,
			},
			"close": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					g.close()
					return &object.Null{}
				},
				HelpText: `close() - Leave the multicast group and close the connection`,
			},
			"group_addr": &object.String{Value: g.groupAddr},
			"port":       object.NewInteger(int64(g.port)),
			"local_addr": &object.String{Value: g.localAddr},
		},
		HelpText: "Multicast group object",
	}
}

func buildLibrary() *object.Library {
	return object.NewLibrary(LibraryName, map[string]*object.Builtin{
		"join": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 1); err != nil {
					return err
				}

				groupAddr, err := args[0].AsString()
				if err != nil {
					return err
				}

				port := int64(0)
				if len(args) > 1 {
					if p, e := args[1].AsInt(); e == nil {
						port = p
					}
				}
				if p := kwargs.Get("port"); p != nil {
					if pv, e := p.AsInt(); e == nil {
						port = pv
					}
				}

				ifaceName := ""
				if iface := kwargs.Get("interface"); iface != nil {
					if iv, e := iface.AsString(); e == nil {
						ifaceName = iv
					}
				}

				addr, addrErr := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", groupAddr, port))
				if addrErr != nil {
					return errors.NewError("invalid multicast address: %s", addrErr.Error())
				}

				if !addr.IP.IsMulticast() {
					return errors.NewError("address %s is not a multicast address", groupAddr)
				}

				var iface *net.Interface
				if ifaceName != "" {
					var ifaceErr error
					iface, ifaceErr = net.InterfaceByName(ifaceName)
					if ifaceErr != nil {
						return errors.NewError("interface not found: %s", ifaceName)
					}
				}

				conn, listenErr := net.ListenMulticastUDP("udp", iface, addr)
				if listenErr != nil {
					return errors.NewError("failed to join multicast group: %s", listenErr.Error())
				}

				localAddr := ""
				if conn.LocalAddr() != nil {
					localAddr = conn.LocalAddr().String()
				}

				g := &multicastGroup{
					conn:      conn,
					addr:      addr,
					iface:     iface,
					groupAddr: groupAddr,
					port:      int(port),
					localAddr: localAddr,
				}

				groups.Lock()
				if old, exists := groups.m[groupAddr]; exists {
					old.close()
				}
				groups.m[groupAddr] = g
				groups.Unlock()

				return buildGroupObject(g)
			},
			HelpText: `join(group_addr, port, interface="") - Join a multicast group

Parameters:
  group_addr (string): Multicast group address (e.g., "239.1.1.1")
  port (int): Port number for the multicast group
  interface (string, optional): Network interface to bind to

Returns:
  Group object with methods: send(), receive(), close()
  Properties: group_addr, port, local_addr

Example:
  import scriptling.net.multicast as mc
  group = mc.join("239.1.1.1", 9999)
  group.send("Hello group!")
  msg = group.receive(timeout=5)
  group.close()`,
		},
	}, nil, LibraryDesc)
}

func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
		extlibs.RegisterCleanup(func() {
			groups.Lock()
			for _, g := range groups.m {
				g.close()
			}
			groups.m = make(map[string]*multicastGroup)
			groups.Unlock()
		})
	})
	registrar.RegisterLibrary(library)
}
