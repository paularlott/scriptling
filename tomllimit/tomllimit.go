// Package tomllimit rejects pathologically nested TOML before it reaches a
// decoder. The decoder in use (BurntSushi/toml) spends memory that grows far
// faster than the input as nesting deepens — a 24 KB document nested a few
// thousand levels deep allocates over a gigabyte — and that work happens
// inside the decoder, below any memory accounting, so a memory limit cannot
// stop it. A cheap single pass over the source caps the nesting depth first.
package tomllimit

import "fmt"

// MaxDepth is the deepest nesting any TOML document given to a decoder may
// have. Real configuration files nest a handful of levels; at this depth the
// decoder's memory stays at baseline, far below the catastrophic range.
const MaxDepth = 100

// ErrTooDeep is returned when a document nests deeper than the cap.
type ErrTooDeep struct{ Max int }

func (e ErrTooDeep) Error() string {
	return fmt.Sprintf("document nests more than %d levels deep", e.Max)
}

// Check reports an *ErrTooDeep when src nests deeper than MaxDepth, in any of
// the ways TOML nests: inline tables and arrays (`{` / `[` in a value), and
// dotted keys and table headers (`a.b.c = 1`, `[a.b.c]`). It is a single
// linear scan that respects strings (basic, literal and their multi-line
// forms) and comments, so brackets and dots inside them are not counted.
func Check(src string) error {
	return CheckMax(src, MaxDepth)
}

// CheckMax is Check with an explicit cap, for testing.
func CheckMax(src string, max int) error {
	// bracket is the live `{`/`[` nesting at this point in a value.
	// keyDots is the number of dots in the key currently being read (before
	// its `=`), so a dotted key or header counts toward depth too.
	bracket, keyDots := 0, 0
	// inKey is true while scanning a bare key (line start, or after a `.`),
	// up to its `=` or the end of a header's `]`. Dots only count there.
	inKey := true

	check := func() error {
		if bracket+keyDots > max {
			return &ErrTooDeep{Max: max}
		}
		return nil
	}

	s := src
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '#':
			// Comment: skip to end of line.
			for i < len(s) && s[i] != '\n' {
				i++
			}
			inKey = true
			keyDots = 0
		case '\n':
			inKey = true
			keyDots = 0
		case '"', '\'':
			i = skipString(s, i)
		case '{', '[':
			// A header `[` / `[[` at the very start of a line (bracket==0,
			// scanning a key) opens a table path, not a value array, so it is
			// counted through keyDots as its dots are read, not as a bracket.
			if c == '[' && bracket == 0 && inKey && atLineStart(s, i) {
				// Skip a second `[` of `[[` without counting it.
				if i+1 < len(s) && s[i+1] == '[' {
					i++
				}
				keyDots = 0
				continue
			}
			bracket++
			if err := check(); err != nil {
				return err
			}
			inKey = true // a new value or key begins inside the container
			keyDots = 0
		case '}', ']':
			if bracket > 0 {
				bracket--
			}
			// After a header's closing `]`, dots no longer count until the
			// next key; after an inline container, the same.
			inKey = false
		case '.':
			if inKey {
				keyDots++
				if err := check(); err != nil {
					return err
				}
			}
		case '=':
			inKey = false
			keyDots = 0
		case ',':
			// Next element of an inline array/table starts a fresh key scan.
			inKey = true
			keyDots = 0
		}
	}
	return nil
}

// atLineStart reports whether only whitespace precedes index i on its line.
func atLineStart(s string, i int) bool {
	for j := i - 1; j >= 0; j-- {
		switch s[j] {
		case ' ', '\t':
			continue
		case '\n':
			return true
		default:
			return false
		}
	}
	return true
}

// skipString advances past the string beginning at s[i] (a `"` or `'`),
// including the multi-line `"""` / `”'` forms, and returns the index of its
// closing quote. An unterminated string runs to end of input; the decoder
// reports that itself. Strings add no depth.
func skipString(s string, i int) int {
	q := s[i]
	multiline := i+2 < len(s) && s[i+1] == q && s[i+2] == q
	if multiline {
		j := i + 3
		for j+2 < len(s) {
			if s[j] == q && s[j+1] == q && s[j+2] == q {
				return j + 2
			}
			if q == '"' && s[j] == '\\' {
				j++ // skip the escaped char
			}
			j++
		}
		return len(s) - 1
	}
	for j := i + 1; j < len(s); j++ {
		if q == '"' && s[j] == '\\' {
			j++
			continue
		}
		if s[j] == q || s[j] == '\n' {
			return j
		}
	}
	return len(s) - 1
}
