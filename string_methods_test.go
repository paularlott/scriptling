package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestStringSplitMethod(t *testing.T) {
	p := New()

	t.Run("split with no arguments - splits on whitespace", func(t *testing.T) {
		result, err := p.Eval(`"hello  world  how are you".split()`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"hello", "world", "how", "are", "you"}
		list, ok := result.(*object.List)
		if !ok {
			t.Fatalf("expected List, got %T", result)
		}
		if len(list.Elements) != len(expected) {
			t.Errorf("expected %d elements, got %d", len(expected), len(list.Elements))
		}
		for i, exp := range expected {
			if list.Elements[i].Inspect() != exp {
				t.Errorf("element %d: expected %s, got %s", i, exp, list.Elements[i].Inspect())
			}
		}
	})

	t.Run("split with separator - splits all occurrences", func(t *testing.T) {
		result, err := p.Eval(`"a:b:c".split(":")`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"a", "b", "c"}
		list, ok := result.(*object.List)
		if !ok {
			t.Fatalf("expected List, got %T", result)
		}
		if len(list.Elements) != len(expected) {
			t.Errorf("expected %d elements, got %d", len(expected), len(list.Elements))
		}
		for i, exp := range expected {
			if list.Elements[i].Inspect() != exp {
				t.Errorf("element %d: expected %s, got %s", i, exp, list.Elements[i].Inspect())
			}
		}
	})

	t.Run("split with separator and maxsplit=0 - no split", func(t *testing.T) {
		result, err := p.Eval(`"a:b:c".split(":", 0)`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"a:b:c"}
		list, ok := result.(*object.List)
		if !ok {
			t.Fatalf("expected List, got %T", result)
		}
		if len(list.Elements) != len(expected) {
			t.Errorf("expected %d elements, got %d", len(expected), len(list.Elements))
		}
		for i, exp := range expected {
			if list.Elements[i].Inspect() != exp {
				t.Errorf("element %d: expected %s, got %s", i, exp, list.Elements[i].Inspect())
			}
		}
	})

	t.Run("split with separator and maxsplit=1 - one split", func(t *testing.T) {
		result, err := p.Eval(`"a:b:c".split(":", 1)`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"a", "b:c"}
		list, ok := result.(*object.List)
		if !ok {
			t.Fatalf("expected List, got %T", result)
		}
		if len(list.Elements) != len(expected) {
			t.Errorf("expected %d elements, got %d", len(expected), len(list.Elements))
		}
		for i, exp := range expected {
			if list.Elements[i].Inspect() != exp {
				t.Errorf("element %d: expected %s, got %s", i, exp, list.Elements[i].Inspect())
			}
		}
	})

	t.Run("split with separator and maxsplit=2 - two splits", func(t *testing.T) {
		result, err := p.Eval(`"a:b:c:d".split(":", 2)`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"a", "b", "c:d"}
		list, ok := result.(*object.List)
		if !ok {
			t.Fatalf("expected List, got %T", result)
		}
		if len(list.Elements) != len(expected) {
			t.Errorf("expected %d elements, got %d", len(expected), len(list.Elements))
		}
		for i, exp := range expected {
			if list.Elements[i].Inspect() != exp {
				t.Errorf("element %d: expected %s, got %s", i, exp, list.Elements[i].Inspect())
			}
		}
	})

	t.Run("split with separator and maxsplit greater than actual splits", func(t *testing.T) {
		result, err := p.Eval(`"a:b:c".split(":", 10)`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"a", "b", "c"}
		list, ok := result.(*object.List)
		if !ok {
			t.Fatalf("expected List, got %T", result)
		}
		if len(list.Elements) != len(expected) {
			t.Errorf("expected %d elements, got %d", len(expected), len(list.Elements))
		}
		for i, exp := range expected {
			if list.Elements[i].Inspect() != exp {
				t.Errorf("element %d: expected %s, got %s", i, exp, list.Elements[i].Inspect())
			}
		}
	})

	t.Run("split with negative maxsplit - unlimited splits", func(t *testing.T) {
		result, err := p.Eval(`"a:b:c".split(":", -1)`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"a", "b", "c"}
		list, ok := result.(*object.List)
		if !ok {
			t.Fatalf("expected List, got %T", result)
		}
		if len(list.Elements) != len(expected) {
			t.Errorf("expected %d elements, got %d", len(expected), len(list.Elements))
		}
		for i, exp := range expected {
			if list.Elements[i].Inspect() != exp {
				t.Errorf("element %d: expected %s, got %s", i, exp, list.Elements[i].Inspect())
			}
		}
	})

	t.Run("original use case - tool_name.split(\":\", 1)", func(t *testing.T) {
		_, err := p.Eval(`
tool_name = "mcp__4_5v_mcp__analyze_image"
parts = tool_name.split(":", 1)
`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		elements, errObj := p.GetVarAsList("parts")
		if errObj != nil {
			t.Fatalf("failed to get parts variable: %v", errObj)
		}
		expected := []string{"mcp__4_5v_mcp__analyze_image"}
		if len(elements) != len(expected) {
			t.Errorf("expected %d elements, got %d", len(expected), len(elements))
		}
		for i, exp := range expected {
			if elements[i].Inspect() != exp {
				t.Errorf("element %d: expected %s, got %s", i, exp, elements[i].Inspect())
			}
		}
	})

	t.Run("split on colon with maxsplit=1 - real case", func(t *testing.T) {
		result, err := p.Eval(`"tool:namespace".split(":", 1)`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"tool", "namespace"}
		list, ok := result.(*object.List)
		if !ok {
			t.Fatalf("expected List, got %T", result)
		}
		if len(list.Elements) != len(expected) {
			t.Errorf("expected %d elements, got %d", len(expected), len(list.Elements))
		}
		for i, exp := range expected {
			if list.Elements[i].Inspect() != exp {
				t.Errorf("element %d: expected %s, got %s", i, exp, list.Elements[i].Inspect())
			}
		}
	})

	t.Run("split string that doesn't contain separator", func(t *testing.T) {
		result, err := p.Eval(`"abc".split(":")`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"abc"}
		list, ok := result.(*object.List)
		if !ok {
			t.Fatalf("expected List, got %T", result)
		}
		if len(list.Elements) != len(expected) {
			t.Errorf("expected %d elements, got %d", len(expected), len(list.Elements))
		}
		for i, exp := range expected {
			if list.Elements[i].Inspect() != exp {
				t.Errorf("element %d: expected %s, got %s", i, exp, list.Elements[i].Inspect())
			}
		}
	})
}
