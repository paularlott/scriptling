package multicast

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/ipv4"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func newScriptling() *scriptling.Scriptling {
	p := scriptling.New()
	Register(p)
	return p
}

// joinGroup is a test helper that opens a multicast socket directly.
func joinGroup(addr string, port int) (*multicastGroup, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenMulticastUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}
	pc := ipv4.NewPacketConn(conn)
	_ = pc.SetMulticastLoopback(true)
	_ = pc.SetMulticastTTL(1)

	key := fmt.Sprintf("%s:%d:%p", addr, port, conn)
	g := &multicastGroup{
		conn:      conn,
		addr:      udpAddr,
		groupAddr: addr,
		port:      port,
		localAddr: conn.LocalAddr().String(),
		key:       key,
	}
	groups.Lock()
	groups.m[key] = g
	groups.Unlock()
	return g, nil
}

func TestMulticastLibraryRegistered(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.multicast as mc
type(mc) == "DICT"
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if b, _ := result.AsBool(); !b {
		t.Error("expected multicast library to have type DICT")
	}
}

func TestMulticastJoinNonMulticastAddress(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.multicast as mc
mc.join("192.168.1.1", 9999)
`)
	if err == nil {
		t.Error("expected error when joining a non-multicast address")
	}
}

func TestMulticastJoinInvalidAddress(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.multicast as mc
mc.join("not-an-ip", 9999)
`)
	if err == nil {
		t.Error("expected error for invalid address")
	}
}

func TestMulticastJoinMissingArgs(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.multicast as mc
mc.join()
`)
	if err == nil {
		t.Error("expected error when no args provided")
	}
}

func TestMulticastJoinInvalidPort(t *testing.T) {
	p := newScriptling()

	for _, script := range []string{
		`import scriptling.net.multicast as mc
mc.join("239.255.0.1", 0)`,
		`import scriptling.net.multicast as mc
mc.join("239.255.0.1", -1)`,
		`import scriptling.net.multicast as mc
mc.join("239.255.0.1", 65536)`,
	} {
		_, err := p.Eval(script)
		if err == nil {
			t.Errorf("expected error for invalid port in: %s", script)
		}
	}
}

func TestMulticastGroupHasMethods(t *testing.T) {
	if testing.Short() {
		t.Skip("requires OS multicast support")
	}

	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.multicast as mc
g = mc.join("239.255.0.1", 9999)
_ = g.send
_ = g.receive
_ = g.close
_ = g.group_addr
_ = g.port
_ = g.local_addr
g.close()
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
}

func TestMulticastCleanupNoPanic(t *testing.T) {
	groups.Lock()
	for _, g := range groups.m {
		g.closeConn()
	}
	groups.m = make(map[string]*multicastGroup)
	groups.Unlock()
}

func TestMulticastCloseIdempotent(t *testing.T) {
	g := &multicastGroup{closed: true}
	g.close()
}

func TestMulticastSendReceiveString(t *testing.T) {
	if testing.Short() {
		t.Skip("requires OS multicast support")
	}

	listener, err := joinGroup("239.255.1.1", 19998)
	if err != nil {
		t.Skipf("could not join multicast group (no OS support?): %v", err)
	}
	defer listener.close()

	sender, err := joinGroup("239.255.1.1", 19998)
	if err != nil {
		t.Fatalf("sender join failed: %v", err)
	}
	defer sender.close()

	var (
		wg      sync.WaitGroup
		recvErr error
		recvMsg string
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		data, _, e := listener.receive(3 * time.Second)
		if e != nil {
			recvErr = e
			return
		}
		if data != nil {
			recvMsg = string(data)
		}
	}()

	time.Sleep(20 * time.Millisecond)

	if err := sender.send([]byte("hello multicast")); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	wg.Wait()

	if recvErr != nil {
		t.Fatalf("receive error: %v", recvErr)
	}
	if recvMsg != "hello multicast" {
		t.Errorf("expected 'hello multicast', got %q", recvMsg)
	}
}

func TestMulticastSendReceiveJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("requires OS multicast support")
	}

	listener, err := joinGroup("239.255.1.2", 19997)
	if err != nil {
		t.Skipf("could not join multicast group: %v", err)
	}
	defer listener.close()

	sender, err := joinGroup("239.255.1.2", 19997)
	if err != nil {
		t.Fatalf("sender join failed: %v", err)
	}
	defer sender.close()

	payload, _ := json.Marshal(map[string]any{"event": "ping", "n": 42})

	var (
		wg      sync.WaitGroup
		recvMsg string
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		data, _, _ := listener.receive(3 * time.Second)
		if data != nil {
			recvMsg = string(data)
		}
	}()

	time.Sleep(20 * time.Millisecond)
	if err := sender.send(payload); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	wg.Wait()

	if !strings.Contains(recvMsg, "ping") {
		t.Errorf("expected JSON with 'ping', got %q", recvMsg)
	}
}

func TestMulticastReceiveTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("requires OS multicast support")
	}

	listener, err := joinGroup("239.255.1.3", 19996)
	if err != nil {
		t.Skipf("could not join multicast group: %v", err)
	}
	defer listener.close()

	data, src, err := listener.receive(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil || src != nil {
		t.Errorf("expected nil on timeout, got data=%q src=%v", data, src)
	}
}

func TestMulticastScriptSendReceive(t *testing.T) {
	if testing.Short() {
		t.Skip("requires OS multicast support")
	}

	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.multicast as mc

listener = mc.join("239.255.1.4", 19995)
sender = mc.join("239.255.1.4", 19995)

sender.send("script test")
msg = listener.receive(timeout=3)

sender.close()
listener.close()

msg["data"] if msg else ""
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}
	if str.Value != "script test" {
		t.Errorf("expected 'script test', got %q", str.Value)
	}
}
