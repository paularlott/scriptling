package object

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func filledDict() *Dict {
	d := NewDict()
	d.Store(DictKey(NewString("zebra")), NewString("zebra"), NewInteger(1))
	d.Store(DictKey(NewString("apple")), NewString("apple"), NewInteger(2))
	d.Store(DictKey(NewString("mango")), NewString("mango"), NewInteger(3))
	return d
}

func keyNames(d *Dict) []string {
	var out []string
	for _, pair := range d.OrderedPairs() {
		out = append(out, pair.Key.Inspect())
	}
	return out
}

func keysEqual(t *testing.T, d *Dict, want []string) {
	t.Helper()
	got := keyNames(d)
	if len(got) != len(want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keys = %v, want %v", got, want)
		}
	}
}

func TestDictInsertionOrder(t *testing.T) {
	d := filledDict()
	keysEqual(t, d, []string{"zebra", "apple", "mango"})

	// Re-storing an existing key keeps its position and updates the value.
	d.Store(DictKey(NewString("apple")), NewString("apple"), NewInteger(99))
	keysEqual(t, d, []string{"zebra", "apple", "mango"})
	if v := d.Pairs[DictKey(NewString("apple"))].Value; v.Inspect() != "99" {
		t.Fatalf("apple value = %s, want 99", v.Inspect())
	}

	// Deletion leaves a tombstone; remaining keys keep their order and new
	// keys append at the end.
	d.Delete(DictKey(NewString("apple")))
	keysEqual(t, d, []string{"zebra", "mango"})
	d.Store(DictKey(NewString("kiwi")), NewString("kiwi"), NewInteger(4))
	keysEqual(t, d, []string{"zebra", "mango", "kiwi"})
}

func TestDictLastInserted(t *testing.T) {
	d := filledDict()
	canonical, pair, ok := d.LastInserted()
	if !ok || pair.Key.Inspect() != "mango" {
		t.Fatalf("LastInserted = %s, want mango", pair.Key.Inspect())
	}
	d.Delete(canonical)
	_, pair, ok = d.LastInserted()
	if !ok || pair.Key.Inspect() != "apple" {
		t.Fatalf("LastInserted after delete = %s, want apple", pair.Key.Inspect())
	}

	empty := NewDict()
	if _, _, ok := empty.LastInserted(); ok {
		t.Fatal("LastInserted on empty dict should fail")
	}

	// A dict assembled directly from a Go map has unknown order but still
	// yields a pair.
	loose := &Dict{Pairs: map[string]DictPair{"k": {Key: NewString("k"), Value: NewInteger(1)}}}
	if _, _, ok := loose.LastInserted(); !ok {
		t.Fatal("LastInserted on unknown-order dict should find a pair")
	}
}

func TestDictOrderUnderChurn(t *testing.T) {
	d := NewDict()
	for i := 0; i < 100; i++ {
		k := "k" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		d.Store(DictKey(NewString(k)), NewString(k), NewInteger(int64(i)))
	}
	// Delete most of them; remaining keys must keep relative order, and
	// re-storing a deleted key must not duplicate it.
	for i := 0; i < 90; i++ {
		k := "k" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		d.Delete(DictKey(NewString(k)))
	}
	keys := keyNames(d)
	if len(keys) != 10 {
		t.Fatalf("live keys = %d, want 10", len(keys))
	}
	for i, k := range keys {
		j := 90 + i
		want := "k" + string(rune('a'+j%26)) + string(rune('a'+j/26))
		if k != want {
			t.Fatalf("key[%d] = %s, want %s", i, k, want)
		}
	}

	// Resurrect a deleted key: it goes to the end, exactly once.
	again := "k" + string(rune('a'+0%26)) + string(rune('a'+0/26))
	d.Store(DictKey(NewString(again)), NewString(again), NewInteger(-1))
	count := 0
	for _, k := range keyNames(d) {
		if k == again {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("resurrected key appears %d times, want 1", count)
	}
	if got := keyNames(d)[len(keys)]; got != again {
		t.Fatalf("resurrected key at %q, want at end %q", got, again)
	}
}

func TestDictUnknownOrderFallback(t *testing.T) {
	// A dict built directly from a Go map (no Store) has no order, but every
	// pair must still be visible exactly once.
	d := &Dict{Pairs: make(map[string]DictPair)}
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		k := NewString(name)
		d.Pairs[DictKey(k)] = DictPair{Key: k, Value: NewInteger(1)}
	}
	keys := keyNames(d)
	if len(keys) != 5 {
		t.Fatalf("fallback iteration lost pairs: %v", keys)
	}
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k] {
			t.Fatalf("duplicate key %s in fallback iteration", k)
		}
		seen[k] = true
	}
}

