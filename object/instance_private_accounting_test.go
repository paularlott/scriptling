package object

import "testing"

// An instance that leaves the shared layouts before it has any field (its
// class has used up its shared layouts) must not fault when memory
// accounting is on.
func TestInstancePrivateLayoutFromEmptyWithAccounting(t *testing.T) {
	was := accountingOn.Load()
	accountingOn.Store(true)
	defer accountingOn.Store(was)

	class := &Class{Name: "Many", Methods: map[string]Object{}}
	// Use up the class's shared layouts with instances that each start with
	// a different first field.
	for n := 0; n < 200; n++ {
		inst := NewInstance(class)
		inst.SetField("f"+string(rune('a'+n%26))+string(rune('a'+n/26)), NewInteger(int64(n)))
		inst.SetField("second", NewInteger(1))
		if v, ok := inst.GetField("second"); !ok || v.Inspect() != "1" {
			t.Fatalf("instance %d lost a field", n)
		}
	}
}

// Instances built from a Go map get their fields in name order, whatever
// order the map yields them in.
func TestNewInstanceWithFieldsOrder(t *testing.T) {
	class := &Class{Name: "T", Methods: map[string]Object{}}
	for n := 0; n < 20; n++ {
		inst := NewInstanceWithFields(class, map[string]Object{
			"d": NewInteger(4), "b": NewInteger(2), "a": NewInteger(1), "c": NewInteger(3), "e": NewInteger(5),
		})
		got := ""
		inst.RangeFields(func(name string, _ Object) bool {
			got += name
			return true
		})
		if got != "abcde" {
			t.Fatalf("field order %q, want abcde", got)
		}
	}
}
