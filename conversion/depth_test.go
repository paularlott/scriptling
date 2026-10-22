package conversion

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func nestedGoSlice(n int) interface{} {
	var v interface{} = int64(1)
	for i := 0; i < n; i++ {
		v = []interface{}{v}
	}
	return v
}

func nestedGoMap(n int) interface{} {
	var v interface{} = int64(1)
	for i := 0; i < n; i++ {
		v = map[string]interface{}{"a": v}
	}
	return v
}

func nestedGoAnyMap(n int) interface{} {
	var v interface{} = int64(1)
	for i := 0; i < n; i++ {
		v = map[interface{}]interface{}{"a": v}
	}
	return v
}

func nestedList(n int) object.Object {
	var v object.Object = object.NewInteger(1)
	for i := 0; i < n; i++ {
		v = &object.List{Elements: []object.Object{v}}
	}
	return v
}

func isRecursionError(obj object.Object) bool {
	e, ok := obj.(*object.Error)
	return ok && e.ExceptionType == "RecursionError" && strings.Contains(e.Message, "too deeply nested")
}

// FromGo recursed without bound, so a deep Go value overflowed the stack, a
// fatal error that ended the process. Past MaxFromGoDepth it now returns a
// RecursionError (CWE-674, vuln-0018).
func TestFromGoRejectsDeepNesting(t *testing.T) {
	for name, v := range map[string]interface{}{
		"slices":         nestedGoSlice(5000000),
		"string maps":    nestedGoMap(MaxFromGoDepth + 1),
		"any-key maps":   nestedGoAnyMap(MaxFromGoDepth + 1),
		"slice at cap+1": nestedGoSlice(MaxFromGoDepth + 1),
	} {
		if got := FromGo(v); !isRecursionError(got) {
			t.Errorf("%s: expected a RecursionError, got %T %.80s", name, got, got.Inspect())
		}
	}
}

// A value MaxFromGoDepth containers deep converts in full.
func TestFromGoDepthBoundary(t *testing.T) {
	for name, v := range map[string]interface{}{
		"slices":       nestedGoSlice(MaxFromGoDepth),
		"string maps":  nestedGoMap(MaxFromGoDepth),
		"any-key maps": nestedGoAnyMap(MaxFromGoDepth),
	} {
		obj := FromGo(v)
		if object.IsError(obj) {
			t.Fatalf("%s at the cap should convert: %s", name, obj.Inspect())
		}
		depth := 0
		for {
			switch o := obj.(type) {
			case *object.List:
				obj = o.Elements[0]
			case *object.Dict:
				obj = o.Pairs[object.DictStringKey("a")].Value
			default:
				goto done
			}
			depth++
		}
	done:
		if depth != MaxFromGoDepth || obj.Inspect() != "1" {
			t.Fatalf("%s: converted %d levels to %s, want %d levels to 1", name, depth, obj.Inspect(), MaxFromGoDepth)
		}
	}
}

// Shallow values convert as before.
func TestFromGoShallowUnchanged(t *testing.T) {
	got := FromGo(map[string]interface{}{
		"b": []interface{}{int64(1), "x", nil, true},
		"a": map[interface{}]interface{}{2: 2.5},
	})
	if want := "{'a': {'2': 2.5}, 'b': [1, 'x', None, True]}"; got.Inspect() != want {
		t.Fatalf("got %s, want %s", got.Inspect(), want)
	}
}

// ToGo silently truncated a deep value to a placeholder, so serializers wrote
// corrupt output. ToGoChecked raises a RecursionError instead; ToGo itself is
// unchanged.
func TestToGoCheckedRejectsDeepNesting(t *testing.T) {
	deep := nestedList(maxToGoDepth + 1)
	if _, err := ToGoChecked(deep); err == nil || !isRecursionError(err) {
		t.Fatalf("expected a RecursionError, got %v", err)
	}
	if _, err := ToGoChecked(nestedList(1000000)); err == nil || !isRecursionError(err) {
		t.Fatalf("expected a RecursionError for a very deep value, got %v", err)
	}

	// At the cap the value converts in full.
	v, err := ToGoChecked(nestedList(maxToGoDepth))
	if err != nil {
		t.Fatalf("a value at the cap should convert: %v", err.Inspect())
	}
	for depth := 0; depth < maxToGoDepth; depth++ {
		v = v.([]interface{})[0]
	}
	if v != int64(1) {
		t.Fatalf("innermost value = %v, want 1", v)
	}

	// ToGo keeps its placeholder behaviour.
	if !strings.Contains(toGoInnermost(ToGo(deep)), tooDeepPlaceholder) {
		t.Fatalf("ToGo should still truncate to %q", tooDeepPlaceholder)
	}
}

// A cycle is not depth: ToGoChecked keeps the cyclic placeholder and applies
// no size limit.
func TestToGoCheckedCyclicAndLarge(t *testing.T) {
	l := &object.List{Elements: []object.Object{object.NewInteger(1)}}
	l.Elements = append(l.Elements, l)
	v, err := ToGoChecked(l)
	if err != nil {
		t.Fatalf("cyclic value: %v", err.Inspect())
	}
	if got := v.([]interface{})[1]; got != cyclicRefPlaceholder {
		t.Fatalf("cycle = %v, want the placeholder", got)
	}

	wide := make([]object.Object, 100000)
	for i := range wide {
		wide[i] = object.NewInteger(int64(i))
	}
	v, err = ToGoChecked(&object.List{Elements: wide})
	if err != nil || len(v.([]interface{})) != len(wide) {
		t.Fatalf("a wide value should convert in full: %v", err)
	}
}

func toGoInnermost(v interface{}) string {
	for {
		s, ok := v.([]interface{})
		if !ok {
			str, _ := v.(string)
			return str
		}
		v = s[0]
	}
}
