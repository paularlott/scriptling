package stdlib

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// ParseResult class for URL parsing results
// ParseResultClass is the class of urlparse() results (see urlparse_py.go
// for it, urlsplit, urljoin and friends).
var ParseResultClass = &object.Class{Name: "ParseResult", Methods: map[string]object.Object{}}

// URLParseLibrary implements Python's urllib.parse module
var URLParseLibrary = object.NewLibrary(URLParseLibraryName, map[string]*object.Builtin{
	"quote": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("quote", kwargs, "safe"); err != nil {
					return err
				}
			}
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}

			str, err := args[0].AsString()
			if err != nil {
				return err
			}

			// Optional safe parameter; Python's default keeps '/' unencoded.
			safe := "/"
			if len(args) == 2 {
				safe, err = args[1].AsString()
				if err != nil {
					return err
				}
			} else if v, ok := kwargs.Lookup("safe"); ok {
				safe, err = v.AsString()
				if err != nil {
					return err
				}
			}

			encoded := urlQuote(str, safe)
			return object.NewString(encoded)
		},
		HelpText: `quote(string, safe='') - URL encode string

Returns a URL-encoded version of the string. Characters in 'safe' are not encoded.`,
	},
	"quote_plus": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("quote_plus", kwargs, "safe"); err != nil {
					return err
				}
			}
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}

			str, err := args[0].AsString()
			if err != nil {
				return err
			}

			// Optional safe parameter
			safe := ""
			if len(args) == 2 {
				safe, err = args[1].AsString()
				if err != nil {
					return err
				}
			} else if v, ok := kwargs.Lookup("safe"); ok {
				safe, err = v.AsString()
				if err != nil {
					return err
				}
			}

			// quote_plus replaces spaces with + and encodes other chars
			encoded := urlQuote(str, safe)
			encoded = strings.ReplaceAll(encoded, "%20", "+")
			return object.NewString(encoded)
		},
		HelpText: `quote_plus(string, safe='') - URL encode string with + for spaces

Like quote(), but also replaces spaces with plus signs.`,
	},
	"unquote": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("unquote", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}

			str, err := args[0].AsString()
			if err != nil {
				return err
			}

			return object.NewString(pyUnquote(str))
		},
		HelpText: `unquote(string) - URL decode string

Returns a URL-decoded version of the string.`,
	},
	"unquote_plus": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("unquote_plus", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}

			str, err := args[0].AsString()
			if err != nil {
				return err
			}

			// unquote_plus replaces + with space first, then unquotes
			str = strings.ReplaceAll(str, "+", " ")
			return object.NewString(pyUnquote(str))
		},
		HelpText: `unquote_plus(string) - URL decode string with + as spaces

Like unquote(), but also replaces plus signs with spaces.`,
	},
	"parse_qs": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("parse_qs", kwargs, "keep_blank_values"); err != nil {
					return err
				}
			}
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}

			str, err := args[0].AsString()
			if err != nil {
				return err
			}
			keepBlank, errObj := keepBlankArg(args, kwargs)
			if errObj != nil {
				return errObj
			}

			// Keys in first-seen order, each with its values in order.
			result := object.NewDict()
			for _, kv := range parseQueryPairs(str, keepBlank) {
				if pair, ok := result.GetByString(kv[0]); ok {
					list := pair.Value.(*object.List)
					list.Elements = append(list.Elements, object.NewString(kv[1]))
					continue
				}
				result.SetByString(kv[0], &object.List{Elements: []object.Object{object.NewString(kv[1])}})
			}
			return result
		},
		HelpText: `parse_qs(qs, keep_blank_values=False) - Parse query string

Parses a URL query string and returns a dictionary where values are lists.`,
	},
	"parse_qsl": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("parse_qsl", kwargs, "keep_blank_values"); err != nil {
					return err
				}
			}
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}

			str, err := args[0].AsString()
			if err != nil {
				return err
			}
			keepBlank, errObj := keepBlankArg(args, kwargs)
			if errObj != nil {
				return errObj
			}

			// (key, value) tuples in the order they appear.
			pairs := parseQueryPairs(str, keepBlank)
			result := make([]object.Object, len(pairs))
			for i, kv := range pairs {
				result[i] = &object.Tuple{Elements: []object.Object{object.NewString(kv[0]), object.NewString(kv[1])}}
			}
			return &object.List{Elements: result}
		},
		HelpText: `parse_qsl(qs, keep_blank_values=False) - Parse query string as list

Parses a URL query string and returns a list of (key, value) tuples.`,
	},
	"urlencode": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("urlencode", kwargs, "doseq"); err != nil {
					return err
				}
			}
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}

			// Pairs keep their order, as in Python, and a value that is not
			// a string is written as str() would show it.
			var out strings.Builder
			add := func(key string, val object.Object) {
				text, err := val.AsString()
				if err != nil {
					text = val.Inspect()
				}
				if out.Len() > 0 {
					out.WriteByte('&')
				}
				out.WriteString(url.QueryEscape(key))
				out.WriteByte('=')
				out.WriteString(url.QueryEscape(text))
			}
			addPair := func(key string, val object.Object) {
				switch v := val.(type) {
				case *object.List:
					for _, elem := range v.Elements {
						add(key, elem)
					}
				case *object.Tuple:
					for _, elem := range v.Elements {
						add(key, elem)
					}
				default:
					add(key, val)
				}
			}
			keyText := func(key object.Object) string {
				if text, err := key.AsString(); err == nil {
					return text
				}
				return key.Inspect()
			}

			switch arg := args[0].(type) {
			case *object.Dict:
				for _, pair := range arg.All() {
					addPair(keyText(pair.Key), pair.Value)
				}
			case *object.List:
				// List of (key, value) tuples
				for _, elem := range arg.Elements {
					tuple, ok := elem.(*object.Tuple)
					if !ok || len(tuple.Elements) != 2 {
						return errors.NewTypeErrorTagged("not a valid non-string sequence or mapping object")
					}
					addPair(keyText(tuple.Elements[0]), tuple.Elements[1])
				}
			default:
				return errors.NewTypeError("DICT or LIST", args[0].Type().String())
			}

			return object.NewString(out.String())
		},
		HelpText: `urlencode(query, doseq=False) - Encode dictionary as query string

Encodes a dictionary or list of tuples into a URL query string.`,
	},
}, nil, "URL parsing and manipulation (urllib.parse compatible)")

