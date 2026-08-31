package plugin

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestServerRegisterLibraryIngestsLibrary proves the shared-registration
// contract: a single *object.Library (as built for compiled-in plugins) feeds
// an external plugin server unchanged — functions, classes as constants, and
// plain constants all survive.
func TestServerRegisterLibraryIngestsLibrary(t *testing.T) {
	builder := object.NewLibraryBuilder("mixed", "mixed library")
	builder.Function("shout", func(s string) string { return s + "!" })
	builder.Constant("meaning", 42)

	classBuilder := object.NewClassBuilder("Box").
		Method("__init__", func(self *object.Instance, value int) {
			self.SetField("value", object.NewInteger(int64(value)))
		}).
		Method("get", func(self *object.Instance) int {
			return int(self.Field("value").(*object.Integer).IntValue())
		})
	builder.Constant("Box", classBuilder.Build())

	server := NewServer("mixed", "1.0.0", "mixed library test")
	server.RegisterLibrary(builder.Build())

	client := policyPipeServer(t, server, nil)
	defer client.Close()

	result, err := client.CallFunction(context.Background(), "shout", []Value{{Type: valueString, Value: "hi"}}, nil)
	if err != nil {
		t.Fatalf("CallFunction: %v", err)
	}
	if result.Type != valueString || result.Value != "hi!" {
		t.Fatalf("function via RegisterLibrary wrong: %#v", result)
	}

	ref, err := client.NewObject(context.Background(), "Box", []Value{{Type: valueInt, Value: int64(7)}}, nil)
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	got, err := client.CallMethod(context.Background(), ref.ID, "get", nil, nil)
	if err != nil {
		t.Fatalf("CallMethod: %v", err)
	}
	if got.Type != valueInt || numberToInt64(got.Value) != 7 {
		t.Fatalf("class method via RegisterLibrary wrong: %#v", got)
	}

	metadata := client.Metadata()
	foundFn, foundClass, foundConst := false, false, false
	for _, fn := range metadata.Schema.Functions {
		if fn.Name == "shout" {
			foundFn = true
		}
	}
	for _, cls := range metadata.Schema.Classes {
		if cls.Name == "Box" {
			foundClass = true
		}
	}
	for _, c := range metadata.Schema.Constants {
		if c.Name == "meaning" {
			foundConst = true
		}
	}
	if !foundFn || !foundClass || !foundConst {
		t.Fatalf("schema incomplete after RegisterLibrary: fn=%v class=%v const=%v", foundFn, foundClass, foundConst)
	}
}
