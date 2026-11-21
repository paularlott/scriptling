package object

import (
	"reflect"
	"testing"
)

func kwargsSets() map[string]Kwargs {
	return map[string]Kwargs{
		"zero":    {},
		"one":     NewKwargsFrom([]string{"a"}, []Object{NewInteger(1)}),
		"several": NewKwargsFrom([]string{"a", "b", "c"}, []Object{NewInteger(1), NewString("x"), NewBoolean(true)}),
	}
}

func TestKwargsAccessors(t *testing.T) {
	want := map[string][]string{"zero": {}, "one": {"a"}, "several": {"a", "b", "c"}}
	for name, k := range kwargsSets() {
		names := want[name]
		if k.Len() != len(names) {
			t.Fatalf("%s: Len = %d, want %d", name, k.Len(), len(names))
		}
		if got := k.Keys(); !reflect.DeepEqual(got, names) {
			t.Fatalf("%s: Keys = %v, want %v", name, got, names)
		}
		var iterated []string
		for n, v := range k.All() {
			if v == nil {
				t.Fatalf("%s: nil value for %s", name, n)
			}
			iterated = append(iterated, n)
		}
		if len(iterated) != len(names) || (len(names) > 0 && !reflect.DeepEqual(iterated, names)) {
			t.Fatalf("%s: All = %v, want %v", name, iterated, names)
		}
		m := k.Map()
		if len(m) != len(names) {
			t.Fatalf("%s: Map has %d entries, want %d", name, len(m), len(names))
		}
		for _, n := range names {
			if !k.Has(n) || k.Get(n) == nil {
				t.Fatalf("%s: missing %s", name, n)
			}
			if v, ok := k.Lookup(n); !ok || v != m[n] {
				t.Fatalf("%s: Lookup(%s) disagrees with Map", name, n)
			}
		}
		if k.Has("zz") || k.Get("zz") != nil {
			t.Fatalf("%s: found a keyword never given", name)
		}
		if _, ok := k.Lookup("zz"); ok {
			t.Fatalf("%s: Lookup found a keyword never given", name)
		}
		// Typed getters fall back to the default when absent.
		if k.MustGetInt("zz", 7) != 7 || k.MustGetString("zz", "d") != "d" ||
			k.MustGetFloat("zz", 1.5) != 1.5 || !k.MustGetBool("zz", true) || k.MustGetList("zz", nil) != nil {
			t.Fatalf("%s: default not returned", name)
		}
	}

	k := kwargsSets()["several"]
	if k.MustGetInt("a", 0) != 1 || k.MustGetString("b", "") != "x" || !k.MustGetBool("c", false) {
		t.Fatal("typed getters returned wrong values")
	}
	if _, err := k.GetInt("b", 0); err == nil {
		t.Fatal("GetInt of a string should fail")
	}
	if _, err := k.GetString("a", ""); err == nil {
		t.Fatal("GetString of an int should fail")
	}
}

func TestNewKwargsSortedAndCopied(t *testing.T) {
	m := map[string]Object{"c": NewInteger(3), "a": NewInteger(1), "b": NewInteger(2)}
	k := NewKwargs(m)
	if got := k.Keys(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("Keys = %v, want sorted", got)
	}
	m["a"] = NewInteger(99)
	delete(m, "b")
	if k.MustGetInt("a", 0) != 1 || !k.Has("b") {
		t.Fatal("NewKwargs must not keep the map")
	}
	// Map returns a copy the caller may change.
	out := k.Map()
	out["a"] = NewInteger(5)
	if k.MustGetInt("a", 0) != 1 {
		t.Fatal("Map must return an owned copy")
	}
}

func TestKwargsMergeAndBuilder(t *testing.T) {
	base := NewKwargsFrom([]string{"a", "b"}, []Object{NewInteger(1), NewInteger(2)})
	over := NewKwargsFrom([]string{"b", "c"}, []Object{NewInteger(20), NewInteger(30)})
	merged := base.Merge(over)
	if got := merged.Keys(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("merged keys = %v", got)
	}
	if merged.MustGetInt("b", 0) != 20 || merged.MustGetInt("c", 0) != 30 {
		t.Fatal("merge did not take the second Kwargs' values")
	}
	if base.MustGetInt("b", 0) != 2 || base.Has("c") {
		t.Fatal("merge changed its receiver")
	}
	if base.Merge(Kwargs{}).Len() != 2 || (Kwargs{}).Merge(over).Len() != 2 {
		t.Fatal("merge with an empty Kwargs")
	}

	var b KwargsBuilder
	b.Grow(2)
	b.Set("x", NewInteger(1))
	b.Set("y", NewInteger(2))
	b.Set("x", NewInteger(3))
	k := b.Kwargs()
	if k.Len() != 2 || k.MustGetInt("x", 0) != 3 {
		t.Fatalf("builder result wrong: %v", k.Keys())
	}
	// Reusing the builder must not touch the Kwargs it handed out.
	b.Set("x", NewInteger(100))
	b.Set("z", NewInteger(100))
	if k.MustGetInt("x", 0) != 3 || k.Has("z") {
		t.Fatal("builder reuse changed an earlier result")
	}
}

// TestKwargsOutlivesCallerBuffer keeps a Kwargs built from a pooled argument
// buffer, as a call site does, then lets the buffer be reused by later calls.
// It fails if a Kwargs borrows the caller's storage.
func TestKwargsOutlivesCallerBuffer(t *testing.T) {
	env := NewEnvironment()
	names := []string{"key", "reverse"}
	var kept []Kwargs
	for i := 0; i < 50; i++ {
		vals := AcquireArgs(env, 2)
		vals[0] = NewInteger(int64(i))
		vals[1] = NewInteger(int64(-i))
		kept = append(kept, NewKwargsFrom(names, vals))
		ReleaseArgs(env, vals)
		// A later call reuses the released buffer.
		reuse := AcquireArgs(env, 2)
		if &reuse[0] != &vals[0] {
			t.Fatal("the pool did not hand back the released buffer; the test proves nothing")
		}
		reuse[0] = NewString("clobbered")
		reuse[1] = NewString("clobbered")
		ReleaseArgs(env, reuse)
	}
	for i, k := range kept {
		if k.MustGetInt("key", -1) != int64(i) || k.MustGetInt("reverse", 1) != int64(-i) {
			t.Fatalf("kept Kwargs %d reads %v / %v after buffer reuse", i, k.Get("key").Inspect(), k.Get("reverse").Inspect())
		}
	}
}
