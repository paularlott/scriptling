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

// TestPackageLibraryGlob verifies package.glob's pattern language over a
// nested bundle. The recursive idiom "**/*.ext" is the one docs lean on, so
// it must match files at any depth including the package root — and "*" must
// stay within one segment.
func TestPackageLibraryGlob(t *testing.T) {
	files := map[string]string{
		"readme.md":         "# root",
		"docs/one.md":       "# one",
		"docs/sub/two.md":   "# two",
		"docs/sub/three.md": "# three",
		"lib/mod.py":        "x = 1",
	}
	b := bundleFromMap(t, "name=\"app\"\nversion=\"1\"\n", files)
	l := NewLoader()
	if err := l.AddBundle(b); err != nil {
		t.Fatalf("AddBundle: %v", err)
	}
	glob := NewPackageLibrary(l).Functions()["glob"].Fn

	match := func(pattern string) []string {
		t.Helper()
		res := glob(context.Background(), object.NewKwargs(nil),
			object.NewString("app"), object.NewString(pattern))
		list, ok := res.(*object.List)
		if !ok {
			t.Fatalf("glob(%q) returned %T (%v), want list", pattern, res, res)
		}
		got := make([]string, 0, len(list.Elements))
		for _, e := range list.Elements {
			s, ok := e.(*object.String)
			if !ok {
				t.Fatalf("glob(%q) element %T, want string", pattern, e)
			}
			got = append(got, s.StringValue())
		}
		return got
	}

	assertMatches := func(pattern string, want ...string) {
		t.Helper()
		got := match(pattern)
		if len(got) != len(want) {
			t.Fatalf("glob(%q) = %v, want %v", pattern, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("glob(%q) = %v, want %v", pattern, got, want)
			}
		}
	}

	// Recursive: every .md anywhere, root included (zero segments for **).
	assertMatches("**/*.md", "docs/one.md", "docs/sub/three.md", "docs/sub/two.md", "readme.md")
	// Single segment: root .md files only, never deeper.
	assertMatches("*.md", "readme.md")
	// One directory level: files directly inside docs (directories are not
	// matched; package.list lists them).
	assertMatches("docs/*", "docs/one.md")
	// A subtree under a prefix.
	assertMatches("docs/**/*.md", "docs/one.md", "docs/sub/three.md", "docs/sub/two.md")
	// Everything (the bundle's manifest.toml is part of the walked tree).
	assertMatches("**", "docs/one.md", "docs/sub/three.md", "docs/sub/two.md", "lib/mod.py", "manifest.toml", "readme.md")
	// No match is an empty list, not an error.
	assertMatches("*.txt")

	// Unknown package is an error object, not a silent empty list.
	res := glob(context.Background(), object.NewKwargs(nil),
		object.NewString("nope"), object.NewString("*.md"))
	if _, ok := res.(*object.Error); !ok {
		t.Fatalf("unknown package: expected *Error, got %T (%v)", res, res)
	}
}
