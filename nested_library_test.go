package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

func TestNestedLibraryDepth(t *testing.T) {
	t.Run("reasonable_nesting_works", func(t *testing.T) {
		p := New()

		// Create 3-level nesting
		p.RegisterScriptLibrary("level1", `
def func1():
    return "level1"
`)
		p.RegisterScriptLibrary("level1.level2", `
def func2():
    return "level2"
`)
		p.RegisterScriptLibrary("level1.level2.level3", `
def func3():
    return "level3"
`)

		// Import deepest level (should load parents automatically)
		err := p.Import("level1.level2.level3")
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		// Verify all levels are accessible
		_, err = p.Eval(`
result1 = level1.func1()
result2 = level1.level2.func2()
result3 = level1.level2.level3.func3()
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
	})

	t.Run("import_order_independence", func(t *testing.T) {
		p := New()

		p.RegisterScriptLibrary("a", `X = 1`)
		p.RegisterScriptLibrary("a.b", `Y = 2`)

		// Import child first, then parent
		if err := p.Import("a.b"); err != nil {
			t.Fatalf("Import a.b failed: %v", err)
		}
		if err := p.Import("a"); err != nil {
			t.Fatalf("Import a failed: %v", err)
		}

		// Both should be accessible
		_, err := p.Eval(`
x = a.X
y = a.b.Y
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}

		x, _ := p.GetVarAsInt("x")
		y, _ := p.GetVarAsInt("y")

		if x != 1 || y != 2 {
			t.Errorf("Expected x=1, y=2, got x=%d, y=%d", x, y)
		}
	})

	t.Run("nested_library_in_script", func(t *testing.T) {
		p := New()
		stdlib.RegisterAll(p)

		// Register a library that imports another library
		p.RegisterScriptLibrary("outer", `
import json

def parse_json(s):
    return json.loads(s)
`)

		// Import and use
		_, err := p.Eval(`
import outer
result = outer.parse_json('{"key": "value"}')
value = result["key"]
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}

		value, _ := p.GetVarAsString("value")
		if value != "value" {
			t.Errorf("Expected 'value', got '%s'", value)
		}
	})
}

func TestLibraryLoadingEdgeCases(t *testing.T) {
	t.Run("reimport_same_library", func(t *testing.T) {
		p := New()
		p.RegisterScriptLibrary("lib", `X = 1`)

		// Import twice - should not error
		if err := p.Import("lib"); err != nil {
			t.Fatalf("First import failed: %v", err)
		}
		if err := p.Import("lib"); err != nil {
			t.Fatalf("Second import failed: %v", err)
		}
	})

	t.Run("parent_and_child_both_have_functions", func(t *testing.T) {
		p := New()

		p.RegisterScriptLibrary("parent", `
def parent_func():
    return "parent"
`)
		p.RegisterScriptLibrary("parent.child", `
def child_func():
    return "child"
`)

		// Import child first
		if err := p.Import("parent.child"); err != nil {
			t.Fatalf("Import child failed: %v", err)
		}

		// Then import parent - should merge properly
		if err := p.Import("parent"); err != nil {
			t.Fatalf("Import parent failed: %v", err)
		}

		// Both should work
		_, err := p.Eval(`
p = parent.parent_func()
c = parent.child.child_func()
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
	})

	t.Run("dotted_import_creates_structure", func(t *testing.T) {
		p := New()

		p.RegisterScriptLibrary("a.b.c", `VALUE = 42`)

		if err := p.Import("a.b.c"); err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		// Should be able to access via full path
		_, err := p.Eval(`result = a.b.c.VALUE`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}

		val, _ := p.GetVarAsInt("result")
		if val != 42 {
			t.Errorf("Expected 42, got %d", val)
		}
	})
}

func TestLibraryErrorHandling(t *testing.T) {
	t.Run("unknown_library", func(t *testing.T) {
		p := New()

		err := p.Import("nonexistent")
		if err == nil {
			t.Error("Expected error for unknown library")
		}
		if !strings.Contains(err.Error(), "unknown library") {
			t.Errorf("Expected 'unknown library' error, got: %v", err)
		}
	})

	t.Run("library_with_syntax_error", func(t *testing.T) {
		p := New()

		err := p.RegisterScriptLibrary("bad", `
def broken(
    # Missing closing paren
`)
		if err != nil {
			// Registration might fail immediately
			return
		}

		// Or fail on import
		err = p.Import("bad")
		if err == nil {
			t.Error("Expected error for library with syntax error")
		}
	})
}

func TestDepthLimits(t *testing.T) {
	t.Run("dotted_path_depth_limit", func(t *testing.T) {
		p := New()

		// Create nested libraries to test CallFunction depth (11 levels)
		p.RegisterScriptLibrary("a.b.c.d.e.f.g.h.i.j.k", `
def deep_func():
    return "too deep"
`)

		// Import should fail due to depth
		err := p.Import("a.b.c.d.e.f.g.h.i.j.k")
		if err == nil {
			t.Error("Expected error for path exceeding depth limit")
			return
		}
		if !strings.Contains(err.Error(), "too deep") {
			t.Errorf("Expected 'too deep' error, got: %v", err)
		}
	})

	t.Run("library_nesting_depth_limit", func(t *testing.T) {
		p := New()

		// Try to create 7-level nesting (6 dots, should exceed limit of 5)
		err := p.RegisterScriptLibrary("l1.l2.l3.l4.l5.l6.l7", `X = 1`)
		if err != nil {
			t.Fatalf("RegisterScriptLibrary failed: %v", err)
		}

		err = p.Import("l1.l2.l3.l4.l5.l6.l7")
		if err == nil {
			t.Error("Expected error for library nesting exceeding depth limit")
			return
		}
		if !strings.Contains(err.Error(), "too deep") {
			t.Errorf("Expected 'too deep' error, got: %v", err)
		}
	})

	t.Run("reasonable_depth_works", func(t *testing.T) {
		p := New()

		// 5 levels should work fine
		p.RegisterScriptLibrary("l1.l2.l3.l4.l5", `X = 42`)

		err := p.Import("l1.l2.l3.l4.l5")
		if err != nil {
			t.Fatalf("5-level nesting should work: %v", err)
		}

		_, err = p.Eval(`result = l1.l2.l3.l4.l5.X`)
		if err != nil {
			t.Fatalf("Accessing 5-level nested value should work: %v", err)
		}

		val, _ := p.GetVarAsInt("result")
		if val != 42 {
			t.Errorf("Expected 42, got %d", val)
		}
	})
}

// TestScriptLibraryChildBeforeParentImport verifies that when a script library
// imports a child module (e.g., a.b.c) before its parent (e.g., a.b), the parent's
// functions are still accessible. This was a bug where the intermediate dict created
// for the child path was falsely treated as "already imported" for the parent.
func TestScriptLibraryChildBeforeParentImport(t *testing.T) {
	t.Run("script_imports_child_then_parent", func(t *testing.T) {
		p := New()

		// Register parent library with a function
		p.RegisterScriptLibrary("ns.parent", `
def greet():
    return "hello"
`)

		// Register child library
		p.RegisterScriptLibrary("ns.parent.child", `
def farewell():
    return "goodbye"
`)

		// Register a script library that imports child FIRST, then parent
		p.RegisterScriptLibrary("consumer", `
import ns.parent.child as child_mod
import ns.parent as parent_mod

# Both should be accessible
child_result = child_mod.farewell()
parent_result = parent_mod.greet()
`)

		_, err := p.Eval(`
import consumer
cr = consumer.child_result
pr = consumer.parent_result
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}

		cr, _ := p.GetVarAsString("cr")
		pr, _ := p.GetVarAsString("pr")
		if cr != "goodbye" {
			t.Errorf("Expected child_result='goodbye', got '%s'", cr)
		}
		if pr != "hello" {
			t.Errorf("Expected parent_result='hello', got '%s'", pr)
		}
	})

	t.Run("script_imports_child_then_parent_method_call", func(t *testing.T) {
		p := New()

		// Register parent library with a function
		p.RegisterScriptLibrary("ns.parent", `
def greet(name):
    return "hello " + name
`)

		// Register child library
		p.RegisterScriptLibrary("ns.parent.child", `
def farewell():
    return "bye"
`)

		// Register a consumer that imports child first, then calls parent method
		p.RegisterScriptLibrary("caller", `
import ns.parent.child as child_mod
import ns.parent as parent_mod

def call_parent():
    return parent_mod.greet("world")

def call_both():
    return parent_mod.greet("x") + " " + child_mod.farewell()
`)

		_, err := p.Eval(`
import caller
r1 = caller.call_parent()
r2 = caller.call_both()
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}

		r1, _ := p.GetVarAsString("r1")
		r2, _ := p.GetVarAsString("r2")
		if r1 != "hello world" {
			t.Errorf("Expected 'hello world', got '%s'", r1)
		}
		if r2 != "hello x bye" {
			t.Errorf("Expected 'hello x bye', got '%s'", r2)
		}
	})

	t.Run("three_level_child_before_parent_in_script", func(t *testing.T) {
		p := New()
		stdlib.RegisterAll(p)

		// Simulates the scriptlingcoder pattern: import a.b.c.d then a.b
		p.RegisterScriptLibrary("a.b", `
def func_b():
    return "from_b"
`)
		p.RegisterScriptLibrary("a.b.c", `
import a.b as parent_b

def func_c():
    return parent_b.func_b()
`)
		p.RegisterScriptLibrary("a.b.c.d", `
import a.b.c as mod_c
import a.b as mod_b

def func_d():
    return mod_b.func_b() + " " + mod_c.func_c()
`)

		_, err := p.Eval(`
import a.b as ab, a.b.c.d as abcd
r = abcd.func_d()
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}

		r, _ := p.GetVarAsString("r")
		if r != "from_b from_b" {
			t.Errorf("Expected 'from_b from_b', got '%s'", r)
		}
	})
}