func TestDictOrderPreservedByStoreFrom(t *testing.T) {
	src := filledDict()
	dst := NewDict()
	dst.StoreFrom(src)
	keysEqual(t, dst, []string{"zebra", "apple", "mango"})

	// Later stores override earlier keys without moving them.
	dst.Store(DictKey(NewString("apple")), NewString("apple"), NewInteger(9))
	dst.StoreFrom(filledDict())
	keysEqual(t, dst, []string{"zebra", "apple", "mango"})
	if v := dst.Pairs[DictKey(NewString("apple"))].Value; v.Inspect() != "2" {
		t.Fatalf("apple value after merge = %s, want 2", v.Inspect())
	}
}

// modelDict is a trivially correct insertion-ordered dict used as the
// reference in TestDictRandomOpsMatchModel.
type modelDict struct {
	keys []string
	vals map[string]int
}

func (m *modelDict) set(k string, v int) {
	if _, ok := m.vals[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.vals[k] = v
}

func (m *modelDict) del(k string) {
	if _, ok := m.vals[k]; !ok {
		return
	}
	delete(m.vals, k)
	for i, kk := range m.keys {
		if kk == k {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			break
		}
	}
}

// TestDictRandomOpsMatchModel drives Store/Delete/Clear/popitem-style removals
// at random, with a skewed mix that produces runs of newest-first, oldest-first
// and middle deletes (the head/tail/stale paths), and checks the dict against
// the reference after every step.
func TestDictRandomOpsMatchModel(t *testing.T) {
	for seed := int64(1); seed <= 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		d := NewDict()
		m := &modelDict{vals: map[string]int{}}
		for step := 0; step < 4000; step++ {
			k := fmt.Sprintf("k%d", rng.Intn(120))
			switch r := rng.Intn(100); {
			case r < 45:
				d.Store(DictStringKey(k), NewString(k), NewInteger(int64(step)))
				m.set(k, step)
			case r < 60:
				d.Delete(DictStringKey(k))
				m.del(k)
			case r < 72: // oldest
				if len(m.keys) > 0 {
					d.Delete(DictStringKey(m.keys[0]))
					m.del(m.keys[0])
				}
			case r < 84: // newest, like popitem
				if len(m.keys) > 0 {
					c, _, ok := d.LastInserted()
					if !ok || c != DictStringKey(m.keys[len(m.keys)-1]) {
						t.Fatalf("seed %d step %d: LastInserted = %q, want %q", seed, step, c, m.keys[len(m.keys)-1])
					}
					d.Delete(c)
					m.del(m.keys[len(m.keys)-1])
				}
			case r < 86:
				d.Clear()
				m = &modelDict{vals: map[string]int{}}
			case r < 96: // move_to_end (r < 91) or move to the front
				if len(m.keys) > 0 {
					ek := m.keys[rng.Intn(len(m.keys))]
					last := r < 91
					val := m.vals[ek]
					if !d.MoveToEnd(DictStringKey(ek), last) {
						t.Fatalf("seed %d step %d: MoveToEnd(%q) reported missing", seed, step, ek)
					}
					m.del(ek)
					m.vals[ek] = val
					if last {
						m.keys = append(m.keys, ek)
					} else {
						m.keys = append([]string{ek}, m.keys...)
					}
				}
				if d.MoveToEnd(DictStringKey("absent-key"), true) {
					t.Fatalf("seed %d step %d: MoveToEnd of an absent key must report false", seed, step)
				}
			case r < 98: // oldest via FirstInserted, as popitem(last=False)
				if len(m.keys) > 0 {
					c, _, ok := d.FirstInserted()
					if !ok || c != DictStringKey(m.keys[0]) {
						t.Fatalf("seed %d step %d: FirstInserted = %q, want %q", seed, step, c, m.keys[0])
					}
					d.Delete(c)
					m.del(m.keys[0])
				}
			default: // update an existing key: position must not change
				if len(m.keys) > 0 {
					ek := m.keys[rng.Intn(len(m.keys))]
					d.Store(DictStringKey(ek), NewString(ek), NewInteger(int64(step)))
					m.set(ek, step)
				}
			}
			got := d.OrderedKeys()
			if len(got) != len(m.keys) || len(d.Pairs) != len(m.keys) {
				t.Fatalf("seed %d step %d: len = %d/%d, want %d", seed, step, len(got), len(d.Pairs), len(m.keys))
			}
			for i, ck := range got {
				if ck != DictStringKey(m.keys[i]) {
					t.Fatalf("seed %d step %d: order[%d] = %q, want %q", seed, step, i, ck, m.keys[i])
				}
				if v := d.Pairs[ck].Value.(*Integer).IntValue(); int(v) != m.vals[m.keys[i]] {
					t.Fatalf("seed %d step %d: value for %s = %d, want %d", seed, step, m.keys[i], v, m.vals[m.keys[i]])
				}
			}
		}
	}
}

