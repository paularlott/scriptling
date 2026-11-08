package mcp

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

func requestToolEntry(extra map[string]object.Object) *object.Dict {
	fields := map[string]object.Object{
		"name":    object.NewString("restart_service"),
		"handler": object.NewString("adminmod.restart"),
	}
	for k, v := range extra {
		fields[k] = v
	}
	return object.NewStringDict(fields)
}

func TestBuildRequestToolProvider_TitleAndHints(t *testing.T) {
	entry := requestToolEntry(map[string]object.Object{
		"title":       object.NewString("Restart Service"),
		"idempotent":  object.NewBoolean(true),
		"destructive": object.NewBoolean(false),
		"read_only":   &object.Null{},
	})
	provider, err := BuildRequestToolProvider([]*object.Dict{entry}, testHandlerConfig())
	if err != nil {
		t.Fatalf("BuildRequestToolProvider: %v", err)
	}
	tool := provider.(*requestToolProvider).tools[0].tool
	if tool.Title != "Restart Service" {
		t.Errorf("Title = %q", tool.Title)
	}
	a := tool.Annotations
	if a == nil {
		t.Fatal("Annotations = nil")
	}
	if a.IdempotentHint == nil || !*a.IdempotentHint || a.DestructiveHint == nil || *a.DestructiveHint {
		t.Errorf("annotations = %+v", a)
	}
	if a.ReadOnlyHint != nil || a.OpenWorldHint != nil {
		t.Errorf("readOnly/openWorld = %v/%v, want unset", a.ReadOnlyHint, a.OpenWorldHint)
	}
}

func TestBuildRequestToolProvider_TitleAndHints_WrongTypeIsError(t *testing.T) {
	for key, val := range map[string]object.Object{
		"title":       object.NewInteger(42),
		"read_only":   object.NewString("yes"),
		"destructive": object.NewInteger(1),
		"idempotent":  object.NewString("true"),
		"open_world":  &object.List{},
	} {
		t.Run(key, func(t *testing.T) {
			entry := requestToolEntry(map[string]object.Object{key: val})
			if _, err := BuildRequestToolProvider([]*object.Dict{entry}, testHandlerConfig()); err == nil {
				t.Fatalf("expected an error for %s=%s", key, val.Inspect())
			}
		})
	}
}
