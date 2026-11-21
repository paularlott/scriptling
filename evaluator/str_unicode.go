package evaluator

import (
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Unicode-aware helpers for str methods whose ASCII fast paths live in
// methods.go. Python applies Unicode's full case mappings (ß uppercases to
// SS, ﬁ to FI, a final Σ lowercases to ς) and its cased / uppercase /
// lowercase properties, which include Other_Uppercase and Other_Lowercase
// characters such as Ⅻ and ª. Each caser is built per call: a Caser keeps
// state and must not be shared between goroutines.

func unicodeUpper(s string) string { return cases.Upper(language.Und).String(s) }
func unicodeLower(s string) string { return cases.Lower(language.Und).String(s) }

// pyCasefold is str.casefold(): Unicode full case folding.
func pyCasefold(s string) string {
	if isASCII(s) {
		return fastStringLower(s)
	}
	return cases.Fold().String(s)
}

func isUpperRune(r rune) bool {
	if r < 0x80 {
		return 'A' <= r && r <= 'Z'
	}
	return unicode.IsUpper(r) || unicode.Is(unicode.Other_Uppercase, r)
}

func isLowerRune(r rune) bool {
	if r < 0x80 {
		return 'a' <= r && r <= 'z'
	}
	return unicode.IsLower(r) || unicode.Is(unicode.Other_Lowercase, r)
}

func isTitleRune(r rune) bool { return r >= 0x80 && unicode.IsTitle(r) }

func isCasedRune(r rune) bool { return isUpperRune(r) || isLowerRune(r) || isTitleRune(r) }

// titleRune is the full titlecase mapping of r (ß is Ss, ﬁ is Fi).
func titleRune(r rune) string {
	if r < 0x80 {
		if 'a' <= r && r <= 'z' {
			r -= 'a' - 'A'
		}
		return string(r)
	}
	if t, ok := specialTitle[r]; ok {
		return t
	}
	return string(unicode.ToTitle(r))
}

var specialTitle = map[rune]string{
	'ß': "Ss", 'ﬀ': "Ff", 'ﬁ': "Fi", 'ﬂ': "Fl", 'ﬃ': "Ffi", 'ﬄ': "Ffl", 'ﬅ': "St", 'ﬆ': "St", 'ŉ': "ʼN",
}

// lowerRune is the full lowercase mapping of r (İ is i̇).
func lowerRune(r rune) string {
	if r < 0x80 {
		if 'A' <= r && r <= 'Z' {
			r += 'a' - 'A'
		}
		return string(r)
	}
	if r == 'İ' {
		return "i̇"
	}
	return string(unicode.ToLower(r))
}

// pyTitle is str.title(): a cased character following a cased character is
// lowercased, any other is titlecased, so "they're" is "They'Re" and
// "hello_world" is "Hello_World", as in Python.
func pyTitle(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevCased := false
	for _, r := range s {
		if isCasedRune(r) {
			if prevCased {
				b.WriteString(lowerRune(r))
			} else {
				b.WriteString(titleRune(r))
			}
			prevCased = true
		} else {
			b.WriteRune(r)
			prevCased = false
		}
	}
	return b.String()
}

// pyCapitalize is str.capitalize(): the first character titlecased, the rest
// lowercased.
func pyCapitalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range s {
		if i == 0 {
			b.WriteString(titleRune(r))
			continue
		}
		rest := s[i:]
		if isASCII(rest) {
			b.WriteString(fastStringLower(rest))
		} else {
			b.WriteString(unicodeLower(rest))
		}
		break
	}
	return b.String()
}

// pySwapcase is str.swapcase(): uppercase characters lowercased and
// lowercase ones uppercased, with the full mappings (ß is SS).
func pySwapcase(s string) string {
	if isASCII(s) {
		b := []byte(s)
		for i, c := range b {
			switch {
			case 'A' <= c && c <= 'Z':
				b[i] = c + ('a' - 'A')
			case 'a' <= c && c <= 'z':
				b[i] = c - ('a' - 'A')
			}
		}
		return string(b)
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case isUpperRune(r):
			b.WriteString(lowerRune(r))
		case isLowerRune(r):
			b.WriteString(unicodeUpper(string(r)))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// pyIsLower, pyIsUpper and pyIsTitle are str.islower(), isupper() and
// istitle() over Python's cased characters.
func pyIsLower(s string) bool {
	cased := false
	for _, r := range s {
		if isUpperRune(r) || isTitleRune(r) {
			return false
		}
		if isLowerRune(r) {
			cased = true
		}
	}
	return cased
}

func pyIsUpper(s string) bool {
	cased := false
	for _, r := range s {
		if isLowerRune(r) || isTitleRune(r) {
			return false
		}
		if isUpperRune(r) {
			cased = true
		}
	}
	return cased
}

func pyIsTitle(s string) bool {
	cased, prevCased := false, false
	for _, r := range s {
		switch {
		case isUpperRune(r) || isTitleRune(r):
			if prevCased {
				return false
			}
			prevCased, cased = true, true
		case isLowerRune(r):
			if !prevCased {
				return false
			}
			prevCased, cased = true, true
		default:
			prevCased = false
		}
	}
	return cased
}

// pyIsIdentifier is str.isidentifier(): a letter (or letter number) or
// underscore, then letters, digits, combining marks and connector
// punctuation.
func pyIsIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		start := r == '_' || unicode.IsLetter(r) || unicode.Is(unicode.Nl, r)
		if i == 0 {
			if !start {
				return false
			}
			continue
		}
		if !start && !unicode.IsDigit(r) && !unicode.In(r, unicode.Mn, unicode.Mc, unicode.Pc) {
			return false
		}
	}
	return true
}

// isDigitRune is Python's str.isdigit() per character: decimal digits plus
// the characters of Numeric_Type Digit (superscripts, subscripts, circled
// and parenthesized digits).
func isDigitRune(r rune) bool {
	if unicode.IsDigit(r) {
		return true
	}
	switch {
	case r == 0xB2 || r == 0xB3 || r == 0xB9,
		r == 0x2070 || (r >= 0x2074 && r <= 0x2079),
		r >= 0x2080 && r <= 0x2089,
		r >= 0x2460 && r <= 0x2468, r >= 0x2474 && r <= 0x247C, r >= 0x2488 && r <= 0x2490,
		r == 0x24EA, r >= 0x24F5 && r <= 0x24FD, r == 0x24FF,
		r >= 0x2776 && r <= 0x277E, r >= 0x2780 && r <= 0x2788, r >= 0x278A && r <= 0x2792,
		r >= 0x1F100 && r <= 0x1F10A:
		return true
	}
	return false
}

// isNumericRune is Python's str.isnumeric() per character: any Unicode
// number, plus the common CJK numeral ideographs (Numeric_Type Numeric).
func isNumericRune(r rune) bool {
	if unicode.IsNumber(r) {
		return true
	}
	switch r {
	case '〇', '一', '二', '三', '四', '五', '六', '七', '八', '九', '十', '百', '千', '万', '億', '兆', '零', '壹', '贰', '叁', '肆', '伍', '陆', '柒', '捌', '玖', '拾', '佰', '仟':
		return true
	}
	return false
}