// TestDictDeleteIsNotQuadratic guards the O(1) delete: removing every key of a
// large dict newest-first, oldest-first and via popitem must stay fast.
func TestDictDeleteIsNotQuadratic(t *testing.T) {
	const n = 200000
	build := func() *Dict {
		d := NewDictSized(n)
		for i := 0; i < n; i++ {
			k := fmt.Sprintf("k%d", i)
			d.Store(DictStringKey(k), NewString(k), NewInteger(int64(i)))
		}
		return d
	}
	start := time.Now()
	d := build()
	for i := n - 1; i >= 0; i-- {
		d.Delete(DictStringKey(fmt.Sprintf("k%d", i)))
	}
	d = build()
	for i := 0; i < n; i++ {
		d.Delete(DictStringKey(fmt.Sprintf("k%d", i)))
	}
	d = build()
	for len(d.Pairs) > 0 {
		c, _, _ := d.LastInserted()
		d.Delete(c)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("3x%d deletes took %v: delete is no longer O(1) amortized", n, elapsed)
	}
}

// TestDictUnknownOrderKeysComeFirstSorted: pairs written straight into Pairs
// (Go code that bypasses Store) have no recorded position. They read back
// first, sorted, so mutating such a dict appends new keys after the old ones
// instead of ahead of them, and the result is deterministic.
func TestDictUnknownOrderKeysComeFirstSorted(t *testing.T) {
	d := &Dict{Pairs: map[string]DictPair{}}
	for _, k := range []string{"zeta", "alpha", "mid"} {
		d.Pairs[DictStringKey(k)] = DictPair{Key: NewString(k), Value: NewInteger(1)}
	}
	d.Store(DictStringKey("late"), NewString("late"), NewInteger(2))
	d.Store(DictStringKey("beta"), NewString("beta"), NewInteger(3))

	want := []string{"alpha", "mid", "zeta", "late", "beta"}
	for run := 0; run < 20; run++ { // map iteration is random; the result must not be
		var got []string
		for _, p := range d.OrderedPairs() {
			got = append(got, p.Key.(*String).StringValue())
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("run %d: order = %v, want %v", run, got, want)
		}
	}
	if c, _, ok := d.LastInserted(); !ok || c != DictStringKey("beta") {
		t.Fatalf("LastInserted = %q, want beta", c)
	}
	d.Delete(DictStringKey("alpha"))
	d.Delete(DictStringKey("beta"))
	d.Delete(DictStringKey("late"))
	if c, _, ok := d.LastInserted(); !ok || c != DictStringKey("zeta") {
		t.Fatalf("after deletes LastInserted = %q, want zeta (sorted-last unknown key)", c)
	}
	if got := d.OrderedKeys(); fmt.Sprint(got) != fmt.Sprint([]string{DictStringKey("mid"), DictStringKey("zeta")}) {
		t.Fatalf("keys after deletes = %v", got)
	}
}

