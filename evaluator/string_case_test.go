package evaluator

import "testing"

// TestFastStringUpper covers the ASCII fast path and the Unicode fallback.
// The Unicode cases guard against the historical bug where the function would
// commit to a byte-only transform after seeing an ASCII lowercase letter and
// then corrupt later non-ASCII runes (e.g. "naïve" -> "NAïVE").
func TestFastStringUpper(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"all ascii lower", "hello", "HELLO"},
		{"mixed ascii", "Hello World", "HELLO WORLD"},
		{"digits and punctuation", "abc 123!?", "ABC 123!?"},
		{"already upper", "HELLO", "HELLO"},
		{"leading unicode", "éclair", "ÉCLAIR"},
		{"trailing unicode", "naïve", "NAÏVE"},
		{"multi word unicode", "naïve résumé", "NAÏVE RÉSUMÉ"},
		{"unicode only", "éàü", "ÉÀÜ"},
		{"non-letter unicode preserved", "a—b", "A—B"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fastStringUpper(tc.in)
			if got != tc.want {
				t.Errorf("fastStringUpper(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestFastStringLower mirrors TestFastStringUpper for the lowercase fast path.
func TestFastStringLower(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"all ascii upper", "HELLO", "hello"},
		{"mixed ascii", "Hello World", "hello world"},
		{"digits and punctuation", "ABC 123!?", "abc 123!?"},
		{"already lower", "hello", "hello"},
		{"leading unicode", "ÉCLAIR", "éclair"},
		{"trailing unicode", "NAÏVE", "naïve"},
		{"multi word unicode", "NAÏVE RÉSUMÉ", "naïve résumé"},
		{"unicode only", "ÉÀÜ", "éàü"},
		{"non-letter unicode preserved", "A—B", "a—b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fastStringLower(tc.in)
			if got != tc.want {
				t.Errorf("fastStringLower(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
