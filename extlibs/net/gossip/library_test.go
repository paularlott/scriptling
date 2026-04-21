package gossip

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func newScriptling() *scriptling.Scriptling {
	p := scriptling.New()
	Register(p, nil)
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
_ = c.handle_with_reply
_ = c.send_request
_ = c.unhandle
_ = c.on_state_change
_ = c.on_metadata_change
_ = c.on_gossip_interval
_ = c.create_node_group
_ = c.create_leader_election
_ = c.nodes
_ = c.alive_nodes
_ = c.nodes_by_tag
_ = c.get_node
_ = c.local_node
_ = c.num_nodes
_ = c.num_alive
_ = c.num_suspect
_ = c.num_dead
_ = c.is_local
_ = c.candidates
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
[type(n) == "DICT", "id" in n, "addr" in n, "state" in n, "metadata" in n, "tags" in n]
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
s = c.num_suspect()
d = c.num_dead()
c.stop()
[n >= 1, a >= 1, s >= 0, d >= 0]
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

func TestGossipGetNode(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
id = c.node_id()
node = c.get_node(id)
missing = c.get_node("00000000-0000-0000-0000-000000000000")
c.stop()
[type(node) == "DICT", node["id"] == id, type(missing) == "NULL"]
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

func TestGossipIsLocal(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
id = c.node_id()
local = c.is_local(id)
other = c.is_local("00000000-0000-0000-0000-000000000000")
c.stop()
[local == True, other == False]
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

func TestGossipNodesByTag(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0", tags=["web", "api"])
c.start()
web_nodes = c.nodes_by_tag("web")
api_nodes = c.nodes_by_tag("api")
missing_nodes = c.nodes_by_tag("nonexistent")
c.stop()
[len(web_nodes) >= 1, len(api_nodes) >= 1, len(missing_nodes) == 0]
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

func TestGossipCandidates(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
cands = c.candidates()
c.stop()
type(cands) == "LIST"
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if b, _ := result.AsBool(); !b {
		t.Error("expected candidates to return a list")
	}
}

func TestGossipNodeGroup(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
c.set_metadata("role", "worker")
ng = c.create_node_group(criteria={"role": "worker"})
count = ng.count()
local_id = c.node_id()
contains_local = ng.contains(local_id)
nodes = ng.nodes()
ng.close()
c.stop()
[count >= 1, contains_local == True, len(nodes) >= 1]
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

func TestGossipNodeGroupMissingCriteria(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
c.create_node_group()
`)
	if err == nil {
		t.Error("expected error for missing criteria")
	}
}

func TestGossipNodeGroupMethods(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
c.set_metadata("zone", "us-east")
ng = c.create_node_group(criteria={"zone": "us-east"})
_ = ng.nodes
_ = ng.contains
_ = ng.count
_ = ng.send_to_peers
_ = ng.close
ng.close()
c.stop()
`)
	if err != nil {
		t.Fatalf("script error (missing node_group method): %v", err)
	}
}

func TestGossipLeaderElection(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
le = c.create_leader_election(check_interval="500ms", leader_timeout="2s")
le.start()
is_leader = le.is_leader()
has_leader = le.has_leader()
leader_id = le.get_leader_id()
le.stop()
c.stop()
[is_leader == True, has_leader == True, leader_id == c.node_id()]
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

func TestGossipLeaderElectionMethods(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
le = c.create_leader_election()
_ = le.start
_ = le.stop
_ = le.is_leader
_ = le.has_leader
_ = le.get_leader_id
_ = le.send_to_peers
_ = le.on_event
le.stop()
c.stop()
`)
	if err != nil {
		t.Fatalf("script error (missing leader_election method): %v", err)
	}
}

func TestGossipUnhandle(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
c.handle(128, lambda msg: None)
removed = c.unhandle(128)
removed_again = c.unhandle(128)
c.stop()
[removed == True, removed_again == False]
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

func TestGossipHTTPTransport(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0", transport="http")
c.start()
c.stop()
`)
	if err != nil {
		t.Fatalf("script error (http transport): %v", err)
	}
}

func TestGossipInvalidTransport(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(bind_addr="127.0.0.1:0", transport="invalid")
`)
	if err == nil {
		t.Error("expected error for invalid transport")
	}
}

func TestGossipAdvancedConfig(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.net.gossip as gossip
c = gossip.create(
    bind_addr="127.0.0.1:0",
    gossip_interval="3s",
    gossip_max_interval="15s",
    fan_out_multiplier=1.5,
    ttl_multiplier=1.2,
    force_reliable_transport=False,
    prefer_ipv6=False,
    health_check_interval="3s",
    suspect_timeout="2s",
    dead_node_timeout="10s",
    node_retention_time="30m",
    compress_min_size=512,
)
c.start()
c.stop()
`)
	if err != nil {
		t.Fatalf("script error (advanced config): %v", err)
	}
}

func TestGossipNodeGroupWithCallbacks(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.net.gossip as gossip

c = gossip.create(bind_addr="127.0.0.1:0")
c.start()
c.set_metadata("role", "api")

ng = c.create_node_group(
    criteria={"role": "api"},
    on_node_added=lambda node: None,
    on_node_removed=lambda node: None,
)
count = ng.count()
ng.close()
c.stop()
count >= 1
`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if b, _ := result.AsBool(); !b {
		t.Error("expected node group to have at least 1 node")
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