// TestNewStringDictIsDeterministic: building from a Go map yields sorted,
// ordered, repeatable output.
func TestNewStringDictIsDeterministic(t *testing.T) {
	entries := map[string]Object{}
	for _, k := range []string{"q", "b", "x", "a", "m", "z", "c"} {
		entries[k] = NewInteger(1)
	}
	want := fmt.Sprint(NewStringDict(entries).OrderedKeys())
	for i := 0; i < 20; i++ {
		d := NewStringDict(entries)
		if got := fmt.Sprint(d.OrderedKeys()); got != want {
			t.Fatalf("run %d: %s != %s", i, got, want)
		}
		d.Store(DictStringKey("new"), NewString("new"), NewInteger(2))
		if keys := d.OrderedKeys(); keys[len(keys)-1] != DictStringKey("new") {
			t.Fatalf("new key must append last, got %v", keys)
		}
	}
}

// TestDictStoreKeepsOriginalKeyObject: updating an existing key keeps its
// position and its first key object, as Python does (d[1]=..; d[True]=.. leaves
// the key as 1).
func TestDictStoreKeepsOriginalKeyObject(t *testing.T) {
	d := NewDict()
	d.Store(DictKey(NewInteger(1)), NewInteger(1), NewString("a"))
	d.Store(DictKey(NewString("x")), NewString("x"), NewString("b"))
	d.Store(DictKey(NewBoolean(true)), NewBoolean(true), NewString("c")) // same canonical key as 1
	pairs := d.OrderedPairs()
	if len(pairs) != 2 {
		t.Fatalf("len = %d, want 2", len(pairs))
	}
	if _, ok := pairs[0].Key.(*Integer); !ok || pairs[0].Value.(*String).StringValue() != "c" {
		t.Fatalf("first pair = %v: want key 1 with updated value c", pairs[0])
	}
}

// TestDeepCopyDictKeepsOrder: deep copies (used for isolated environments)
// preserve insertion order, including after later mutation.
func TestDeepCopyDictKeepsOrder(t *testing.T) {
	d := NewDict()
	for _, k := range []string{"z", "a", "m"} {
		d.Store(DictStringKey(k), NewString(k), NewInteger(1))
	}
	c := deepCopyDict(d)
	c.Store(DictStringKey("b"), NewString("b"), NewInteger(2))
	got := fmt.Sprint(c.OrderedKeys())
	want := fmt.Sprint([]string{DictStringKey("z"), DictStringKey("a"), DictStringKey("m"), DictStringKey("b")})
	if got != want {
		t.Fatalf("copy order = %s, want %s", got, want)
	}
}

func fieldNamesOf(inst *Instance) []string {
	var out []string
	inst.RangeFields(func(name string, _ Object) bool {
		out = append(out, name)
		return true
	})
	return out
}

// TestInstanceFieldsKeepAssignmentOrder: fields past the inline ones used to
// iterate in random map order; deletes used to swap-remove. Both must follow
// assignment order, so vars(obj) / obj.__dict__ match Python.
func TestInstanceFieldsKeepAssignmentOrder(t *testing.T) {
	names := []string{"z", "a", "m", "q", "r", "s", "t", "u", "b"}
	for run := 0; run < 20; run++ {
		inst := &Instance{Class: &Class{Name: "C"}}
		for i, n := range names {
			inst.SetField(n, NewInteger(int64(i)))
		}
		inst.SetField("a", NewInteger(99)) // update keeps position
		if got := fieldNamesOf(inst); fmt.Sprint(got) != fmt.Sprint(names) {
			t.Fatalf("run %d: order = %v, want %v", run, got, names)
		}
	}
	// Deleting from the inline part, the middle and the overflow part keeps
	// the remaining order, and a later field still appends last.
	inst := &Instance{Class: &Class{Name: "C"}}
	model := append([]string{}, names...)
	for i, n := range names {
		inst.SetField(n, NewInteger(int64(i)))
	}
	for _, del := range []string{"z", "r", "b", "m"} {
		inst.DeleteField(del)
		for i, n := range model {
			if n == del {
				model = append(model[:i], model[i+1:]...)
				break
			}
		}
		if got := fieldNamesOf(inst); fmt.Sprint(got) != fmt.Sprint(model) {
			t.Fatalf("after deleting %s: order = %v, want %v", del, got, model)
		}
		inst.DeleteField("not-there") // no-op
	}
	inst.SetField("last", NewInteger(1))
	model = append(model, "last")
	if got := fieldNamesOf(inst); fmt.Sprint(got) != fmt.Sprint(model) {
		t.Fatalf("after append: order = %v, want %v", got, model)
	}
	if inst.FieldCount() != len(model) {
		t.Fatalf("FieldCount = %d, want %d", inst.FieldCount(), len(model))
	}
	for _, n := range model {
		if _, ok := inst.GetField(n); !ok {
			t.Fatalf("field %s lost", n)
		}
	}
}

