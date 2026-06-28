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
	platformStr := result.(*object.String).StringValue()
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
	result = PlatformLibrary.Functions()["system"].Fn(ctx, object.NewKwargs(nil), object.NewInteger(1))
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
	uuidStr := result.(*object.String).StringValue()
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
	uuidStr = result.(*object.String).StringValue()
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
	uuidStr = result.(*object.String).StringValue()
	if !uuidRegex.MatchString(uuidStr) {
		t.Errorf("uuid7() returned invalid UUID: %q", uuidStr)
	}
	if uuidStr[14] != '7' {
		t.Errorf("uuid7() should have version 7, got %q", uuidStr)
	}

	// Test with args (should error)
	result = UUIDLibrary.Functions()["uuid4"].Fn(ctx, object.NewKwargs(nil), object.NewInteger(1))
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("uuid4() with arg should return error, got %v", result.Type())
	}
}

func TestHashlibLibrary(t *testing.T) {
	ctx := context.Background()
	hexRegex := regexp.MustCompile(`^[0-9a-f]+$`)

	// hexdigest calls the .hexdigest() method on a Hash instance result.
	hexdigest := func(t *testing.T, result object.Object) string {
		t.Helper()
		inst, ok := result.(*object.Instance)
		if !ok {
			t.Fatalf("expected Hash instance, got %v", result.Type())
		}
		fn, ok := inst.Class.LookupMember("hexdigest")
		if !ok {
			t.Fatalf("Hash instance has no hexdigest method")
		}
		res := fn.(*object.Builtin).Fn(ctx, object.NewKwargs(nil), inst)
		str, ok := res.(*object.String)
		if !ok {
			t.Fatalf("hexdigest() returned %v, want string", res.Type())
		}
		return str.StringValue()
	}

	// Test sha256
	result := HashlibLibrary.Functions()["sha256"].Fn(ctx, object.NewKwargs(nil), object.NewString("hello"))
	if result.Type() != object.INSTANCE_OBJ {
		t.Errorf("sha256() returned %v, want instance", result.Type())
	}
	hashStr := hexdigest(t, result)
	if !hexRegex.MatchString(hashStr) {
		t.Errorf("sha256() returned invalid hex: %q", hashStr)
	}
	if len(hashStr) != 64 {
		t.Errorf("sha256() hash length = %d, want 64", len(hashStr))
	}

	// Test sha1
	result = HashlibLibrary.Functions()["sha1"].Fn(ctx, object.NewKwargs(nil), object.NewString("hello"))
	if result.Type() != object.INSTANCE_OBJ {
		t.Errorf("sha1() returned %v, want instance", result.Type())
	}
	hashStr = hexdigest(t, result)
	if !hexRegex.MatchString(hashStr) {
		t.Errorf("sha1() returned invalid hex: %q", hashStr)
	}
	if len(hashStr) != 40 {
		t.Errorf("sha1() hash length = %d, want 40", len(hashStr))
	}

	// Test md5
	result = HashlibLibrary.Functions()["md5"].Fn(ctx, object.NewKwargs(nil), object.NewString("hello"))
	if result.Type() != object.INSTANCE_OBJ {
		t.Errorf("md5() returned %v, want instance", result.Type())
	}
	hashStr = hexdigest(t, result)
	if !hexRegex.MatchString(hashStr) {
		t.Errorf("md5() returned invalid hex: %q", hashStr)
	}
	if len(hashStr) != 32 {
		t.Errorf("md5() hash length = %d, want 32", len(hashStr))
	}

	// Test known values
	result = HashlibLibrary.Functions()["sha256"].Fn(ctx, object.NewKwargs(nil), object.NewString(""))
	expectedSHA256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hexdigest(t, result) != expectedSHA256 {
		t.Errorf("sha256('') = %q, want %q", hexdigest(t, result), expectedSHA256)
	}

	result = HashlibLibrary.Functions()["md5"].Fn(ctx, object.NewKwargs(nil), object.NewString(""))
	expectedMD5 := "d41d8cd98f00b204e9800998ecf8427e"
	if hexdigest(t, result) != expectedMD5 {
		t.Errorf("md5('') = %q, want %q", hexdigest(t, result), expectedMD5)
	}

	// Constructors with no argument return an empty hash object (no error).
	result = HashlibLibrary.Functions()["sha256"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.INSTANCE_OBJ {
		t.Errorf("sha256() without args should return empty hash, got %v", result.Type())
	}

	// Test instance attributes
	inst := result.(*object.Instance)
	if name := inst.Field("name").(*object.String).StringValue(); name != "sha256" {
		t.Errorf("sha256().name = %q, want sha256", name)
	}
	if inst.Field("digest_size").(*object.Integer).IntValue() != 32 {
		t.Errorf("sha256().digest_size = %d, want 32", inst.Field("digest_size").(*object.Integer).IntValue())
	}
	if inst.Field("block_size").(*object.Integer).IntValue() != 64 {
		t.Errorf("sha256().block_size = %d, want 64", inst.Field("block_size").(*object.Integer).IntValue())
	}

	// Test with invalid args (should error)
	result = HashlibLibrary.Functions()["sha256"].Fn(ctx, object.NewKwargs(nil), object.NewInteger(42))
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("sha256() with int should return error, got %v", result.Type())
	}
}

