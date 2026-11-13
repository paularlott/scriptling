package stdlib

// Moved from extlibs/shlex_tempfile_shutil_test.go with the shlex library.

import "testing"

// ===================== shlex =====================

func TestShlexQuote(t *testing.T) {
	cases := map[string]string{
		"":          "''",
		"safe":      "safe",
		"file.txt":  "file.txt",
		"a/b/c-d_e": "a/b/c-d_e",
		"has space": "'has space'",
		"it's":      "'it'\"'\"'s'",
		"$HOME":     "'$HOME'",
		"a@b.com":   "a@b.com",
		"100%":      "100%",
	}
	for input, want := range cases {
		got := shlexQuote(input)
		if got != want {
			t.Errorf("quote(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestShlexSplit(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"a b c", []string{"a", "b", "c"}},
		{"  spaced  out  ", []string{"spaced", "out"}},
		{`'single quoted'`, []string{"single quoted"}},
		{`"double quoted"`, []string{"double quoted"}},
		{`arg\ with\ spaces`, []string{"arg with spaces"}},
		{`--flag="quoted value"`, []string{"--flag=quoted value"}},
		{`one 'two three' four`, []string{"one", "two three", "four"}},
		{``, nil},
	}
	for _, c := range cases {
		got, err := shlexSplit(c.input)
		if err != nil {
			t.Errorf("split(%q) error: %v", c.input, err)
			continue
		}
		if !equalStrSlices(got, c.want) {
			t.Errorf("split(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestShlexSplitErrors(t *testing.T) {
	if _, err := shlexSplit("unterminated 'quote"); err == nil {
		t.Error("expected error for unterminated single quote")
	}
	if _, err := shlexSplit(`unterminated "quote`); err == nil {
		t.Error("expected error for unterminated double quote")
	}
	if _, err := shlexSplit("trailing\\"); err == nil {
		t.Error("expected error for trailing backslash")
	}
}

func TestShlexJoin(t *testing.T) {
	got := shlexJoin([]string{"safe", "has space", ""})
	want := "safe 'has space' ''"
	if got != want {
		t.Errorf("join = %q, want %q", got, want)
	}
}

// equalStrSlices is a test helper.
func equalStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
