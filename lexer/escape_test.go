package lexer

import "testing"

func TestUnescapeString(t *testing.T) {
	for _, tc := range []struct {
		in    string
		bytes bool
		want  string
	}{
		{`plain`, false, "plain"},
		{`a\nb\tc`, false, "a\nb\tc"},
		{`\x41\101\0`, false, "AA\x00"},
		{`\u00e9\U0001F600`, false, "é\U0001F600"},
		{`\d+\s`, false, `\d+\s`}, // unknown escapes are kept
		{`\x4\xZZ\N{BULLET}`, false, `\x4\xZZ\N{BULLET}`},
		{"one\\\ntwo", false, "onetwo"},
		{`\xff\377`, true, "\xff\xff"}, // raw bytes
		{`\u1234`, true, `\u1234`},     // not an escape in bytes
		{`\uD800`, false, `\uD800`},    // surrogate kept as written
		{`end\`, false, `end\`},
	} {
		if got := UnescapeString(tc.in, tc.bytes); got != tc.want {
			t.Errorf("UnescapeString(%q, %v) = %q, want %q", tc.in, tc.bytes, got, tc.want)
		}
	}
}

func TestStringLiteralLineCounting(t *testing.T) {
	// Each literal spans a backslash continuation; the identifier after it
	// must be on line 3.
	const bs = "\\"
	for _, src := range []string{
		"x = \"a" + bs + "\nb\"\ny",
		"x = \"a" + bs + "\r\nb\"\ny",
		"x = f\"a" + bs + "\nb\"\ny",
		"x = r\"a" + bs + "\nb\"\ny",
		"x = \"\"\"a" + bs + "\"\nb\"\"\"\ny",
	} {
		l := New(src)
		tok := l.NextToken()
		for tok.Literal != "y" && tok.Type != "EOF" {
			tok = l.NextToken()
		}
		if tok.Line != 3 {
			t.Errorf("%q: y on line %d, want 3", src, tok.Line)
		}
	}
}

func TestTripleQuotedEscapedQuote(t *testing.T) {
	const bs = "\\"
	for src, want := range map[string]string{
		`"""a` + bs + `""""`:                        `a"`,
		`"""say ` + bs + `"""hi` + bs + `""" ok"""`: `say """hi""" ok`,
	} {
		if tok := New(src).NextToken(); tok.Literal != want {
			t.Errorf("%s: got %q, want %q", src, tok.Literal, want)
		}
	}
}