func TestHmacLibrary(t *testing.T) {
	ctx := context.Background()

	// Known SHA-256 HMAC: key="key", msg="The quick brown fox jumps over the lazy dog"
	known := "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"

	hexdigest := func(result object.Object) string {
		inst := result.(*object.Instance)
		fn, _ := inst.Class.LookupMember("hexdigest")
		return fn.(*object.Builtin).Fn(ctx, object.NewKwargs(nil), inst).(*object.String).StringValue()
	}

	// hmac.new with string digestmod
	result := HmacLibrary.Functions()["new"].Fn(ctx, object.NewKwargs(nil),
		object.NewString("key"), object.NewString("The quick brown fox jumps over the lazy dog"), object.NewString("sha256"))
	if hexdigest(result) != known {
		t.Errorf("hmac.new sha256 = %q, want %q", hexdigest(result), known)
	}

	// hmac.new passing the hashlib.sha256 constructor reference as digestmod
	result2 := HmacLibrary.Functions()["new"].Fn(ctx, object.NewKwargs(nil),
		object.NewString("key"), object.NewString("msg"), HashlibSHA256Builtin)
	if hexdigest(result2) != hexdigest(HmacLibrary.Functions()["new"].Fn(ctx, object.NewKwargs(nil),
		object.NewString("key"), object.NewString("msg"), object.NewString("sha256"))) {
		t.Errorf("hashlib.sha256 as digestmod should match \"sha256\"")
	}

	// Default digestmod (omitted) is sha256
	result3 := HmacLibrary.Functions()["new"].Fn(ctx, object.NewKwargs(nil),
		object.NewString("key"), object.NewString("msg"))
	if hexdigest(result3) != hexdigest(HmacLibrary.Functions()["new"].Fn(ctx, object.NewKwargs(nil),
		object.NewString("key"), object.NewString("msg"), object.NewString("sha256"))) {
		t.Errorf("omitted digestmod should default to sha256")
	}

	// compare_digest
	r := HmacLibrary.Functions()["compare_digest"].Fn(ctx, object.NewKwargs(nil),
		object.NewString("abc"), object.NewString("abc"))
	if !r.(*object.Boolean).BoolValue() {
		t.Errorf("compare_digest('abc','abc') should be true")
	}
	r = HmacLibrary.Functions()["compare_digest"].Fn(ctx, object.NewKwargs(nil),
		object.NewString("abc"), object.NewString("abd"))
	if r.(*object.Boolean).BoolValue() {
		t.Errorf("compare_digest('abc','abd') should be false")
	}

	// compare_digest requires strings
	r = HmacLibrary.Functions()["compare_digest"].Fn(ctx, object.NewKwargs(nil),
		object.NewInteger(1), object.NewInteger(1))
	if r.Type() != object.ERROR_OBJ {
		t.Errorf("compare_digest with ints should error, got %v", r.Type())
	}

	// Unsupported digestmod errors
	r = HmacLibrary.Functions()["new"].Fn(ctx, object.NewKwargs(nil),
		object.NewString("k"), object.NewString("m"), object.NewString("nonsense"))
	if r.Type() != object.ERROR_OBJ {
		t.Errorf("hmac.new with unknown algorithm should error, got %v", r.Type())
	}
}
