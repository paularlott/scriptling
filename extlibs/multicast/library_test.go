package multicast

import (
	"testing"

	"github.com/paularlott/scriptling"
)

func newScriptling() *scriptling.Scriptling {
	p := scriptling.New()
	Register(p)
	return p
}

func TestMulticastLibraryRegistered(t *testing.T) {
	p := newScriptling()

	result, err := p.Eval(`
import scriptling.multicast as mc
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

	// A unicast address must be rejected before any OS call.
	_, err := p.Eval(`
import scriptling.multicast as mc
mc.join("192.168.1.1", 9999)
`)
	if err == nil {
		t.Error("expected error when joining a non-multicast address")
	}
}

func TestMulticastJoinInvalidAddress(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.multicast as mc
mc.join("not-an-ip", 9999)
`)
	if err == nil {
		t.Error("expected error for invalid address")
	}
}

func TestMulticastJoinMissingArgs(t *testing.T) {
	p := newScriptling()

	_, err := p.Eval(`
import scriptling.multicast as mc
mc.join()
`)
	if err == nil {
		t.Error("expected error when no args provided")
	}
}

func TestMulticastGroupHasMethods(t *testing.T) {
	if testing.Short() {
		t.Skip("requires OS multicast support")
	}

	p := newScriptling()

	// Access each method to verify it exists (accessing a non-existent attribute errors).
	_, err := p.Eval(`
import scriptling.multicast as mc
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
	// Cleanup with no open groups must not panic.
	groups.Lock()
	for _, g := range groups.m {
		g.close()
	}
	groups.m = make(map[string]*multicastGroup)
	groups.Unlock()
}

func TestMulticastCloseIdempotent(t *testing.T) {
	// Double-close on the Go struct must not panic.
	g := &multicastGroup{closed: true}
	g.close()
}
