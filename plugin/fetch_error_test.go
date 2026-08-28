package plugin

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestMapFetchErrorDoesNotDoubleSentinel covers the wrapping contract between
// fetchers and hosts. Fetchers are told to wrap ErrFetchNotFound themselves, so
// the message that arrives over the wire usually already starts with the
// sentinel's text. Re-prefixing it produced "fetch source not found: fetch
// source not found: knot://x", which users saw verbatim.
func TestMapFetchErrorDoesNotDoubleSentinel(t *testing.T) {
	sentinel := ErrFetchNotFound.Error()

	cases := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "fetcher already wrapped the sentinel",
			message: fmt.Sprintf("%s: rv://nope", sentinel),
			want:    sentinel + ": rv://nope",
		},
		{
			name:    "fetcher wrapped with a path detail",
			message: fmt.Sprintf("%s: lib/x.py in knot://libs", sentinel),
			want:    sentinel + ": lib/x.py in knot://libs",
		},
		{
			name:    "fetcher sent a bare message",
			message: "lib/x.py is gone",
			want:    sentinel + ": lib/x.py is gone",
		},
		{
			name:    "fetcher sent only the sentinel",
			message: sentinel,
			want:    sentinel,
		},
		{
			name:    "fetcher sent nothing",
			message: "",
			want:    sentinel,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := mapFetchError(&RPCError{Code: FetchNotFoundCode, Message: tc.message})
			if !errors.Is(err, ErrFetchNotFound) {
				t.Fatalf("expected the result to wrap ErrFetchNotFound, got %v", err)
			}
			if err.Error() != tc.want {
				t.Fatalf("mapFetchError = %q, want %q", err.Error(), tc.want)
			}
			// Whatever the input, the sentinel must never appear twice.
			if n := strings.Count(err.Error(), sentinel); n != 1 {
				t.Fatalf("sentinel appears %d times in %q, want exactly 1", n, err.Error())
			}
		})
	}
}

func TestMapFetchErrorPassesThroughOtherErrors(t *testing.T) {
	if got := mapFetchError(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}

	other := &RPCError{Code: -32000, Message: "backend unreachable"}
	got := mapFetchError(other)
	if errors.Is(got, ErrFetchNotFound) {
		t.Fatal("a non-not-found RPC error must not be mapped to ErrFetchNotFound")
	}
	if got.Error() != "backend unreachable" {
		t.Fatalf("expected the message preserved, got %q", got.Error())
	}

	plain := errors.New("transport closed")
	if mapFetchError(plain) != plain {
		t.Fatal("expected a plain error to pass through unchanged")
	}
}

func TestTrimNotFoundPrefix(t *testing.T) {
	sentinel := ErrFetchNotFound.Error()
	cases := []struct {
		in     string
		detail string
		found  bool
	}{
		{sentinel + ": x", "x", true},
		{sentinel, "", true},
		{sentinel + ":", "", true},
		{"other: x", "other: x", false},
		{"", "", false},
	}
	for _, tc := range cases {
		detail, found := trimNotFoundPrefix(tc.in)
		if detail != tc.detail || found != tc.found {
			t.Errorf("trimNotFoundPrefix(%q) = (%q, %v), want (%q, %v)", tc.in, detail, found, tc.detail, tc.found)
		}
	}
}
