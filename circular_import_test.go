package scriptling

import (
	"strings"
	"testing"
)

// TestCircularImportDetected pins the import machinery's cycle detection: a
// module's exported names only exist once its evaluation completes, so
// without the check a pair of modules importing each other re-evaluates
// forever (a hang that ends in stack exhaustion). A cycle must fail fast
// with the chain named; ordinary chains still load.
func TestCircularImportDetected(t *testing.T) {
	sl := New()
	if err := sl.RegisterScriptLibrary("cyc_a", "import cyc_b\na_value = 1\n"); err != nil {
		t.Fatalf("register cyc_a: %v", err)
	}
	if err := sl.RegisterScriptLibrary("cyc_b", "import cyc_a\nb_value = 2\n"); err != nil {
		t.Fatalf("register cyc_b: %v", err)
	}

	_, err := sl.Eval("import cyc_a")
	if err == nil {
		t.Fatal("expected a circular import error")
	}
	if !strings.Contains(err.Error(), "circular import") || !strings.Contains(err.Error(), "cyc_a -> cyc_b -> cyc_a") {
		t.Fatalf("expected the chain to be named, got: %v", err)
	}
}

// TestSelfImportDetected is the two-node cycle's smallest cousin.
func TestSelfImportDetected(t *testing.T) {
	sl := New()
	if err := sl.RegisterScriptLibrary("selfie", "import selfie\nx = 1\n"); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, err := sl.Eval("import selfie")
	if err == nil || !strings.Contains(err.Error(), "circular import") {
		t.Fatalf("expected a circular import error, got: %v", err)
	}
}

// TestDeepLinearImportStillLoads proves the chain tracking does not mistake
// a long acyclic chain for a cycle.
func TestDeepLinearImportStillLoads(t *testing.T) {
	sl := New()
	if err := sl.RegisterScriptLibrary("deep_c", "c = 3\n"); err != nil {
		t.Fatal(err)
	}
	if err := sl.RegisterScriptLibrary("deep_b", "import deep_c\nb = deep_c.c + 1\n"); err != nil {
		t.Fatal(err)
	}
	if err := sl.RegisterScriptLibrary("deep_a", "import deep_b\na = deep_b.b + 1\n"); err != nil {
		t.Fatal(err)
	}
	result, err := sl.Eval("import deep_a\ndeep_a.a")
	if err != nil {
		t.Fatalf("linear import chain failed: %v", err)
	}
	if result.Inspect() != "5" {
		t.Fatalf("deep_a.a = %s, want 5", result.Inspect())
	}
}
