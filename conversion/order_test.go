package conversion

import (
	"fmt"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func keyNames(d *object.Dict) []string {
	var out []string
	for _, p := range d.OrderedPairs() {
		out = append(out, p.Key.(*object.String).StringValue())
	}
	return out
}

// TestParseJSONKeepsDocumentOrder: objects decode in the document's key order,
// at every nesting level, and later mutation appends.
func TestParseJSONKeepsDocumentOrder(t *testing.T) {
	doc := `{"zeta": 1, "alpha": {"y": 1, "b": [ {"q": 1, "a": 2} ]}, "mid": 3, "beta": 4, "omega": 5, "chi": 6, "psi": 7}`
	for run := 0; run < 20; run++ {
		obj, err := ParseJSON(doc)
		if err != nil {
			t.Fatal(err)
		}
		d := obj.(*object.Dict)
		want := "[zeta alpha mid beta omega chi psi]"
		if got := fmt.Sprint(keyNames(d)); got != want {
			t.Fatalf("run %d: top-level order = %s, want %s", run, got, want)
		}
		inner := d.Pairs[object.DictStringKey("alpha")].Value.(*object.Dict)
		if got := fmt.Sprint(keyNames(inner)); got != "[y b]" {
			t.Fatalf("nested order = %s", got)
		}
		leaf := inner.Pairs[object.DictStringKey("b")].Value.(*object.List).Elements[0].(*object.Dict)
		if got := fmt.Sprint(keyNames(leaf)); got != "[q a]" {
			t.Fatalf("list-nested order = %s", got)
		}
		d.Store(object.DictStringKey("new"), object.NewString("new"), object.NewInteger(1))
		if got := keyNames(d); got[len(got)-1] != "new" {
			t.Fatalf("mutation after decode must append, got %v", got)
		}
	}
}

// TestParseJSONDuplicateKeys: Python keeps the first position and the last value.
func TestParseJSONDuplicateKeys(t *testing.T) {
	obj, err := ParseJSON(`{"a": 1, "b": 2, "a": 3}`)
	if err != nil {
		t.Fatal(err)
	}
	d := obj.(*object.Dict)
	if got := fmt.Sprint(keyNames(d)); got != "[a b]" {
		t.Fatalf("order = %s", got)
	}
	if v := d.Pairs[object.DictStringKey("a")].Value.(*object.Integer).IntValue(); v != 3 {
		t.Fatalf("a = %d, want last value 3", v)
	}
}

// TestParseJSONErrors: malformed documents still error rather than panic.
func TestParseJSONErrors(t *testing.T) {
	for _, bad := range []string{``, `{"a": 1`, `{"a" 1}`, `[1,`, `{1: 2}`, `[`, `{`, `{"a":}`} {
		if _, err := ParseJSON(bad); err == nil {
			t.Errorf("ParseJSON(%q) = nil error, want an error", bad)
		}
	}
}

// TestFromGoMapIsDeterministic: Go maps have no order, so conversions sort.
func TestFromGoMapIsDeterministic(t *testing.T) {
	m := map[string]interface{}{"q": 1, "b": 2, "x": 3, "a": 4, "m": 5, "z": 6}
	want := "[a b m q x z]"
	for i := 0; i < 20; i++ {
		if got := fmt.Sprint(keyNames(FromGo(m).(*object.Dict))); got != want {
			t.Fatalf("run %d: %s, want %s", i, got, want)
		}
	}
	mi := map[interface{}]interface{}{"q": 1, "b": 2, 3: 3, "a": 4}
	for i := 0; i < 20; i++ {
		if got := fmt.Sprint(keyNames(FromGo(mi).(*object.Dict))); got != "[3 a b q]" {
			t.Fatalf("run %d: %s", i, got)
		}
	}
}
