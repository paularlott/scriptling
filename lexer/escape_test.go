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
