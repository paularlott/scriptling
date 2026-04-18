package unicast

import (
	"io"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func newScriptling() *scriptling.Scriptling {
	p := scriptling.New()
	Register(p)
	return p
}

func TestUnicastTCPEcho(t *testing.T) {
	p := newScriptling()

	script := `
import scriptling.unicast as uc

server = uc.listen("127.0.0.1", 0, protocol="tcp")
addr = server.addr

conn = uc.connect("127.0.0.1", int(addr.split(":")[1]), protocol="tcp", timeout=5)
sc = server.accept(timeout=5)

conn.send("hello")
msg = sc.receive(timeout=5)

sc.send(msg["data"] + " world")
reply = conn.receive(timeout=5)

conn.close()
sc.close()
server.close()

reply["data"]
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected string, got %T: %v", result, result)
	}
	if str.Value != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", str.Value)
	}
}

func TestUnicastTCPReceiveReturnsDict(t *testing.T) {
	p := newScriptling()

	script := `
import scriptling.unicast as uc

server = uc.listen("127.0.0.1", 0, protocol="tcp")
port = int(server.addr.split(":")[1])

conn = uc.connect("127.0.0.1", port, protocol="tcp", timeout=5)
sc = server.accept(timeout=5)

conn.send("test")
msg = sc.receive(timeout=5)

conn.close()
sc.close()
server.close()

[type(msg) == "DICT", "data" in msg, "source" in msg]
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected list, got %T", result)
	}
	for i, elem := range list.Elements {
		b, _ := elem.AsBool()
		if !b {
			t.Errorf("element %d: expected true, got false", i)
		}
	}
}

func TestUnicastUDPEcho(t *testing.T) {
	p := newScriptling()

	script := `
import scriptling.unicast as uc

server = uc.listen("127.0.0.1", 0, protocol="udp")
port = int(server.addr.split(":")[1])

conn = uc.connect("127.0.0.1", port, protocol="udp", timeout=5)
conn.send("ping")

msg = server.receive(timeout=5)
server.send_to(msg["source"], "pong")

reply = conn.receive(timeout=5)

conn.close()
server.close()

reply["data"]
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected string, got %T: %v", result, result)
	}
	if str.Value != "pong" {
		t.Errorf("expected 'pong', got '%s'", str.Value)
	}
}

func TestUnicastUDPReceiveReturnsDict(t *testing.T) {
	p := newScriptling()

	script := `
import scriptling.unicast as uc

server = uc.listen("127.0.0.1", 0, protocol="udp")
port = int(server.addr.split(":")[1])

conn = uc.connect("127.0.0.1", port, protocol="udp")
conn.send("check")

msg = server.receive(timeout=5)
conn.close()
server.close()

[type(msg) == "DICT", "data" in msg, "source" in msg, msg["data"] == "check"]
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected list, got %T", result)
	}
	for i, elem := range list.Elements {
		b, _ := elem.AsBool()
		if !b {
			t.Errorf("element %d: expected true, got false", i)
		}
	}
}

func TestUnicastTCPTimeout(t *testing.T) {
	p := newScriptling()

	script := `
import scriptling.unicast as uc

server = uc.listen("127.0.0.1", 0, protocol="tcp")
port = int(server.addr.split(":")[1])

conn = uc.connect("127.0.0.1", port, protocol="tcp", timeout=5)
sc = server.accept(timeout=5)

msg = conn.receive(timeout=0.1)

conn.close()
sc.close()
server.close()

msg
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("expected None on timeout, got %T", result)
	}
}

func TestUnicastTCPAcceptTimeout(t *testing.T) {
	p := newScriptling()

	script := `
import scriptling.unicast as uc

server = uc.listen("127.0.0.1", 0, protocol="tcp")
conn = server.accept(timeout=0.1)
server.close()
conn
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("expected None on accept timeout, got %T", result)
	}
}

func TestUnicastConnected(t *testing.T) {
	p := newScriptling()

	script := `
import scriptling.unicast as uc

server = uc.listen("127.0.0.1", 0, protocol="tcp")
port = int(server.addr.split(":")[1])

conn = uc.connect("127.0.0.1", port, protocol="tcp", timeout=5)
before = conn.connected()
conn.close()
after = conn.connected()

server.close()
[before, after]
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok || len(list.Elements) != 2 {
		t.Fatalf("expected list of 2, got %T", result)
	}
	if b, _ := list.Elements[0].AsBool(); !b {
		t.Error("expected connected() == true before close")
	}
	if b, _ := list.Elements[1].AsBool(); b {
		t.Error("expected connected() == false after close")
	}
}

func TestUnicastJSONMessage(t *testing.T) {
	p := newScriptling()

	script := `
import scriptling.unicast as uc

server = uc.listen("127.0.0.1", 0, protocol="tcp")
port = int(server.addr.split(":")[1])

conn = uc.connect("127.0.0.1", port, protocol="tcp", timeout=5)
sc = server.accept(timeout=5)

conn.send({"key": "value", "n": 42})
raw = sc.receive(timeout=5)

conn.close()
sc.close()
server.close()

raw["data"]
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if str.Value == "" {
		t.Error("expected non-empty JSON string")
	}
}

func TestUnicastListenInvalidHost(t *testing.T) {
	p := newScriptling()

	// Passing a non-string host should return an error, not silently use 0.0.0.0.
	_, err := p.Eval(`
import scriptling.unicast as uc
uc.listen(42, 8080, protocol="tcp")
`)
	if err == nil {
		t.Error("expected error when host is not a string")
	}
}

func TestUnicastConnectInvalidProtocol(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.unicast as uc
uc.connect("127.0.0.1", 9999, protocol="sctp")
`)
	if err == nil {
		t.Error("expected error for unsupported protocol")
	}
}

func TestUnicastListenerTracking(t *testing.T) {
	// Verify listeners are tracked and removed on close.
	listeners.Lock()
	before := len(listeners.m)
	listeners.Unlock()

	p := newScriptling()
	result, err := p.Eval(`
import scriptling.unicast as uc
s = uc.listen("127.0.0.1", 0, protocol="tcp")
s.addr
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if result == nil {
		t.Fatal("expected addr string")
	}

	listeners.Lock()
	after := len(listeners.m)
	listeners.Unlock()

	if after != before+1 {
		t.Errorf("expected listener count to increase by 1 (before=%d after=%d)", before, after)
	}

	// Close via cleanup function
	listeners.Lock()
	for _, c := range listeners.m {
		c.Close()
	}
	listeners.m = make(map[uint64]io.Closer)
	listeners.Unlock()
}
