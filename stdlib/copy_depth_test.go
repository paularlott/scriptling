package stdlib

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestDeepcopyDepthCap: nesting deeper than maxDeepcopyDepth must produce a
// catchable error, never a goroutine stack overflow. (Equality and repr
// share the unbounded-recursion exposure for such structures; the copy
// library gets a bound.)
func TestDeepcopyDepthCap(t *testing.T) {
	var deep object.Object = object.NewInteger(0)
	for i := 0; i < maxDeepcopyDepth+5; i++ {
		deep = &object.List{Elements: []object.Object{deep}}
	}

	result := deepCopy(context.Background(), deep, make(map[object.Object]object.Object))
	err, ok := result.(*object.Error)
	if !ok {
		t.Fatalf("expected error past depth cap, got %v", result.Type())
	}
	if !strings.Contains(err.Message, "nesting depth") {
		t.Errorf("error = %q", err.Message)
	}

	// Just inside the cap still copies fine.
	var inner object.Object = object.NewInteger(1)
	for i := 0; i < maxDeepcopyDepth-2; i++ {
		inner = &object.List{Elements: []object.Object{inner}}
	}
	copied := deepCopy(context.Background(), inner, make(map[object.Object]object.Object))
	if copied == inner {
		t.Fatal("deep copy returned the original")
	}
}