// TestInstanceFieldsRandomOpsMatchModel drives SetField/DeleteField at random
// against a slice model, across the inline/overflow boundary.
func TestInstanceFieldsRandomOpsMatchModel(t *testing.T) {
	for seed := int64(1); seed <= 30; seed++ {
		rng := rand.New(rand.NewSource(seed))
		inst := &Instance{Class: &Class{Name: "C"}}
		var model []string
		vals := map[string]int{}
		for step := 0; step < 600; step++ {
			n := fmt.Sprintf("f%d", rng.Intn(14))
			if rng.Intn(100) < 60 {
				if _, ok := vals[n]; !ok {
					model = append(model, n)
				}
				vals[n] = step
				inst.SetField(n, NewInteger(int64(step)))
			} else {
				if _, ok := vals[n]; ok {
					delete(vals, n)
					for i, m := range model {
						if m == n {
							model = append(model[:i], model[i+1:]...)
							break
						}
					}
				}
				inst.DeleteField(n)
			}
			if got := fieldNamesOf(inst); fmt.Sprint(got) != fmt.Sprint(model) {
				t.Fatalf("seed %d step %d: order = %v, want %v", seed, step, got, model)
			}
			for _, m := range model {
				v, ok := inst.GetField(m)
				if !ok || int(v.(*Integer).IntValue()) != vals[m] {
					t.Fatalf("seed %d step %d: field %s wrong", seed, step, m)
				}
			}
		}
	}
}

// TestDictOwnerWriteThrough: a dict linked to an instance (obj.__dict__) writes
// stores, deletes and clears through to the instance's fields; copies are not
// linked, and non-string keys stay in the view only.
func TestDictOwnerWriteThrough(t *testing.T) {
	inst := &Instance{Class: &Class{Name: "C"}}
	inst.SetField("a", NewInteger(1))
	view := NewDict()
	view.SetByString("a", NewInteger(1))
	view.LinkOwner(inst)

	view.SetByString("b", NewInteger(2))
	view.Store(DictKey(NewString("a")), NewString("a"), NewInteger(10))
	if v, _ := inst.GetField("b"); v == nil || v.(*Integer).IntValue() != 2 {
		t.Fatal("new key did not write through")
	}
	if v, _ := inst.GetField("a"); v.(*Integer).IntValue() != 10 {
		t.Fatal("update did not write through")
	}
	view.Store(DictKey(NewInteger(5)), NewInteger(5), NewInteger(5)) // not an attribute name
	if inst.FieldCount() != 2 {
		t.Fatalf("non-string key leaked into the instance: %v", fieldNamesOf(inst))
	}
	view.DeleteByString("a")
	if inst.HasField("a") {
		t.Fatal("delete did not write through")
	}

	unlinked := NewDictSized(2)
	unlinked.StoreFrom(view)
	unlinked.SetByString("zzz", NewInteger(1))
	if inst.HasField("zzz") {
		t.Fatal("a copy must not be linked to the instance")
	}

	view.Clear()
	if inst.FieldCount() != 0 {
		t.Fatalf("clear left fields: %v", fieldNamesOf(inst))
	}
}
