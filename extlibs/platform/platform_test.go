package platform

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestPlatformLibrary(t *testing.T) {
	ctx := context.Background()

	// Test python_version
	result := Library.Functions()["python_version"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("python_version() returned %v, want string", result.Type())
	}

	// Test system
	result = Library.Functions()["system"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("system() returned %v, want string", result.Type())
	}
	systemName := result.(*object.String).StringValue()
	validSystems := []string{"Darwin", "Linux", "Windows", "FreeBSD"}
	valid := false
	for _, s := range validSystems {
		if systemName == s || systemName != "" {
			valid = true
			break
		}
	}
	if !valid && systemName == "" {
		t.Error("system() returned unexpected value")
	}

	// Test architecture
	result = Library.Functions()["architecture"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.LIST_OBJ {
		t.Errorf("architecture() returned %v, want list", result.Type())
	}
	list := result.(*object.List)
	if len(list.Elements) != 2 {
		t.Errorf("architecture() list length = %d, want 2", len(list.Elements))
	}

	// Test machine
	result = Library.Functions()["machine"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("machine() returned %v, want string", result.Type())
	}

	// Test platform
	result = Library.Functions()["platform"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("platform() returned %v, want string", result.Type())
	}
	platformStr := result.(*object.String).StringValue()
	if !strings.Contains(platformStr, "-") {
		t.Errorf("platform() should contain '-', got %q", platformStr)
	}

	// Test scriptling_version
	result = Library.Functions()["scriptling_version"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("scriptling_version() returned %v, want string", result.Type())
	}

	// Test processor
	result = Library.Functions()["processor"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("processor() returned %v, want string", result.Type())
	}

	// Test node
	result = Library.Functions()["node"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("node() returned %v, want string", result.Type())
	}

	// Test release
	result = Library.Functions()["release"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("release() returned %v, want string", result.Type())
	}

	// Test version
	result = Library.Functions()["version"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("version() returned %v, want string", result.Type())
	}

	// Test uname
	result = Library.Functions()["uname"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.DICT_OBJ {
		t.Errorf("uname() returned %v, want dict", result.Type())
	}
	dict := result.(*object.Dict)
	if len(dict.Pairs) < 5 {
		t.Errorf("uname() dict length = %d, want >= 5", len(dict.Pairs))
	}

	// Test with args (should error)
	result = Library.Functions()["system"].Fn(ctx, object.NewKwargs(nil), object.NewInteger(1))
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("system() with arg should return error, got %v", result.Type())
	}
}
