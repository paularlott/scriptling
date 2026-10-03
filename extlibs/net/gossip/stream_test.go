package gossip

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// TestGossipStreams runs two real nodes in one process: A serves streams,
// B joins A and reads replies with open_stream().
func TestGossipStreams(t *testing.T) {
	for _, tc := range []struct{ name, opts string }{
		{"socket", `transport="socket"`},
		{"http", `transport="http"`},
		{"encrypted", `encryption_key="0123456789abcdef0123456789abcdef"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A node bound to port 0 reports ":0" as its address, so reserve
			// a real port for the node others join.
			addr := freeAddr(t)
			server := newScriptling()
			if _, err := server.Eval(`
import scriptling.net.gossip as gossip
a = gossip.create(bind_addr="` + addr + `", ` + tc.opts + `)

def lines(msg, w):
    for i in range(msg["payload"]["n"]):
        w.write("line " + str(i) + "\n")

def big(msg, w):
    chunk = b"x" * 65536
    for i in range(16):
        w.write(chunk)

def fails(msg, w):
    w.write("partial")
    raise ValueError("boom")

saved = []
def keeps(msg, w):
    saved.append(w)
    w.write("ok")

a.handle_stream(200, lines)
a.handle_stream(201, big)
a.handle_stream(202, fails)
a.handle_stream(203, keeps)
a.start()
node = a.local_node()
a_id = node["id"]
a_addr = node["addr"]
`); err != nil {
				t.Fatalf("server: %v", err)
			}
			defer server.Eval("a.stop()")
			aID, _ := server.GetVarAsString("a_id")
			aAddr, _ := server.GetVarAsString("a_addr")

			client := newScriptling()
			result, err := client.Eval(`
import time
import scriptling.net.gossip as gossip
b = gossip.create(bind_addr="127.0.0.1:0", ` + tc.opts + `)
b.start()
b.join(["` + aAddr + `"])
for _ in range(200):
    if b.get_node("` + aID + `") is not None:
        break
    time.sleep(0.02)

out = []

# Line by line, then the end of the reply.
with b.open_stream("` + aID + `", 200, {"n": 3}) as s:
    out.append(s.readline())
    out.append(s.read())
    out.append(s.read())

# A reply far larger than a packet.
s = b.open_stream("` + aID + `", 201, None)
data = s.read()
s.close()
out.append(len(data))

# A handler error reaches the caller.
try:
    b.open_stream("` + aID + `", 202, None).read()
    out.append("no error")
except Exception as e:
    out.append("boom" in str(e))

out.append(b.open_stream("` + aID + `", 203, None).read())
b.stop()
out
`)
			if err != nil {
				t.Fatalf("client: %v", err)
			}
			want := `[b'line 0\n', b'line 1\nline 2\n', b'', 1048576, True, b'ok']`
			if got := result.Inspect(); got != want {
				t.Fatalf("got %s\nwant %s", got, want)
			}

			// The writer kept by the handler is closed once it returned.
			_, err = server.Eval(`saved[0].write("late")`)
			if err == nil || !strings.Contains(err.Error(), "only valid inside the stream handler") {
				t.Fatalf("late write: %v", err)
			}
		})
	}
}

func TestGossipStreamErrors(t *testing.T) {
	p := newScriptling()
	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
errs = []
for f in [lambda: c.handle_stream(5, lambda m, w: None), lambda: c.open_stream("missing", 200, None), lambda: c.open_stream(c.local_node()["id"], 1, None)]:
    try:
        f()
        errs.append("no error")
    except Exception as e:
        errs.append(str(e))
c.stop()
errs
`)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	list := result.(*object.List)
	for i, want := range []string{">= 128", "node not found", ">= 128"} {
		if got := list.Elements[i].Inspect(); !strings.Contains(got, want) {
			t.Errorf("error %d = %s, want it to mention %q", i, got, want)
		}
	}
}

// freeAddr returns a loopback address with a port that was free a moment ago.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// handle_with_reply / send_request between two real nodes.
func TestGossipRequestReplyAcrossNodes(t *testing.T) {
	addr := freeAddr(t)
	server := newScriptling()
	if _, err := server.Eval(`
import scriptling.net.gossip as gossip
a = gossip.create(bind_addr="` + addr + `")
a.handle_with_reply(210, lambda msg: {"echo": msg["payload"], "type": msg["type"]})
a.start()
a_id = a.local_node()["id"]
`); err != nil {
		t.Fatalf("server: %v", err)
	}
	defer server.Eval("a.stop()")
	aID, _ := server.GetVarAsString("a_id")

	client := newScriptling()
	result, err := client.Eval(`
import time
import scriptling.net.gossip as gossip
b = gossip.create(bind_addr="127.0.0.1:0")
b.start()
b.join(["` + addr + `"])
for _ in range(200):
    if b.get_node("` + aID + `") is not None:
        break
    time.sleep(0.02)
r = b.send_request("` + aID + `", 210, {"k": [1, 2]})
b.stop()
r
`)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if got := result.Inspect(); got != "{'echo': {'k': [1, 2]}, 'type': 210}" && got != "{'type': 210, 'echo': {'k': [1, 2]}}" {
		t.Fatalf("reply = %s", got)
	}
}

// startSlowStreamServer starts a node whose type-220 handler sends 256 KiB
// (enough to be flushed to the caller), holds the reply open for a few
// seconds, then sends "late". It returns a client joined to it with the
// stream open as s and its first 4 bytes read into first.
func startSlowStreamServer(t *testing.T) (client *scriptling.Scriptling) {
	t.Helper()
	addr := freeAddr(t)
	server := newScriptling()
	if _, err := server.Eval(`
import time
import scriptling.net.gossip as gossip
a = gossip.create(bind_addr="` + addr + `")
def slow(msg, w):
    w.write(b"x" * 262144)
    time.sleep(3)
    w.write("late")
a.handle_stream(220, slow)
a.start()
a_id = a.local_node()["id"]
`); err != nil {
		t.Fatalf("server: %v", err)
	}
	t.Cleanup(func() { server.Eval("a.stop()") })
	aID, _ := server.GetVarAsString("a_id")

	client = newScriptling()
	if _, err := client.Eval(`
import time
import scriptling.net.gossip as gossip
b = gossip.create(bind_addr="127.0.0.1:0")
b.start()
b.join(["` + addr + `"])
for _ in range(200):
    if b.get_node("` + aID + `") is not None:
        break
    time.sleep(0.02)
s = b.open_stream("` + aID + `", 220, None)
first = s.read(4)
`); err != nil {
		t.Fatalf("client: %v", err)
	}
	return client
}

// close() from another task must end a read blocked waiting for data, rather
// than wait for the read (which used to deadlock).
func TestGossipStreamCloseDuringRead(t *testing.T) {
	client := startSlowStreamServer(t)
	defer client.Eval("b.stop()")

	readErr := make(chan error, 1)
	go func() {
		_, err := client.Eval(`s.read()`)
		readErr <- err
	}()
	time.Sleep(200 * time.Millisecond)

	closed := make(chan error, 1)
	go func() {
		_, err := client.Eval(`s.close()`)
		closed <- err
	}()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close() blocked behind a pending read")
	}
	select {
	case err := <-readErr:
		if err == nil || !strings.Contains(err.Error(), "closed stream") {
			t.Fatalf("read after close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("read did not end when the stream was closed")
	}
}

// Stopping the cluster ends the streams it opened.
func TestGossipStreamEndsWithCluster(t *testing.T) {
	client := startSlowStreamServer(t)

	readErr := make(chan error, 1)
	go func() {
		_, err := client.Eval(`s.read()`)
		readErr <- err
	}()
	time.Sleep(200 * time.Millisecond)

	stopped := make(chan error, 1)
	go func() {
		_, err := client.Eval(`b.stop()`)
		stopped <- err
	}()
	for _, ch := range []chan error{stopped, readErr} {
		select {
		case <-ch:
		case <-time.After(2500 * time.Millisecond):
			t.Fatal("stream outlived its cluster")
		}
	}
}

// read(size) allocates for the data received, not the size asked for.
func TestGossipStreamReadHugeSize(t *testing.T) {
	client := startSlowStreamServer(t)
	defer client.Eval("b.stop()")
	result, err := client.Eval(`first`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != `b'xxxx'` {
		t.Fatalf("read(4) = %s", got)
	}
	result, err = client.Eval(`
data = s.read(1 << 40)
s.close()
data
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(result.(*object.Bytes).BytesValue()); got != 262144-4+4 {
		t.Fatalf("read(huge) returned %d bytes", got)
	}
}

func TestGossipHTTPTransportOptions(t *testing.T) {
	p := newScriptling()
	result, err := p.Eval(`
import scriptling.net.gossip as gossip
out = []
for kw in [
    {"encryption_key": "0123456789abcdef"},
    {"bind_addr": "0.0.0.0:0"},
]:
    try:
        gossip.create(transport="http", **kw)
        out.append("no error")
    except Exception as e:
        out.append(str(e))

# A port-0 bind advertises the port actually chosen; a scheme-less
# advertise_addr becomes a URL.
c = gossip.create(transport="http", bind_addr="127.0.0.1:0")
out.append(c.local_node()["addr"])
c.stop()
c = gossip.create(transport="http", bind_addr="127.0.0.1:0", advertise_addr="node1.example:9000")
out.append(c.local_node()["addr"])
c.stop()
out
`)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	list := result.(*object.List)
	checks := []func(string) bool{
		func(s string) bool { return strings.Contains(s, "not supported with the http transport") },
		func(s string) bool { return strings.Contains(s, "set advertise_addr") },
		func(s string) bool { return strings.HasPrefix(s, "http://127.0.0.1:") && !strings.HasSuffix(s, ":0") },
		func(s string) bool { return s == "http://node1.example:9000" },
	}
	for i, ok := range checks {
		if got := list.Elements[i].Inspect(); !ok(got) {
			t.Errorf("result %d = %s", i, got)
		}
	}
}
