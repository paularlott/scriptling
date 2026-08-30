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
