package scriptling

import (
	"fmt"
	"sync"
	"testing"
)

// TestMultipleScriptlingInstances verifies that multiple Scriptling instances
// can coexist without interfering with each other (no global state issues)
func TestMultipleScriptlingInstances(t *testing.T) {
	// Create two separate Scriptling instances
	s1 := New()
	s2 := New()

	// Register different libraries in each
	s1.RegisterScriptLibrary("lib1", `
def func1():
    return "from lib1"
`)

	s2.RegisterScriptLibrary("lib2", `
def func2():
    return "from lib2"
`)

	// Test that s1 can import lib1 but not lib2
	result, err := s1.Eval(`
import lib1
lib1.func1()
`)
	if err != nil {
		t.Fatalf("s1 failed to import lib1: %v", err)
	}
	if result.Inspect() != "from lib1" {
		t.Errorf("s1: expected 'from lib1', got '%s'", result.Inspect())
	}

	// s1 should not have lib2
	_, err = s1.Eval("import lib2")
	if err == nil {
		t.Error("s1 should not have access to lib2")
	}

	// Test that s2 can import lib2 but not lib1
	result, err = s2.Eval(`
import lib2
lib2.func2()
`)
	if err != nil {
		t.Fatalf("s2 failed to import lib2: %v", err)
	}
	if result.Inspect() != "from lib2" {
		t.Errorf("s2: expected 'from lib2', got '%s'", result.Inspect())
	}

	// s2 should not have lib1
	_, err = s2.Eval("import lib1")
	if err == nil {
		t.Error("s2 should not have access to lib1")
	}
}

// TestConcurrentEval tests that multiple Scriptling instances can be created and evaluated concurrently
func TestConcurrentEval(t *testing.T) {
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Run 10 concurrent evaluations, each with its own Scriptling instance
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s := New()
			s.SetVar("base", 100)
			result, err := s.Eval("base + 1")
			if err != nil {
				errors <- err
				return
			}
			if result.Inspect() != "101" {
				errors <- fmt.Errorf("expected '101', got '%s'", result.Inspect())
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			t.Errorf("concurrent eval error: %v", err)
		}
	}
}
