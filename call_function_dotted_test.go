package scriptling

import (
	"testing"
)

func TestCallFunctionWithDottedPath(t *testing.T) {
	t.Run("call_library_function", func(t *testing.T) {
		p := New()
		
		// Register a script library with a function
		err := p.RegisterScriptLibrary("mylib", `
def testHandler(x):
    return x * 2
`)
		if err != nil {
			t.Fatalf("RegisterScriptLibrary failed: %v", err)
		}
		
		// Import the library
		if err := p.Import("mylib"); err != nil {
			t.Fatalf("Import failed: %v", err)
		}
		
		// Call function using dotted path
		result, err := p.CallFunction("mylib.testHandler", 21)
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		
		val, objErr := result.AsInt()
		if objErr != nil {
			t.Fatalf("result is not an int: %v", objErr)
		}
		if val != 42 {
			t.Errorf("expected 42, got %d", val)
		}
	})
	
	t.Run("call_nested_library_function", func(t *testing.T) {
		p := New()
		
		// Register nested script libraries
		err := p.RegisterScriptLibrary("parent", `
def parent_func():
    return "parent"
`)
		if err != nil {
			t.Fatalf("RegisterScriptLibrary parent failed: %v", err)
		}
		
		err = p.RegisterScriptLibrary("parent.child", `
def child_func():
    return "child"
`)
		if err != nil {
			t.Fatalf("RegisterScriptLibrary child failed: %v", err)
		}
		
		// Import both
		if err := p.Import("parent"); err != nil {
			t.Fatalf("Import parent failed: %v", err)
		}
		if err := p.Import("parent.child"); err != nil {
			t.Fatalf("Import child failed: %v", err)
		}
		
		// Call nested function
		result, err := p.CallFunction("parent.child.child_func")
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		
		val, objErr := result.AsString()
		if objErr != nil {
			t.Fatalf("result is not a string: %v", objErr)
		}
		if val != "child" {
			t.Errorf("expected 'child', got '%s'", val)
		}
	})
	
	t.Run("error_function_not_found", func(t *testing.T) {
		p := New()
		
		_, err := p.CallFunction("nonexistent.func")
		if err == nil {
			t.Error("Expected error for nonexistent function")
		}
	})
	
	t.Run("error_not_a_module", func(t *testing.T) {
		p := New()
		
		// Set a non-dict value
		p.SetVar("notamodule", 42)
		
		_, err := p.CallFunction("notamodule.func")
		if err == nil {
			t.Error("Expected error when traversing non-module")
		}
	})
}
