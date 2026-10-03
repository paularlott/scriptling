package evaluator

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// Every name in builtinMethodNames must reach a real method in the type's
// dispatcher: a wrong argument count is fine, "has no attribute" is not.
func TestBuiltinMethodTablesMatchDispatch(t *testing.T) {
	samples := map[object.ObjectType]object.Object{
		object.STRING_OBJ:      object.NewString("ab"),
		object.LIST_OBJ:        &object.List{Elements: []object.Object{object.NewInteger(1)}},
		object.DICT_OBJ:        &object.Dict{Pairs: map[string]object.DictPair{}},
		object.TUPLE_OBJ:       &object.Tuple{Elements: []object.Object{object.NewInteger(1)}},
		object.SET_OBJ:         object.NewSet(),
		object.BYTES_OBJ:       object.NewBytes([]byte("ab")),
		object.FLOAT_ARRAY_OBJ: object.NewFloatArray1D([]float64{1}),
	}
	env := object.NewEnvironment()
	ctx := SetEnvInContext(context.Background(), env)
	for typ, names := range builtinMethodNames {
		sample, ok := samples[typ]
		if !ok {
			t.Fatalf("no sample value for %s", typ)
		}
		for _, name := range names {
			result := callStringMethodWithKeywords(ctx, sample, name, nil, nil, env)
			if exc, ok := result.(*object.Exception); ok && strings.Contains(exc.Message, "has no attribute") {
				t.Errorf("%s.%s is listed but not dispatched: %s", typ, name, exc.Message)
			}
		}
	}
}

func TestBuiltinMethodReferences(t *testing.T) {
	cases := map[string]string{
		`sorted(["b", "A", "c"], key=str.lower)`:             `['A', 'b', 'c']`,
		`list(map(str.strip, [" a ", "b "]))`:                `['a', 'b']`,
		"items = []\ncb = items.append\ncb(1)\ncb(2)\nitems": `[1, 2]`,
		"d = {'a': 1}\nk = d.keys\nlist(k())":                `['a']`,
		`dict.fromkeys(["x"], 0)`:                            `{'x': 0}`,
		`str.join(",", ["a", "b"])`:                          `a,b`,
		`[hasattr([], "append"), hasattr("", "lower"), hasattr([], "nope"), hasattr({}, "keys")]`: `[True, True, False, True]`,
		`dir([])[:3]`:                         `['append', 'clear', 'copy']`,
		`"Hi".lower`:                          `<built-in method lower of str object>`,
		`str.lower`:                           `<method 'lower' of 'str' objects>`,
		"s = {1}\na = s.add\na(2)\nsorted(s)": `[1, 2]`,
	}
	for script, want := range cases {
		got := testEval(script)
		if got == nil || got.Inspect() != want {
			t.Errorf("%s\n got %v, want %s", script, got, want)
		}
	}

	got := testEval("try:\n    str.lower(5)\nexcept TypeError as e:\n    r = str(e)\nr")
	if got == nil || got.Inspect() != "descriptor 'lower' for 'str' objects doesn't apply to a 'int' object" {
		t.Errorf("unbound type check: got %v", got)
	}
}
