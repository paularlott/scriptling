package stdlib

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestPlatformLibrary(t *testing.T) {
	ctx := context.Background()

	// Test python_version
	result := PlatformLibrary.Functions()["python_version"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("python_version() returned %v, want string", result.Type())
	}

	// Test system
	result = PlatformLibrary.Functions()["system"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("system() returned %v, want string", result.Type())
	}
	systemName := result.(*object.String).Value
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
	result = PlatformLibrary.Functions()["architecture"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.LIST_OBJ {
		t.Errorf("architecture() returned %v, want list", result.Type())
	}
	list := result.(*object.List)
	if len(list.Elements) != 2 {
		t.Errorf("architecture() list length = %d, want 2", len(list.Elements))
	}

	// Test machine
	result = PlatformLibrary.Functions()["machine"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("machine() returned %v, want string", result.Type())
	}

	// Test platform
	result = PlatformLibrary.Functions()["platform"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("platform() returned %v, want string", result.Type())
	}
	platformStr := result.(*object.String).Value
	if !strings.Contains(platformStr, "-") {
		t.Errorf("platform() should contain '-', got %q", platformStr)
	}

	// Test scriptling_version
	result = PlatformLibrary.Functions()["scriptling_version"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("scriptling_version() returned %v, want string", result.Type())
	}

	// Test processor
	result = PlatformLibrary.Functions()["processor"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("processor() returned %v, want string", result.Type())
	}

	// Test node
	result = PlatformLibrary.Functions()["node"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("node() returned %v, want string", result.Type())
	}

	// Test release
	result = PlatformLibrary.Functions()["release"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("release() returned %v, want string", result.Type())
	}

	// Test version
	result = PlatformLibrary.Functions()["version"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("version() returned %v, want string", result.Type())
	}

	// Test uname
	result = PlatformLibrary.Functions()["uname"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.DICT_OBJ {
		t.Errorf("uname() returned %v, want dict", result.Type())
	}
	dict := result.(*object.Dict)
	if len(dict.Pairs) < 5 {
		t.Errorf("uname() dict length = %d, want >= 5", len(dict.Pairs))
	}

	// Test with args (should error)
	result = PlatformLibrary.Functions()["system"].Fn(ctx, object.NewKwargs(nil), &object.Integer{Value: 1})
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("system() with arg should return error, got %v", result.Type())
	}
}

func TestUUIDLibrary(t *testing.T) {
	ctx := context.Background()
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	// Test uuid4
	result := UUIDLibrary.Functions()["uuid4"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("uuid4() returned %v, want string", result.Type())
	}
	uuidStr := result.(*object.String).Value
	if !uuidRegex.MatchString(uuidStr) {
		t.Errorf("uuid4() returned invalid UUID: %q", uuidStr)
	}
	if uuidStr[14] != '4' {
		t.Errorf("uuid4() should have version 4, got %q", uuidStr)
	}

	// Test uuid1
	result = UUIDLibrary.Functions()["uuid1"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("uuid1() returned %v, want string", result.Type())
	}
	uuidStr = result.(*object.String).Value
	if !uuidRegex.MatchString(uuidStr) {
		t.Errorf("uuid1() returned invalid UUID: %q", uuidStr)
	}
	if uuidStr[14] != '1' {
		t.Errorf("uuid1() should have version 1, got %q", uuidStr)
	}

	// Test uuid7
	result = UUIDLibrary.Functions()["uuid7"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("uuid7() returned %v, want string", result.Type())
	}
	uuidStr = result.(*object.String).Value
	if !uuidRegex.MatchString(uuidStr) {
		t.Errorf("uuid7() returned invalid UUID: %q", uuidStr)
	}
	if uuidStr[14] != '7' {
		t.Errorf("uuid7() should have version 7, got %q", uuidStr)
	}

	// Test with args (should error)
	result = UUIDLibrary.Functions()["uuid4"].Fn(ctx, object.NewKwargs(nil), &object.Integer{Value: 1})
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("uuid4() with arg should return error, got %v", result.Type())
	}
}

func TestHashlibLibrary(t *testing.T) {
	ctx := context.Background()
	hexRegex := regexp.MustCompile(`^[0-9a-f]+$`)

	// Test sha256
	result := HashlibLibrary.Functions()["sha256"].Fn(ctx, object.NewKwargs(nil), &object.String{Value: "hello"})
	if result.Type() != object.STRING_OBJ {
		t.Errorf("sha256() returned %v, want string", result.Type())
	}
	hashStr := result.(*object.String).Value
	if !hexRegex.MatchString(hashStr) {
		t.Errorf("sha256() returned invalid hex: %q", hashStr)
	}
	if len(hashStr) != 64 {
		t.Errorf("sha256() hash length = %d, want 64", len(hashStr))
	}

	// Test sha1
	result = HashlibLibrary.Functions()["sha1"].Fn(ctx, object.NewKwargs(nil), &object.String{Value: "hello"})
	if result.Type() != object.STRING_OBJ {
		t.Errorf("sha1() returned %v, want string", result.Type())
	}
	hashStr = result.(*object.String).Value
	if !hexRegex.MatchString(hashStr) {
		t.Errorf("sha1() returned invalid hex: %q", hashStr)
	}
	if len(hashStr) != 40 {
		t.Errorf("sha1() hash length = %d, want 40", len(hashStr))
	}

	// Test md5
	result = HashlibLibrary.Functions()["md5"].Fn(ctx, object.NewKwargs(nil), &object.String{Value: "hello"})
	if result.Type() != object.STRING_OBJ {
		t.Errorf("md5() returned %v, want string", result.Type())
	}
	hashStr = result.(*object.String).Value
	if !hexRegex.MatchString(hashStr) {
		t.Errorf("md5() returned invalid hex: %q", hashStr)
	}
	if len(hashStr) != 32 {
		t.Errorf("md5() hash length = %d, want 32", len(hashStr))
	}

	// Test known values
	result = HashlibLibrary.Functions()["sha256"].Fn(ctx, object.NewKwargs(nil), &object.String{Value: ""})
	expectedSHA256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if result.(*object.String).Value != expectedSHA256 {
		t.Errorf("sha256('') = %q, want %q", result.(*object.String).Value, expectedSHA256)
	}

	result = HashlibLibrary.Functions()["md5"].Fn(ctx, object.NewKwargs(nil), &object.String{Value: ""})
	expectedMD5 := "d41d8cd98f00b204e9800998ecf8427e"
	if result.(*object.String).Value != expectedMD5 {
		t.Errorf("md5('') = %q, want %q", result.(*object.String).Value, expectedMD5)
	}

	// Test with invalid args (should error)
	result = HashlibLibrary.Functions()["sha256"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("sha256() without args should return error, got %v", result.Type())
	}

	result = HashlibLibrary.Functions()["sha256"].Fn(ctx, object.NewKwargs(nil), &object.Integer{Value: 42})
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("sha256() with int should return error, got %v", result.Type())
	}
}
