package stdlib

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

// TestJSONDumpsIndent: indent accepts a number of spaces (the common idiom)
// and a string used verbatim; 0 newline-separates; keys are always sorted.
func TestJSONDumpsIndent(t *testing.T) {
	dumps := JSONLibrary.Functions()["dumps"]
	ctx := context.Background()

	dump := func(v object.Object, indent object.Object) string {
		kwargs := object.NewKwargs(nil)
		if indent != nil {
			kwargs = object.NewKwargs(map[string]object.Object{"indent": indent})
		}
		result := dumps.Fn(ctx, kwargs, v)
		s, ok := result.(*object.String)
		if !ok {
			t.Fatalf("dumps returned %s: %s", result.Type(), result.Inspect())
		}
		return s.StringValue()
	}

	dict := func(pairs ...any) object.Object {
		m := map[string]object.Object{}
		for i := 0; i < len(pairs); i += 2 {
			m[pairs[i].(string)] = conversion.FromGo(pairs[i+1])
		}
		return object.NewStringDict(m)
	}

	if got := dump(dict("a", []any{1, 2}), object.NewInteger(2)); got != "{\n  \"a\": [\n    1,\n    2\n  ]\n}" {
		t.Errorf("number indent: got %q", got)
	}
	if got := dump(dict("a", []any{1}), object.NewString("\t")); got != "{\n\t\"a\": [\n\t\t1\n\t]\n}" {
		t.Errorf("string indent: got %q", got)
	}
	if got := dump(dict("a", 1), object.NewInteger(0)); got != "{\n\"a\": 1\n}" {
		t.Errorf("zero indent: got %q", got)
	}
	if got := dump(dict("a", []any{1, 2}), nil); got != "{\"a\":[1,2]}" {
		t.Errorf("no indent: got %q", got)
	}
	if got := dump(dict("b", 1, "a", 2), nil); got != "{\"a\":2,\"b\":1}" {
		t.Errorf("keys sorted: got %q", got)
	}

	// Negative indent is an error surfaced as a script error object.
	errResult := dumps.Fn(ctx, object.NewKwargs(map[string]object.Object{"indent": object.NewInteger(-1)}), dict())
	errObj, ok := errResult.(*object.Error)
	if !ok || !strings.Contains(errObj.Message, "indent must not be negative") {
		t.Fatalf("negative indent: got %s (%s)", errResult.Type(), errResult.Inspect())
	}
}
