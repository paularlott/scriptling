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

// TestJSONDumpsFloats: floats serialize with Python's spellings ("2.0",
// "1e+20"), not Go encoding/json's ("2", full digits).
func TestJSONDumpsFloats(t *testing.T) {
	dumps := JSONLibrary.Functions()["dumps"]
	ctx := context.Background()

	dump := func(v object.Object) string {
		result := dumps.Fn(ctx, object.NewKwargs(nil), v)
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

	want := `{"a":2.0,"b":1e+20,"c":0.1,"d":[1.5,2]}`
	if got := dump(dict("a", 2.0, "b", 1e20, "c", 0.1, "d", []any{1.5, 2})); got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

// TestJSONDecodeErrorIsValueError: bad JSON raises a JSONDecodeError-typed
// error (Python's own classification); the exception hierarchy maps
// JSONDecodeError→ValueError, so except ValueError catches it too.
func TestJSONDecodeErrorIsValueError(t *testing.T) {
	loads := JSONLibrary.Functions()["loads"]
	result := loads.Fn(context.Background(), object.NewKwargs(nil), object.NewString("{bad"))
	err, ok := result.(*object.Error)
	if !ok {
		t.Fatalf("loads returned %s: %s", result.Type(), result.Inspect())
	}
	if err.ExceptionType != "JSONDecodeError" {
		t.Errorf("expected JSONDecodeError classification, got %q", err.ExceptionType)
	}
	// The module exposes the type so `except json.JSONDecodeError:` resolves.
	if JSONLibrary.Constants()["JSONDecodeError"] == nil {
		t.Error("json library does not expose JSONDecodeError")
	}
}

// A cyclic dict must serialize to a placeholder instead of recursing
// forever: unbounded recursion in objectToJSON overflows the Go stack with a
// fatal error that no recover() can catch, aborting the whole host process.
func TestJSONDumpsHandlesCyclicStructures(t *testing.T) {
	dumps := JSONLibrary.Functions()["dumps"]
	ctx := context.Background()

	call := func(v object.Object) string {
		t.Helper()
		result := dumps.Fn(ctx, object.NewKwargs(nil), v)
		s, ok := result.(*object.String)
		if !ok {
			t.Fatalf("dumps returned %s: %s", result.Type(), result.Inspect())
		}
		return s.StringValue()
	}

	t.Run("direct dict self-reference", func(t *testing.T) {
		d := &object.Dict{Pairs: map[string]object.DictPair{}}
		key := object.NewString("self")
		d.Pairs[object.DictKey(key)] = object.DictPair{Key: key, Value: d}
		got := call(d)
		want := "{\"self\":\"<cyclic reference>\"}"
		if got != want {
			t.Fatalf("dumps() = %q, want %q", got, want)
		}
	})

	t.Run("indirect dict-list-dict cycle", func(t *testing.T) {
		d := &object.Dict{Pairs: map[string]object.DictPair{}}
		l := &object.List{Elements: []object.Object{d}}
		key := object.NewString("k")
		d.Pairs[object.DictKey(key)] = object.DictPair{Key: key, Value: l}
		got := call(d)
		want := "{\"k\":[\"<cyclic reference>\"]}"
		if got != want {
			t.Fatalf("dumps() = %q, want %q", got, want)
		}
	})

	t.Run("shared but acyclic subtree still fully renders", func(t *testing.T) {
		shared := &object.List{Elements: []object.Object{object.NewInteger(1)}}
		outer := &object.List{Elements: []object.Object{shared, shared}}
		got := call(outer)
		want := `[[1],[1]]`
		if got != want {
			t.Fatalf("dumps() = %q, want %q (a DAG is not a cycle)", got, want)
		}
	})
}
