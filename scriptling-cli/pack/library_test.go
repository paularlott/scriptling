package pack

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestPackageLibraryReadBytes verifies package.read_bytes returns raw Bytes
// unchanged — critical for binary assets like msgpack/protobuf payloads that
// would corrupt if forced through a string.
func TestPackageLibraryReadBytes(t *testing.T) {
	// Bundle with a binary file containing bytes that aren't valid UTF-8.
	binary := []byte{0xFF, 0x00, 0x80, 0x01, 0xFE, 0x68, 0x69}
	files := map[string]string{"data/blob.bin": string(binary)}
	b := bundleFromMap(t, "name=\"app\"\nversion=\"1\"\n", files)

	l := NewLoader()
	if err := l.AddBundle(b); err != nil {
		t.Fatalf("AddBundle: %v", err)
	}

	lib := NewPackageLibrary(l)
	readBytes := lib.Functions()["read_bytes"].Fn

	got := readBytes(context.Background(), object.NewKwargs(nil),
		object.NewString("app"), object.NewString("data/blob.bin"))
	bObj, ok := got.(*object.Bytes)
	if !ok {
		t.Fatalf("read_bytes returned %T, want *Bytes (value=%v)", got, got)
	}
	if &bObj == nil || len(bObj.BytesValue()) != len(binary) {
		t.Fatalf("read_bytes returned %d bytes, want %d", bObj.Len(), len(binary))
	}
	for i, want := range binary {
		if bObj.BytesValue()[i] != want {
			t.Errorf("byte %d: got %x, want %x", i, bObj.BytesValue()[i], want)
		}
	}

	// Compare against read_file (String) — both should hold the same bytes,
	// just exposed under different types.
	readFile := lib.Functions()["read_file"].Fn
	strRes := readFile(context.Background(), object.NewKwargs(nil),
		object.NewString("app"), object.NewString("data/blob.bin"))
	s, ok := strRes.(*object.String)
	if !ok {
		t.Fatalf("read_file returned %T, want *String", strRes)
	}
	if s.StringValue() != string(binary) {
		t.Errorf("read_file content mismatch (byte-level)")
	}
}

// TestPackageLibraryReadBytesErrors verifies unhappy paths surface cleanly
// without panicking.
func TestPackageLibraryReadBytesErrors(t *testing.T) {
	b := bundleFromMap(t, "name=\"app\"\nversion=\"1\"\n", map[string]string{"x.txt": "hi"})
	l := NewLoader()
	_ = l.AddBundle(b)
	lib := NewPackageLibrary(l)
	readBytes := lib.Functions()["read_bytes"].Fn

	// Unknown package
	res := readBytes(context.Background(), object.NewKwargs(nil),
		object.NewString("nope"), object.NewString("x.txt"))
	if _, ok := res.(*object.Error); !ok {
		t.Fatalf("unknown package: expected *Error, got %T (%v)", res, res)
	}

	// Missing file in known package
	res = readBytes(context.Background(), object.NewKwargs(nil),
		object.NewString("app"), object.NewString("missing.bin"))
	if _, ok := res.(*object.Error); !ok {
		t.Fatalf("missing file: expected *Error, got %T (%v)", res, res)
	}

	// Wrong arg types
	res = readBytes(context.Background(), object.NewKwargs(nil),
		object.NewInteger(42), object.NewString("x.txt"))
	if _, ok := res.(*object.Error); !ok {
		t.Fatalf("non-string name: expected *Error, got %T (%v)", res, res)
	}
}
