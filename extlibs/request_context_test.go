package extlibs

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestRequestContextCopyIsDeep pins the concurrency contract of
// request_context(): a JSON-RPC batch dispatches its elements concurrently,
// and each handler's copy must be deep enough that writing into a nested
// dict or list is local, never a write through to the request's own (or
// another handler's) value.
func TestRequestContextCopyIsDeep(t *testing.T) {
	user := object.NewStringDict(map[string]object.Object{"name": object.NewString("ada")})
	tags := &object.List{Elements: []object.Object{object.NewString("blue")}}
	root := object.NewStringDict(map[string]object.Object{"user": user, "tags": tags})
	req := object.NewInstanceWithFields(nil, map[string]object.Object{"context": root})
	ctx := WithRequestContext(context.Background(), req)

	call := func() *object.Dict {
		builtin := RequestContextBuiltins()["request_context"]
		dict, ok := builtin.Fn(ctx, object.NewKwargs(nil)).(*object.Dict)
		if !ok {
			t.Fatal("request_context did not return a dict")
		}
		return dict
	}
	first := call()
	second := call()

	firstUserPair, ok := first.GetByString("user")
	if !ok {
		t.Fatal("context user missing")
	}
	firstUser, ok := firstUserPair.Value.(*object.Dict)
	if !ok {
		t.Fatal("context user is not a dict")
	}
	firstUser.SetByString("name", object.NewString("mutated"))
	secondUserPair, ok := second.GetByString("user")
	if !ok {
		t.Fatal("second context user missing")
	}
	secondUser, ok := secondUserPair.Value.(*object.Dict)
	if !ok {
		t.Fatal("second context user is not a dict")
	}
	if got := dictString(t, secondUser, "name"); got != "ada" {
		t.Fatalf("nested dict write escaped the copy: name = %s", got)
	}
	if got := dictString(t, user, "name"); got != "ada" {
		t.Fatalf("nested dict write escaped to the request itself: name = %s", got)
	}

	firstTagsPair, ok := first.GetByString("tags")
	if !ok {
		t.Fatal("context tags missing")
	}
	firstTags, ok := firstTagsPair.Value.(*object.List)
	if !ok {
		t.Fatal("context tags is not a list")
	}
	firstTags.Elements[0] = object.NewString("mutated")
	secondTagsPair, ok := second.GetByString("tags")
	if !ok {
		t.Fatal("second context tags missing")
	}
	secondTags, ok := secondTagsPair.Value.(*object.List)
	if !ok {
		t.Fatal("second context tags is not a list")
	}
	if got := secondTags.Elements[0].Inspect(); got != "blue" {
		t.Fatalf("list write escaped the copy: tags[0] = %s", got)
	}
}

func dictString(t *testing.T, dict *object.Dict, key string) string {
	t.Helper()
	pair, ok := dict.GetByString(key)
	if !ok {
		t.Fatalf("key %s missing", key)
	}
	value, err := pair.Value.AsString()
	if err != nil {
		t.Fatalf("key %s is not a string: %v", key, err)
	}
	return value
}

// TestRequestContextCopyDoesNotIsolateSetOrBytes characterises the KNOWN edge
// of request_context()'s deep copy: copyValue recurses through dicts and lists
// but passes every other object (including the mutable Set and Bytes types)
// through by reference. So if a middleware stores a Set or Bytes in
// request.context and a handler mutates it, the mutation is visible to the
// other concurrent handlers of a JSON-RPC batch.
//
// This is not a launch blocker — the practical trigger (middleware stashing a
// mutable set/bytes in context AND a handler mutating it AND concurrent batch
// dispatch) is narrow, and the documented guidance is to keep request context
// to plain data. The test exists so the boundary is explicit: if copyValue is
// later extended to copy sets/bytes, this test should be updated to assert
// isolation instead.
func TestRequestContextCopyDoesNotIsolateSetOrBytes(t *testing.T) {
	shared := object.NewSet()
	shared.Elements["blue"] = object.NewString("blue")
	root := object.NewStringDict(map[string]object.Object{"labels": shared})
	req := object.NewInstanceWithFields(nil, map[string]object.Object{"context": root})
	ctx := WithRequestContext(context.Background(), req)

	call := func() *object.Dict {
		builtin := RequestContextBuiltins()["request_context"]
		dict, ok := builtin.Fn(ctx, object.NewKwargs(nil)).(*object.Dict)
		if !ok {
			t.Fatal("request_context did not return a dict")
		}
		return dict
	}

	first := call()
	second := call()

	firstPair, ok := first.GetByString("labels")
	if !ok {
		t.Fatal("labels missing from first copy")
	}
	firstSet, ok := firstPair.Value.(*object.Set)
	if !ok {
		t.Fatalf("labels is not a set: %T", firstPair.Value)
	}
	// Mutate the set obtained from the first copy.
	firstSet.Elements["green"] = object.NewString("green")

	secondPair, _ := second.GetByString("labels")
	secondSet := secondPair.Value.(*object.Set)
	// Current behaviour: the two copies share the same *Set, so the mutation
	// is visible. Documenting this explicitly.
	if _, leaked := secondSet.Elements["green"]; !leaked {
		t.Fatal("set is now isolated between copies — copyValue was extended; " +
			"update this test to assert isolation (and drop it from the known-edge list)")
	}
}
