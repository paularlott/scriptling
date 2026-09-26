package stdlib

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestNamedGroups: (?P<name>...) captures are reachable by name and through
// groupdict(), matching Python.
func TestNamedGroups(t *testing.T) {
	search := ReLibrary.Functions()["search"]
	if search == nil {
		t.Fatal("re.search not registered")
	}
	ctx := context.Background()
	result := search.Fn(ctx, object.NewKwargs(nil), object.NewString(`(?P<year>\d{4})-(?P<mon>\d{2})`), object.NewString("2026-09-27"))
	m, ok := result.(*object.Instance)
	if !ok {
		t.Fatalf("search returned %s: %s", result.Type(), result.Inspect())
	}
	group := MatchClass.Methods["group"].(*object.Builtin).Fn
	groupdict := MatchClass.Methods["groupdict"].(*object.Builtin).Fn

	if got := group(ctx, object.NewKwargs(nil), m, object.NewString("year")); got.Inspect() != "2026" {
		t.Errorf("group('year'): got %s", got.Inspect())
	}
	if got := group(ctx, object.NewKwargs(nil), m, object.NewInteger(1)); got.Inspect() != "2026" {
		t.Errorf("group(1): got %s", got.Inspect())
	}
	dict, ok := groupdict(ctx, object.NewKwargs(nil), m).(*object.Dict)
	if !ok {
		t.Fatalf("groupdict returned non-dict")
	}
	if y, found := dict.GetByString("year"); !found || y.Value.Inspect() != "2026" {
		t.Errorf("groupdict year: %+v", dict.Inspect())
	}
	if mo, found := dict.GetByString("mon"); !found || mo.Value.Inspect() != "09" {
		t.Errorf("groupdict mon: %+v", dict.Inspect())
	}

	// Unknown names raise a catchable IndexError like Python.
	errResult := group(ctx, object.NewKwargs(nil), m, object.NewString("nope"))
	exc, ok := errResult.(*object.Exception)
	if !ok || exc.ExceptionType != "IndexError" {
		t.Errorf("unknown group should raise IndexError, got %s", errResult.Inspect())
	}
}
