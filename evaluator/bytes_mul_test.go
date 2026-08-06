package evaluator

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestBytesMultiplication covers BYTES * INT repetition, including the
// int64-overflow guard in evalBytesMultiplication that must return a clean
// error instead of panicking via make() with a negative capacity.
func TestBytesMultiplication(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string // expected bytes content; "" for empty
	}{
		{"bytes times int", `result = bytes("ab") * 3`, "ababab"},
		{"int times bytes", `result = 3 * bytes("ab")`, "ababab"},
		{"single repeat", `result = bytes("x") * 1`, "x"},
		{"zero repeat is empty", `result = bytes("ab") * 0`, ""},
		{"negative repeat is empty", `result = bytes("ab") * -1`, ""},
		{"empty bytes times anything", `result = bytes("") * 5`, ""},
	}
	for _, c := range cases {
		got := evalSrc(t, c.src)
		if object.IsError(got) {
			t.Errorf("%s: unexpected error: %q", c.name, got.Inspect())
			continue
		}
		b, ok := got.(*object.Bytes)
		if !ok {
			t.Errorf("%s: got %s, want BYTES", c.name, got.Type())
			continue
		}
		if string(b.BytesValue()) != c.want {
			t.Errorf("%s: got %q, want %q", c.name, b.BytesValue(), c.want)
		}
	}
}

// TestBytesMultiplicationOverflow verifies the allocation-size guard fires
// when len(src)*multiplier would overflow int64, returning a runtime error
// rather than crashing the interpreter. With srcLen=2, a multiplier of 2^62
// makes the product (2^63) exceed MaxInt64.
func TestBytesMultiplicationOverflow(t *testing.T) {
	got := evalSrc(t, "result = bytes(\"ab\") * 4611686018427387904")
	if !object.IsError(got) {
		b, _ := got.(*object.Bytes)
		t.Fatalf("expected overflow error, got %s (len=%d)", got.Type(), func() int {
			if b != nil {
				return len(b.BytesValue())
			}
			return 0
		}())
	}
	if !strings.Contains(got.Inspect(), "too large") {
		t.Errorf("expected 'too large' in error, got %q", got.Inspect())
	}
}