// URLLibrary is the parent urllib module - registered so `import urllib` works
var URLLibLibrary = object.NewLibrary(URLLibLibraryName, nil, nil, "URL handling modules")

// urlQuote encodes a string for URL, with optional safe characters
func urlQuote(s string, safe string) string {
	// Python's quote: everything outside [A-Za-z0-9._~-] plus the safe
	// characters becomes %XX (uppercase hex), byte-wise over UTF-8. Go's
	// PathEscape is not equivalent (it leaves path-legal characters such
	// as '=' unescaped).
	var result strings.Builder
	for _, b := range []byte(s) {
		c := rune(b)
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' || strings.ContainsRune(safe, c) {
			result.WriteByte(b)
		} else {
			result.WriteString(fmt.Sprintf("%%%02X", b))
		}
	}
	return result.String()
}

// keepBlankArg reads parse_qs/parse_qsl's keep_blank_values, given second or
// by keyword.
func keepBlankArg(args []object.Object, kwargs object.Kwargs) (bool, object.Object) {
	v := kwargs.Get("keep_blank_values")
	if len(args) > 1 {
		if v != nil {
			return false, errors.NewTypeErrorTagged("argument for parse_qs() given by name ('keep_blank_values') and position (2)")
		}
		v = args[1]
	}
	if v == nil {
		return false, nil
	}
	return v.AsBool()
}

// parseQueryPairs splits a query string as Python's parse_qsl does: fields
// separated by &, + as space, %-escapes decoded leniently (a bad one is kept
// as it is), and fields with an empty value, or no = at all, dropped unless
// keepBlank is set.
func parseQueryPairs(qs string, keepBlank bool) [][2]string {
	var pairs [][2]string
	for _, field := range strings.Split(qs, "&") {
		if field == "" {
			continue
		}
		name, value, hasEq := strings.Cut(field, "=")
		if !hasEq && !keepBlank {
			continue
		}
		if value == "" && !keepBlank {
			continue
		}
		pairs = append(pairs, [2]string{
			pyUnquote(strings.ReplaceAll(name, "+", " ")),
			pyUnquote(strings.ReplaceAll(value, "+", " ")),
		})
	}
	return pairs
}

// pyUnquote decodes %XX escapes as Python's urllib.parse.unquote does: an
// escape that is not two hex digits is left as it is, and bytes that do not
// form UTF-8 become U+FFFD, never an error.
func pyUnquote(s string) string {
	if strings.IndexByte(s, '%') < 0 {
		return s
	}
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHexDigit(s[i+1]) && isHexDigit(s[i+2]) {
			b = append(b, unhex(s[i+1])<<4|unhex(s[i+2]))
			i += 2
			continue
		}
		b = append(b, s[i])
	}
	if utf8.Valid(b) {
		return string(b)
	}
	var out strings.Builder
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		out.WriteRune(r) // RuneError for each invalid byte
		b = b[size:]
	}
	return out.String()
}

func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	}
	return c - 'A' + 10
}
