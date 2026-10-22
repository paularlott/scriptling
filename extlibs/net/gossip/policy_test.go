package gossip

import (
	"net"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/stdlib"
)

func newPolicyScriptling(cfg *netsecurity.Config) *scriptling.Scriptling {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	Register(p, nil, cfg)
	return p
}

// freeAddr6 reserves a port on the IPv6 loopback, skipping where it is absent.
func freeAddr6(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}

// startNode starts an unrestricted node at addr that records the payloads of
// type-150 messages in got, and returns its interpreter and node ID.
func startNode(t *testing.T, addr, opts string) (*scriptling.Scriptling, string) {
	t.Helper()
	p := newScriptling()
	if _, err := p.Eval(`
import scriptling.net.gossip as gossip
n = gossip.create(bind_addr="` + addr + `"` + opts + `)
got = []
n.handle(150, lambda msg: got.append(msg["payload"]))
n.start()
node_id = n.local_node()["id"]
node_addr = n.local_node()["addr"]
`); err != nil {
		t.Fatalf("start node %s: %v", addr, err)
	}
	t.Cleanup(func() { p.Eval("n.stop()") })
	id, _ := p.GetVarAsString("node_id")
	return p, id
}

func evalInt(t *testing.T, p *scriptling.Scriptling, expr string) int64 {
	t.Helper()
	r, err := p.Eval(expr)
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	v, _ := r.AsInt()
	return v
}

// A policy that denies the peer's address keeps a cluster from joining it,
// on both transports: the denied node never learns of the joiner.
func TestGossipPolicyBlocksDeniedPeer(t *testing.T) {
	for _, tc := range []struct{ name, opts string }{
		{"socket", ""},
		{"http", `, transport="http"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr := freeAddr(t)
			server, _ := startNode(t, addr, tc.opts)
			aAddr, _ := server.GetVarAsString("node_addr")

			client := newPolicyScriptling(&netsecurity.Config{})
			if _, err := client.Eval(`
import time
import scriptling.net.gossip as gossip
b = gossip.create(bind_addr="127.0.0.1:0"` + tc.opts + `)
b.start()
try:
    b.join(["` + aAddr + `"])
except Exception:
    pass
time.sleep(1)
`); err != nil {
				t.Fatalf("client: %v", err)
			}
			defer client.Eval("b.stop()")

			if n := evalInt(t, server, "n.num_nodes()"); n != 1 {
				t.Errorf("POLICY-BYPASS [gossip %s]: denied peer sees %d nodes, want 1", tc.name, n)
			}
			if n := evalInt(t, client, "b.num_nodes()"); n != 1 {
				t.Errorf("joiner sees %d nodes, want 1", n)
			}
		})
	}
}

// allow_cidrs opens a cluster under a policy, and messaging works with all
// traffic carried over TCP.
func TestGossipPolicyAllowCIDRsPermitsCluster(t *testing.T) {
	for _, tc := range []struct{ name, opts string }{
		{"socket", ""},
		{"http", `, transport="http"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr := freeAddr(t)
			server, aID := startNode(t, addr, tc.opts)
			aAddr, _ := server.GetVarAsString("node_addr")

			client := newPolicyScriptling(&netsecurity.Config{AllowedCIDRs: []string{"127.0.0.0/8"}})
			if _, err := client.Eval(`
import time
import scriptling.net.gossip as gossip
b = gossip.create(bind_addr="127.0.0.1:0"` + tc.opts + `)
b.start()
b.join(["` + aAddr + `"])
for _ in range(200):
    if b.get_node("` + aID + `") is not None:
        break
    time.sleep(0.02)
b.send_to("` + aID + `", 150, "hello")
`); err != nil {
				t.Fatalf("client: %v", err)
			}
			defer client.Eval("b.stop()")

			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) && evalInt(t, server, "len(got)") == 0 {
				time.Sleep(20 * time.Millisecond)
			}
			if r, _ := server.Eval("got"); r.Inspect() != "['hello']" {
				t.Errorf("permitted peer got %s, want ['hello']", r.Inspect())
			}
		})
	}
}

