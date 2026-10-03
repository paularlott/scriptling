package lexer

import (
	"strings"
	"unicode/utf8"
)

// UnescapeString decodes Python escape sequences in a string literal's
// content. In bytesMode, \x and octal escapes give raw bytes and \u, \U and
// \N are not escapes (as in a bytes literal).
func UnescapeString(s string, bytesMode bool) string {
	i := strings.IndexByte(s, '\\')
	if i < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:i])
	for i < len(s) {
		if s[i] == '\\' {
			i = AppendEscape(&b, s, i, bytesMode)
			continue
		}
		j := strings.IndexByte(s[i:], '\\')
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+j])
		i += j
	}
	return b.String()
}

// AppendEscape decodes the escape sequence starting at the backslash s[i],
// writes the result to b and returns the index just past it. Sequences that
// are not escapes (\d, \N{...}, an incomplete \x) are kept as written, as
// Python keeps unknown escapes.
func AppendEscape(b *strings.Builder, s string, i int, bytesMode bool) int {
	if i+1 >= len(s) {
		b.WriteByte('\\')
		return i + 1
	}
	c := s[i+1]
	if simple := strings.IndexByte(`\'"abfnrtv`, c); simple >= 0 {
		b.WriteByte("\\'\"\a\b\f\n\r\t\v"[simple])
		return i + 2
	}
	switch c {
	case '\n':
		// A backslash at the end of a line continues the literal.
		return i + 2
	case '\r':
		if i+2 < len(s) && s[i+2] == '\n' {
			return i + 3
		}
		return i + 2
	case '0', '1', '2', '3', '4', '5', '6', '7':
		j, v := i+1, 0
		for j < len(s) && j < i+4 && s[j] >= '0' && s[j] <= '7' {
			v = v*8 + int(s[j]-'0')
			j++
		}
		writeCode(b, rune(v), bytesMode)
		return j
	case 'x':
		if v, ok := hexValue(s, i+2, 2); ok {
			writeCode(b, v, bytesMode)
			return i + 4
		}
	case 'u', 'U':
		digits := 4
		if c == 'U' {
			digits = 8
		}
		if v, ok := hexValue(s, i+2, digits); ok && !bytesMode && utf8.ValidRune(v) {
			b.WriteRune(v)
			return i + 2 + digits
		}
	}
	b.WriteByte('\\')
	b.WriteByte(c)
	return i + 2
}

// writeCode writes an \x or octal escape: a raw byte in a bytes literal, the
// code point otherwise.
func writeCode(b *strings.Builder, v rune, bytesMode bool) {
	if bytesMode {
		b.WriteByte(byte(v))
		return
	}
	b.WriteRune(v)
}

func hexValue(s string, start, digits int) (rune, bool) {
	if start+digits > len(s) {
		return 0, false
	}
	var v rune
	for _, c := range []byte(s[start : start+digits]) {
		switch {
		case c >= '0' && c <= '9':
			v = v*16 + rune(c-'0')
		case c|0x20 >= 'a' && c|0x20 <= 'f':
			v = v*16 + rune(c|0x20-'a'+10)
		default:
			return 0, false
		}
	}
	return v, true
}
