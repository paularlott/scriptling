package gossip

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func newScriptling() *scriptling.Scriptling {
	p := scriptling.New()
	Register(p)
	return p
}

func TestGossipLibraryRegistered(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
type(gossip) == "DICT"
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if b, _ := result.AsBool(); !b {
		t.Error("expected gossip library to have type DICT")
	}
}

func TestGossipMSGUSERConstant(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
gossip.MSG_USER
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	n, err2 := result.AsInt()
	if err2 != nil {
		t.Fatalf("expected integer, got %T", result)
	}
	if n != 128 {
		t.Errorf("expected MSG_USER == 128, got %d", n)
	}
}

func TestGossipCreateAndStop(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
id = c.node_id()
c.stop()
len(id) > 0
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if b, _ := result.AsBool(); !b {
		t.Error("expected node_id to be non-empty")
	}
}

func TestGossipClusterHasMethods(t *testing.T) {
	// Accessing each attribute verifies it exists; a missing attribute causes a script error.
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
_ = c.start
_ = c.join
_ = c.leave
_ = c.stop
_ = c.send
_ = c.send_tagged
_ = c.send_to
_ = c.handle
_ = c.on_state_change
_ = c.nodes
_ = c.alive_nodes
_ = c.local_node
_ = c.num_nodes
_ = c.num_alive
_ = c.set_metadata
_ = c.get_metadata
_ = c.all_metadata
_ = c.delete_metadata
_ = c.node_id
c.stop()
`)
	if err != nil {
		t.Fatalf("script error (missing method): %v", err)
	}
}

func TestGossipLocalNode(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
n = c.local_node()
c.stop()
[type(n) == "DICT", "id" in n, "addr" in n, "state" in n, "metadata" in n]
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected list, got %T", result)
	}
	for i, elem := range list.Elements {
		if b, _ := elem.AsBool(); !b {
			t.Errorf("element %d: expected true", i)
		}
	}
}

func TestGossipMetadata(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()

c.set_metadata("role", "worker")
role = c.get_metadata("role")

missing = c.get_metadata("nonexistent")
c.delete_metadata("role")
after_delete = c.get_metadata("role")

c.stop()
[role, type(missing) == "NULL", type(after_delete) == "NULL"]
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok || len(list.Elements) != 3 {
		t.Fatalf("expected list of 3, got %T", result)
	}
	if s, _ := list.Elements[0].AsString(); s != "worker" {
		t.Errorf("expected role=='worker', got '%s'", s)
	}
	for i := 1; i < 3; i++ {
		if b, _ := list.Elements[i].AsBool(); !b {
			t.Errorf("element %d: expected true", i)
		}
	}
}

func TestGossipDecodeJSON(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
d = gossip.decode_json('{"x": 1, "y": "hello"}')
[type(d) == "DICT", d["x"] == 1, d["y"] == "hello"]
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected list, got %T", result)
	}
	for i, elem := range list.Elements {
		if b, _ := elem.AsBool(); !b {
			t.Errorf("element %d: expected true", i)
		}
	}
}

func TestGossipDecodeJSONInvalid(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.gossip as gossip
gossip.decode_json("not json")
`)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGossipSendMessageTypeTooLow(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
c.send(64, "data")
`)
	if err == nil {
		t.Error("expected error for message_type < 128")
	}
}

func TestGossipNodeCount(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
n = c.num_nodes()
a = c.num_alive()
c.stop()
[n >= 1, a >= 1]
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected list, got %T", result)
	}
	for i, elem := range list.Elements {
		if b, _ := elem.AsBool(); !b {
			t.Errorf("element %d: expected true", i)
		}
	}
}

func TestGossipCleanupNoPanic(t *testing.T) {
	clusters.Lock()
	for id, c := range clusters.m {
		c.Stop()
		delete(clusters.m, id)
	}
	clusters.Unlock()
}
