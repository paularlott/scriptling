package gossip

import (
	"net"
	"strings"
	"testing"

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
