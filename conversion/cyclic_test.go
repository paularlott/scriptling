package conversion

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestToGoCyclicContainers pins the crash guard: a script can build a
// self-referential list or dict, and converting it to Go used to recurse
// until the Go stack overflowed — a fatal, unrecoverable process crash.
// Cycles now convert to a placeholder; shared (non-cyclic) substructures
// still convert fully.
func TestToGoCyclicContainers(t *testing.T) {
	inner := &object.List{Elements: []object.Object{object.NewInteger(1)}}
	cyclicList := &object.List{Elements: []object.Object{object.NewString("self"), inner}}
	cyclicList.Elements[1] = cyclicList // inner slot now points back at the list

	got := ToGo(cyclicList)
	arr, ok := got.([]interface{})
	if !ok {
		t.Fatalf("expected a slice, got %T", got)
	}
	if arr[0] != "self" {
		t.Fatalf("expected the scalar to convert, got %v", arr[0])
	}
	if arr[1] != cyclicRefPlaceholder {
		t.Fatalf("expected the cycle to become a placeholder, got %v", arr[1])
	}

	// A dict referencing itself through a value.
	selfDict := &object.Dict{Pairs: map[string]object.DictPair{}}
	selfDict.Pairs["loop"] = object.DictPair{
		Key: object.NewString("loop"), Value: selfDict,
	}
	got = ToGo(selfDict)
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected a map, got %T", got)
	}
	if m["loop"] != cyclicRefPlaceholder {
		t.Fatalf("expected the dict cycle to become a placeholder, got %v", m["loop"])
	}

	// Sharing is not a cycle: the same list under two parents converts fully
	// both times (the path set only remembers the active branch).
	shared := &object.List{Elements: []object.Object{object.NewInteger(7)}}
	parent := &object.List{Elements: []object.Object{shared, shared}}
	got = ToGo(parent)
	arr, ok = got.([]interface{})
	if !ok || len(arr) != 2 {
		t.Fatalf("expected two entries, got %#v", got)
	}
	for i, e := range arr {
		sub, ok := e.([]interface{})
		if !ok || len(sub) != 1 || sub[0] != int64(7) {
			t.Fatalf("shared entry %d did not convert fully: %#v", i, e)
		}
	}
}
