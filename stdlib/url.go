package stdlib

import (
	"context"
	"net/url"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var urlLibrary = object.NewLibrary(map[string]*object.Builtin{
	"quote": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			encoded := url.PathEscape(str)
			return &object.String{Value: encoded}
		},
		HelpText: `quote(string) - URL encode string

Returns a URL-encoded version of the string.`,
	},
	"unquote": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
	"urlparse": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			u, err := url.Parse(str)
			if err != nil {
				return errors.NewError("parse() invalid URL")
			}

			// Return dict with URL components
			pairs := make(map[string]object.DictPair)
			pairs["scheme"] = object.DictPair{
				Key:   &object.String{Value: "scheme"},
				Value: &object.String{Value: u.Scheme},
			}
			pairs["host"] = object.DictPair{
				Key:   &object.String{Value: "host"},
				Value: &object.String{Value: u.Host},
			}
			pairs["path"] = object.DictPair{
				Key:   &object.String{Value: "path"},
				Value: &object.String{Value: u.Path},
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
		HelpText: `urlparse(url_string) - Parse URL into components

Returns a dictionary with URL components: scheme, host, path, query, fragment.`,
	},
	"urlunparse": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			dict, ok := args[0].AsDict()
			if !ok {
				return errors.NewTypeError("DICT", args[0].Type().String())
			}

			u := &url.URL{}

			if value, ok := dict["scheme"]; ok {
				if str, ok := value.AsString(); ok {
					u.Scheme = str
				}
			}
			if value, ok := dict["host"]; ok {
				if str, ok := value.AsString(); ok {
					u.Host = str
				}
			}
			if value, ok := dict["path"]; ok {
				if str, ok := value.AsString(); ok {
					u.Path = str
				}
			}
			if value, ok := dict["query"]; ok {
				if str, ok := value.AsString(); ok {
					u.RawQuery = str
				}
			}
			if value, ok := dict["fragment"]; ok {
				if str, ok := value.AsString(); ok {
					u.Fragment = str
				}
			}

			return &object.String{Value: u.String()}
		},
		HelpText: `urlunparse(dict) - Construct URL from components

Constructs a URL string from a dictionary of URL components.`,
	},
	"urljoin": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
				return errors.NewError("join() invalid base URL")
			}

			refURL, err := url.Parse(ref)
			if err != nil {
				return errors.NewError("join() invalid reference URL")
			}

			joined := baseURL.ResolveReference(refURL)
			return &object.String{Value: joined.String()}
		},
		HelpText: `urljoin(base, ref) - Join base URL with reference

Joins a base URL with a reference URL, resolving relative references.`,
	},
	"urlsplit": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
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

			// Return tuple-like list with URL components (scheme, netloc, path, query, fragment)
			elements := []object.Object{
				&object.String{Value: u.Scheme},
				&object.String{Value: u.Host},
				&object.String{Value: u.Path},
				&object.String{Value: u.RawQuery},
				&object.String{Value: u.Fragment},
			}

			return &object.List{Elements: elements}
		},
		HelpText: `urlsplit(url_string) - Split URL into components

Returns a list with URL components: [scheme, host, path, query, fragment].`,
	},
	"urlunsplit": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			elements, ok := args[0].AsList()
			if !ok {
				return errors.NewTypeError("LIST", args[0].Type().String())
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
		HelpText: `urlunsplit(list) - Construct URL from component list

Constructs a URL string from a list of 5 URL components.`,
	},
	"parse_qs": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
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
		HelpText: `parse_qs(query_string) - Parse query string

Parses a URL query string and returns a dictionary of key-value pairs.`,
	},
	"urlencode": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			dict, ok := args[0].AsDict()
			if !ok {
				return errors.NewTypeError("DICT", args[0].Type().String())
			}

			values := make(url.Values)
			for key, value := range dict {
				switch val := value.(type) {
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

			return &object.String{Value: values.Encode()}
		},
		HelpText: `urlencode(dict) - Encode dictionary as query string

Encodes a dictionary of key-value pairs into a URL query string.`,
	},
})

func GetURLLibrary() *object.Library {
	return object.NewLibraryWithDescription(urlLibrary.Functions(), "URL parsing and manipulation library")
}
