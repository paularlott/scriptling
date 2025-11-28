package stdlib

import (
	"context"
	"net/url"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// URLParseLibrary implements Python's urllib.parse module
var URLParseLibrary = object.NewLibrary(map[string]*object.Builtin{
	"quote": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}

			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			// Optional safe parameter (characters not to encode)
			safe := ""
			if len(args) == 2 {
				safe, ok = args[1].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
			}

			encoded := urlQuote(str, safe)
			return &object.String{Value: encoded}
		},
		HelpText: `quote(string, safe='') - URL encode string

Returns a URL-encoded version of the string. Characters in 'safe' are not encoded.`,
	},
	"quote_plus": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}

			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			// Optional safe parameter
			safe := ""
			if len(args) == 2 {
				safe, ok = args[1].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
			}

			// quote_plus replaces spaces with + and encodes other chars
			encoded := urlQuote(str, safe)
			encoded = strings.ReplaceAll(encoded, "%20", "+")
			return &object.String{Value: encoded}
		},
		HelpText: `quote_plus(string, safe='') - URL encode string with + for spaces

Like quote(), but also replaces spaces with plus signs.`,
	},
	"unquote": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			decoded, err := url.PathUnescape(str)
			if err != nil {
				return errors.NewError("unquote() invalid URL encoding")
			}
			return &object.String{Value: decoded}
		},
		HelpText: `unquote(string) - URL decode string

Returns a URL-decoded version of the string.`,
	},
	"unquote_plus": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			// unquote_plus replaces + with space first, then unquotes
			str = strings.ReplaceAll(str, "+", " ")
			decoded, err := url.PathUnescape(str)
			if err != nil {
				return errors.NewError("unquote_plus() invalid URL encoding")
			}
			return &object.String{Value: decoded}
		},
		HelpText: `unquote_plus(string) - URL decode string with + as spaces

Like unquote(), but also replaces plus signs with spaces.`,
	},
	"urlparse": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			u, err := url.Parse(str)
			if err != nil {
				return errors.NewError("urlparse() invalid URL")
			}

			// Build netloc to include userinfo (matching Python's behavior)
			netloc := u.Host
			if u.User != nil {
				if pwd, hasPwd := u.User.Password(); hasPwd {
					netloc = u.User.Username() + ":" + pwd + "@" + u.Host
				} else {
					netloc = u.User.Username() + "@" + u.Host
				}
			}

			// Return dict with URL components matching Python's ParseResult
			pairs := make(map[string]object.DictPair)
			pairs["scheme"] = object.DictPair{
				Key:   &object.String{Value: "scheme"},
				Value: &object.String{Value: u.Scheme},
			}
			pairs["netloc"] = object.DictPair{
				Key:   &object.String{Value: "netloc"},
				Value: &object.String{Value: netloc},
			}
			pairs["path"] = object.DictPair{
				Key:   &object.String{Value: "path"},
				Value: &object.String{Value: u.Path},
			}
			pairs["params"] = object.DictPair{
				Key:   &object.String{Value: "params"},
				Value: &object.String{Value: ""},
			}
			pairs["query"] = object.DictPair{
				Key:   &object.String{Value: "query"},
				Value: &object.String{Value: u.RawQuery},
			}
			pairs["fragment"] = object.DictPair{
				Key:   &object.String{Value: "fragment"},
				Value: &object.String{Value: u.Fragment},
			}

			return &object.Dict{Pairs: pairs}
		},
		HelpText: `urlparse(urlstring) - Parse URL into components

Returns a dictionary with URL components: scheme, netloc, path, params, query, fragment.`,
	},
	"urlunparse": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			// Accept either dict or list/tuple
			switch arg := args[0].(type) {
			case *object.Dict:
				u := &url.URL{}
				if value, ok := arg.Pairs["scheme"]; ok {
					if str, ok := value.Value.AsString(); ok {
						u.Scheme = str
					}
				}
				if value, ok := arg.Pairs["netloc"]; ok {
					if str, ok := value.Value.AsString(); ok {
						u.Host = str
					}
				}
				if value, ok := arg.Pairs["path"]; ok {
					if str, ok := value.Value.AsString(); ok {
						u.Path = str
					}
				}
				if value, ok := arg.Pairs["query"]; ok {
					if str, ok := value.Value.AsString(); ok {
						u.RawQuery = str
					}
				}
				if value, ok := arg.Pairs["fragment"]; ok {
					if str, ok := value.Value.AsString(); ok {
						u.Fragment = str
					}
				}
				return &object.String{Value: u.String()}

			case *object.List:
				if len(arg.Elements) != 6 {
					return errors.NewError("urlunparse() requires exactly 6 elements")
				}
				components := make([]string, 6)
				for i, elem := range arg.Elements {
					str, ok := elem.AsString()
					if !ok {
						return errors.NewTypeError("STRING", elem.Type().String())
					}
					components[i] = str
				}
				u := &url.URL{
					Scheme:   components[0],
					Host:     components[1],
					Path:     components[2],
					RawQuery: components[4],
					Fragment: components[5],
				}
				return &object.String{Value: u.String()}

			case *object.Tuple:
				if len(arg.Elements) != 6 {
					return errors.NewError("urlunparse() requires exactly 6 elements")
				}
				components := make([]string, 6)
				for i, elem := range arg.Elements {
					str, ok := elem.AsString()
					if !ok {
						return errors.NewTypeError("STRING", elem.Type().String())
					}
					components[i] = str
				}
				u := &url.URL{
					Scheme:   components[0],
					Host:     components[1],
					Path:     components[2],
					RawQuery: components[4],
					Fragment: components[5],
				}
				return &object.String{Value: u.String()}

			default:
				return errors.NewTypeError("DICT, LIST, or TUPLE", args[0].Type().String())
			}
		},
		HelpText: `urlunparse(components) - Construct URL from components

Constructs a URL string from a 6-tuple or dict of URL components.`,
	},
	"urljoin": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}

			base, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			ref, ok := args[1].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}

			baseURL, err := url.Parse(base)
			if err != nil {
				return errors.NewError("urljoin() invalid base URL")
			}

			refURL, err := url.Parse(ref)
			if err != nil {
				return errors.NewError("urljoin() invalid reference URL")
			}

			joined := baseURL.ResolveReference(refURL)
			return &object.String{Value: joined.String()}
		},
		HelpText: `urljoin(base, url) - Join base URL with reference

Joins a base URL with a reference URL, resolving relative references.`,
	},
	"urlsplit": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			u, err := url.Parse(str)
			if err != nil {
				return errors.NewError("urlsplit() invalid URL")
			}

			// Build netloc to include userinfo (matching Python's behavior)
			netloc := u.Host
			if u.User != nil {
				if pwd, hasPwd := u.User.Password(); hasPwd {
					netloc = u.User.Username() + ":" + pwd + "@" + u.Host
				} else {
					netloc = u.User.Username() + "@" + u.Host
				}
			}

			// Return tuple-like list with URL components (scheme, netloc, path, query, fragment)
			elements := []object.Object{
				&object.String{Value: u.Scheme},
				&object.String{Value: netloc},
				&object.String{Value: u.Path},
				&object.String{Value: u.RawQuery},
				&object.String{Value: u.Fragment},
			}

			return &object.List{Elements: elements}
		},
		HelpText: `urlsplit(urlstring) - Split URL into components

Returns a 5-tuple: (scheme, netloc, path, query, fragment).`,
	},
	"urlunsplit": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			elements, ok := args[0].AsList()
			if !ok {
				return errors.NewTypeError("LIST or TUPLE", args[0].Type().String())
			}

			if len(elements) != 5 {
				return errors.NewError("urlunsplit() requires exactly 5 elements")
			}

			components := make([]string, 5)
			for i, elem := range elements {
				str, ok := elem.AsString()
				if !ok {
					return errors.NewTypeError("STRING", elem.Type().String())
				}
				components[i] = str
			}

			u := &url.URL{
				Scheme:   components[0],
				Host:     components[1],
				Path:     components[2],
				RawQuery: components[3],
				Fragment: components[4],
			}

			return &object.String{Value: u.String()}
		},
		HelpText: `urlunsplit(components) - Construct URL from component tuple

Constructs a URL string from a 5-tuple of URL components.`,
	},
	"parse_qs": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}

			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			values, err := url.ParseQuery(str)
			if err != nil {
				return errors.NewError("parse_qs() invalid query string")
			}

			// Convert to dict with lists of values
			pairs := make(map[string]object.DictPair)
			for key, vals := range values {
				elements := make([]object.Object, len(vals))
				for i, val := range vals {
					elements[i] = &object.String{Value: val}
				}
				pairs[key] = object.DictPair{
					Key:   &object.String{Value: key},
					Value: &object.List{Elements: elements},
				}
			}

			return &object.Dict{Pairs: pairs}
		},
		HelpText: `parse_qs(qs, keep_blank_values=False) - Parse query string

Parses a URL query string and returns a dictionary where values are lists.`,
	},
	"parse_qsl": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}

			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			values, err := url.ParseQuery(str)
			if err != nil {
				return errors.NewError("parse_qsl() invalid query string")
			}

			// Convert to list of (key, value) tuples
			var result []object.Object
			for key, vals := range values {
				for _, val := range vals {
					tuple := &object.Tuple{
						Elements: []object.Object{
							&object.String{Value: key},
							&object.String{Value: val},
						},
					}
					result = append(result, tuple)
				}
			}

			return &object.List{Elements: result}
		},
		HelpText: `parse_qsl(qs, keep_blank_values=False) - Parse query string as list

Parses a URL query string and returns a list of (key, value) tuples.`,
	},
	"urlencode": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}

			values := make(url.Values)

			switch arg := args[0].(type) {
			case *object.Dict:
				for key, pair := range arg.Pairs {
					switch val := pair.Value.(type) {
					case *object.String:
						values[key] = []string{val.Value}
					case *object.List:
						strVals := make([]string, len(val.Elements))
						for i, elem := range val.Elements {
							if str, ok := elem.AsString(); ok {
								strVals[i] = str
							}
						}
						values[key] = strVals
					}
				}
			case *object.List:
				// List of (key, value) tuples
				for _, elem := range arg.Elements {
					if tuple, ok := elem.(*object.Tuple); ok && len(tuple.Elements) == 2 {
						if key, ok := tuple.Elements[0].AsString(); ok {
							if val, ok := tuple.Elements[1].AsString(); ok {
								values.Add(key, val)
							}
						}
					}
				}
			default:
				return errors.NewTypeError("DICT or LIST", args[0].Type().String())
			}

			return &object.String{Value: values.Encode()}
		},
		HelpText: `urlencode(query, doseq=False) - Encode dictionary as query string

Encodes a dictionary or list of tuples into a URL query string.`,
	},
}, nil, "URL parsing and manipulation (urllib.parse compatible)")

// URLLibrary is the parent urllib module with parse as a sub-library
var URLLibLibrary = object.NewLibraryWithSubs(
	nil, // No functions at urllib level
	nil, // No constants
	map[string]*object.Library{
		"parse": URLParseLibrary,
	},
	"URL handling modules",
)

// urlQuote encodes a string for URL, with optional safe characters
func urlQuote(s string, safe string) string {
	var result strings.Builder
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' || strings.ContainsRune(safe, c) {
			result.WriteRune(c)
		} else if c == ' ' {
			result.WriteString("%20")
		} else {
			result.WriteString(url.PathEscape(string(c)))
		}
	}
	return result.String()
}