// Under a policy the socket transport sends nothing over UDP (datagrams
// would bypass the dialer), so its UDP port is never bound.
func TestGossipPolicyDisablesUDP(t *testing.T) {
	for _, tc := range []struct {
		name      string
		policy    *netsecurity.Config
		udpBusy   bool
		forceUser string
	}{
		{"no policy", nil, true, ""},
		{"policy", &netsecurity.Config{}, false, ""},
		{"policy overrides force_reliable_transport=False", &netsecurity.Config{}, false, ", force_reliable_transport=False"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr := freeAddr(t)
			p := newScriptling()
			if tc.policy != nil {
				p = newPolicyScriptling(tc.policy)
			}
			if _, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="` + addr + `"` + tc.forceUser + `)
c.start()
`); err != nil {
				t.Fatalf("create: %v", err)
			}
			defer p.Eval("c.stop()")

			udpAddr, _ := net.ResolveUDPAddr("udp", addr)
			l, err := net.ListenUDP("udp", udpAddr)
			if err == nil {
				l.Close()
			}
			if busy := err != nil; busy != tc.udpBusy {
				t.Errorf("UDP port bound = %v, want %v", busy, tc.udpBusy)
			}
		})
	}
}

// Peers learned from the cluster are subject to the policy too: B may reach
// A, A tells B about C, and B must never connect to C. The unrestricted run
// proves C would otherwise receive B's message.
func TestGossipPolicyAppliesToLearnedPeers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy *netsecurity.Config
		cGets  string
	}{
		{"unrestricted", nil, "['to-c']"},
		// Only A's address is granted; C on ::1 stays denied as loopback.
		{"policy", &netsecurity.Config{AllowedCIDRs: []string{"127.0.0.1/32"}}, "[]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, aID := startNode(t, freeAddr(t), "")
			aAddr, _ := a.GetVarAsString("node_addr")
			c, cID := startNode(t, freeAddr6(t), "")
			if _, err := c.Eval(`
import time
n.join(["` + aAddr + `"])
for _ in range(200):
    if n.get_node("` + aID + `") is not None:
        break
    time.sleep(0.02)
`); err != nil {
				t.Fatalf("C join A: %v", err)
			}

			b := newScriptling()
			if tc.policy != nil {
				b = newPolicyScriptling(tc.policy)
			}
			if _, err := b.Eval(`
import time
import scriptling.net.gossip as gossip
b = gossip.create(bind_addr="127.0.0.1:0")
b.start()
b.join(["` + aAddr + `"])
b_id = b.local_node()["id"]
learned = False
for _ in range(300):
    if b.get_node("` + cID + `") is not None:
        learned = True
        break
    time.sleep(0.02)
`); err != nil {
				t.Fatalf("B: %v", err)
			}
			defer b.Eval("b.stop()")
			if r, _ := b.Eval("learned"); r.Inspect() != "True" {
				t.Fatal("B never learned of C through A; the test cannot check learned peers")
			}
			// C accepts messages only from nodes it knows: wait until A has
			// told it about B.
			bID, _ := b.GetVarAsString("b_id")
			if _, err := c.Eval(`
for _ in range(300):
    if n.get_node("` + bID + `") is not None:
        break
    time.sleep(0.02)
`); err != nil {
				t.Fatalf("C learn B: %v", err)
			}

			if _, err := b.Eval(`
b.send_to("` + aID + `", 150, "to-a", reliable=True)
try:
    b.send_to("` + cID + `", 150, "to-c", reliable=True)
except Exception:
    pass
`); err != nil {
				t.Fatalf("B send: %v", err)
			}

			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) && (evalInt(t, a, "len(got)") == 0 || (tc.policy == nil && evalInt(t, c, "len(got)") == 0)) {
				time.Sleep(20 * time.Millisecond)
			}
			time.Sleep(300 * time.Millisecond)
			if r, _ := a.Eval("got"); r.Inspect() != "['to-a']" {
				t.Errorf("permitted peer A got %s, want ['to-a']", r.Inspect())
			}
			if r, _ := c.Eval("got"); r.Inspect() != tc.cGets {
				t.Errorf("peer C got %s, want %s", r.Inspect(), tc.cGets)
			}
		})
	}
}
