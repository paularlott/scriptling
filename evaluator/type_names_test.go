package evaluator

import "testing"

// TestTypeNamesMatchPython: type(x) reports Python's type(x).__name__.
func TestTypeNamesMatchPython(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{`1`, "int"},
		{`1.5`, "float"},
		{`"a"`, "str"},
		{`b"a"`, "bytes"},
		{`True`, "bool"},
		{`None`, "NoneType"},
		{`[]`, "list"},
		{`()`, "tuple"},
		{`{}`, "dict"},
		{`set()`, "set"},
		{`frozenset()`, "frozenset"},
		{`slice(1, 2)`, "slice"},
		{`{}.keys()`, "dict_keys"},
		{`{}.values()`, "dict_values"},
		{`{}.items()`, "dict_items"},
		{`range(3)`, "range"},
		{`map(str, [])`, "map"},
		{`filter(None, [])`, "filter"},
		{`zip()`, "zip"},
		{`enumerate([])`, "enumerate"},
		{`reversed([])`, "list_reverseiterator"},
		{`reversed(())`, "reversed"},
		{`reversed(range(3))`, "range_iterator"},
		{`reversed({})`, "dict_reversekeyiterator"},
		{`iter([])`, "list_iterator"},
		{`iter(())`, "tuple_iterator"},
		{`iter("ab")`, "str_ascii_iterator"},
		{`iter(set())`, "set_iterator"},
		{`iter({})`, "dict_keyiterator"},
		{`iter({}.values())`, "dict_valueiterator"},
		{`iter({}.items())`, "dict_itemiterator"},
		{`len`, "builtin_function_or_method"},
		{`int`, "type"},
		{`ValueError`, "type"},
		{`lambda: 1`, "function"},
		{`ValueError("x")`, "ValueError"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := testEval("type(" + tt.expr + ").__name__")
			if got.Inspect() != tt.want {
				t.Errorf("type(%s).__name__ = %s, want %s", tt.expr, got.Inspect(), tt.want)
			}
		})
	}
}

func TestTypeNamesOfDefinitions(t *testing.T) {
	script := `
def f():
    pass
class C:
    def m(self):
        pass
[type(f), type(C), type(C()), type(C().m)]`
	got := testEval(script).Inspect()
	want := "['function', 'type', 'C', 'method']"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// TestTypeNameComparisons: type() returns Python's names, compared exactly;
// the upper-case names it returned before 0.32.0 no longer match.
func TestTypeNameComparisons(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{`type(1) == "int"`, "True"},
		{`type(1) == "INTEGER"`, "False"},
		{`type(None) == "NULL"`, "False"},
		{`type(range(3)) == "ITERATOR"`, "False"},
		{`type(1) == int`, "True"},
		{`type(1) is int`, "True"},
		{`type(1) != str`, "True"},
		{`type(True) == bool`, "True"},
		{`type(1) == type(2)`, "True"},
		{`type(1) == type("a")`, "False"},
		{`type(range(1)) == type(map(str, []))`, "False"},
		{`type(1.5) in ("int", "float")`, "True"},
		{`"INTEGER" == int`, "False"},
		{`"STR" == str`, "False"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := testEval(tt.expr)
			if got.Inspect() != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got.Inspect(), tt.want)
			}
		})
	}
}

// TestIsinstanceTypeNames: isinstance() takes a type name as type() reports
// it, exactly, including subtypes.
func TestIsinstanceTypeNames(t *testing.T) {
	script := `
class Base:
    pass
class Child(Base):
    pass
[isinstance(1, "int"), isinstance(True, "int"), isinstance(True, "bool"),
 isinstance(None, "NoneType"), isinstance([], "list"), isinstance(Child(), "Base"),
 isinstance(1, "INTEGER"), isinstance(1, "Int"), isinstance(Child(), "base"),
 isinstance(None, "None"), isinstance(1, "str"), isinstance(1, ("str", "int"))]`
	got := testEval(script).Inspect()
	want := "[True, True, True, True, True, True, False, False, False, False, False, True]"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestTypeNameMatchCase(t *testing.T) {
	script := `
def kind(x):
    match type(x):
        case "int":
            return "int"
        case "INTEGER":
            return "legacy"
        case _:
            return "other"
[kind(1), kind("a")]`
	got := testEval(script).Inspect()
	if got != "['int', 'other']" {
		t.Errorf("got %s", got)
	}
}
