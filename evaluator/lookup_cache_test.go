package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestLookupCacheInvalidatesAfterBaseClassMutation(t *testing.T) {
	input := `
class Counter:
    def value(self):
        return 1

counter = Counter()
first = counter.value()
Counter.value = lambda self: 2
second = counter.value()

[first, second]
`

	evaluated := testEval(input)
	list, ok := evaluated.(*object.List)
	if !ok {
		t.Fatalf("result is not a list. got=%T", evaluated)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("expected 2 results, got %d", len(list.Elements))
	}
	testIntegerObject(t, list.Elements[0], 1)
	testIntegerObject(t, list.Elements[1], 2)
}
