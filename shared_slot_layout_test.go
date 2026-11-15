package scriptling_test

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

// The parse cache shares a program, and with it the slot layout of its
// top-level variables, between interpreters running the same source. An
// interpreter that went on to run more code used to grow that shared layout
// in place, so another interpreter starting from the same source found its
// variables at the wrong slots and read the wrong values.
func TestSharedSlotLayoutNotMutatedAcrossInterpreters(t *testing.T) {
	const setup = "slot_layout_small = 1\n"
	p := scriptling.New()
	if _, err := p.Eval(setup); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := p.Eval("for i in range(3):\n    slot_layout_l = slot_layout_small\n"); err != nil {
			t.Fatal(err)
		}
	}
	q := scriptling.New()
	if _, err := q.Eval(setup); err != nil {
		t.Fatal(err)
	}
	result, err := q.Eval("slot_layout_d = {}\nfor i in range(3):\n    slot_layout_d[i] = i\nslot_layout_d")
	if err != nil {
		t.Fatalf("second interpreter: %v", err)
	}
	if got := result.Inspect(); got != "{0: 0, 1: 1, 2: 2}" {
		t.Fatalf("got %s", got)
	}

	// The same cause made a name bound by from-import in one interpreter
	// unreadable after another had run a call that failed.
	r := scriptling.New(scriptling.WithMaxValueBytes(8 << 20))
	stdlib.RegisterAll(r)
	r.Eval("slot_layout_b = 1")
	r.Eval("import collections\nstr(collections.deque([[0]*100000]*50))")
	s := scriptling.New(scriptling.WithMaxValueBytes(8 << 20))
	stdlib.RegisterAll(s)
	s.Eval("slot_layout_b = 1")
	if _, err := s.Eval("from collections import namedtuple\nnamedtuple"); err != nil {
		t.Fatalf("from-import after another interpreter ran: %v", err)
	}
}
