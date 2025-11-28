package scriptling

import (
	"testing"
)

func TestOutputCapture(t *testing.T) {
	// Test default behavior (should not capture)
	p1 := New()
	_, err := p1.Eval(`print("test")`)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}

	// Should return empty string when capture is not enabled
	output := p1.GetOutput()
	if output != "" {
		t.Errorf("Expected empty output when capture disabled, got: %q", output)
	}

	// Test output capture enabled
	p2 := New()
	p2.EnableOutputCapture()

	_, err = p2.Eval(`
print("Line 1")
print("Line 2")
print("Result:", 42)
`)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}

	captured := p2.GetOutput()
	// Python's print() separates arguments with spaces, not newlines
	expected := "Line 1\nLine 2\nResult: 42\n"
	if captured != expected {
		t.Errorf("Expected %q, got %q", expected, captured)
	}

	// Test that buffer is cleared after GetOutput
	captured2 := p2.GetOutput()
	if captured2 != "" {
		t.Errorf("Expected empty string after second GetOutput, got %q", captured2)
	}

	// Test multiple captures
	_, err = p2.Eval(`print("New output")`)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}

	captured3 := p2.GetOutput()
	expected3 := "New output\n"
	if captured3 != expected3 {
		t.Errorf("Expected %q, got %q", expected3, captured3)
	}
}
