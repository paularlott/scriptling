package evaluator

import (
	"strings"
	"testing"
	"time"

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

// TestRepetitionQuotas pins the repetition guards: string, bytes, list and
// tuple repetition refuse oversized results with a clean error (previously
// the list path panicked on len arithmetic, and constant-folded string
// repetition panicked outside the evaluator's recovery boundary), while
// large-but-bounded repetitions still work.
func TestRepetitionQuotas(t *testing.T) {
	cases := []struct {
		src string
	}{
		{`"ab" * 4611686018427387904`},
		{`4611686018427387904 * "ab"`},
		{`n = 4611686018427387904` + "\n" + `"ab" * n`},
		{`[1, 2, 3] * 4611686018427387904`},
		{`4611686018427387904 * [1, 2, 3]`},
		{`(1, 2) * 4611686018427387904`},
		{`4611686018427387904 * (1, 2)`},
		{`"ab" * 1073741825`}, // one past the 1 GiB byte quota
	}
	for _, tc := range cases {
		_, err := evalInEnv(t, tc.src)
		if err == nil {
			t.Errorf("%q: expected a too-large refusal", tc.src)
			continue
		}
		if !strings.Contains(err.Error(), "repetition result too large") {
			t.Errorf("%q: unexpected error: %v", tc.src, err)
		}
	}

	// Bounded repetitions of every spelling still evaluate.
	result, err := evalInEnv(t, `
mixed = [3 * "xy", "xy" * 3, 2 * [1], [1] * 2, (7,) * 2, 2 * (7,), len("z" * 1000000)]
mixed
`)
	if err != nil {
		t.Fatalf("bounded repetition failed: %v", err)
	}
	want := `[xyxyxy, xyxyxy, [1, 1], [1, 1], (7, 7), (7, 7), 1000000]`
	if result.Inspect() != want {
		t.Fatalf("bounded repetition = %s, want %s", result.Inspect(), want)
	}
}

// TestEmptyRepetitionIsInstant pins the CPU guard: multiplying an empty
// string, bytes or list by a huge count used to loop that many times over
// nothing (effectively forever) even though the result is empty.
func TestEmptyRepetitionIsInstant(t *testing.T) {
	src := `
huge = 4611686018427387904
results = [len("" * huge), len(huge * ""), len(bytes() * huge), len(huge * bytes()), [] * huge, huge * [], () * huge, huge * ()]
`
	done := make(chan object.Object, 1)
	go func() {
		result, err := evalInEnv(t, src+"\nresults")
		if err != nil {
			t.Errorf("eval: %v", err)
		}
		done <- result
	}()
	select {
	case r := <-done:
		want := "[0, 0, 0, 0, [], [], (), ()]"
		if r == nil || r.Inspect() != want {
			t.Fatalf("empty repetition results = %v, want %s", r, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("empty repetition ran past five seconds: CPU burn not short-circuited")
	}
}
