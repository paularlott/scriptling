package stdlib

import (
	"context"
	"net/url"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var urlLibrary = object.NewLibrary(map[string]*object.Builtin{
	"encode": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			encoded := url.QueryEscape(str.Value)
			return &object.String{Value: encoded}
		},
	},
	"decode": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			decoded, err := url.QueryUnescape(str.Value)
			if err != nil {
				return errors.NewError("decode() invalid URL encoding")
			}
			return &object.String{Value: decoded}
		},
	},
	"parse": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			u, err := url.Parse(str.Value)
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
	},
	"build": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			dict, ok := args[0].(*object.Dict)
			if !ok {
				return errors.NewTypeError("DICT", string(args[0].Type()))
			}

			u := &url.URL{}

			if pair, ok := dict.Pairs["scheme"]; ok {
				if str, ok := pair.Value.(*object.String); ok {
					u.Scheme = str.Value
				}
			}
			if pair, ok := dict.Pairs["host"]; ok {
				if str, ok := pair.Value.(*object.String); ok {
					u.Host = str.Value
				}
			}
			if pair, ok := dict.Pairs["path"]; ok {
				if str, ok := pair.Value.(*object.String); ok {
					u.Path = str.Value
				}
			}
			if pair, ok := dict.Pairs["query"]; ok {
				if str, ok := pair.Value.(*object.String); ok {
					u.RawQuery = str.Value
				}
			}
			if pair, ok := dict.Pairs["fragment"]; ok {
				if str, ok := pair.Value.(*object.String); ok {
					u.Fragment = str.Value
				}
			}

			return &object.String{Value: u.String()}
		},
	},
	"join": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			base, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			ref, ok := args[1].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[1].Type()))
			}

			baseURL, err := url.Parse(base.Value)
			if err != nil {
				return errors.NewError("join() invalid base URL")
			}

			refURL, err := url.Parse(ref.Value)
			if err != nil {
				return errors.NewError("join() invalid reference URL")
			}

			joined := baseURL.ResolveReference(refURL)
			return &object.String{Value: joined.String()}
		},
	},
	"query_parse": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}

			values, err := url.ParseQuery(str.Value)
			if err != nil {
				return errors.NewError("query_parse() invalid query string")
			}

			// Convert to dict
			pairs := make(map[string]object.DictPair)
			for key, vals := range values {
				// Join multiple values with comma
				value := strings.Join(vals, ",")
				pairs[key] = object.DictPair{
					Key:   &object.String{Value: key},
					Value: &object.String{Value: value},
				}
			}

			return &object.Dict{Pairs: pairs}
		},
	},
})

func GetURLLibrary() *object.Library {
	return urlLibrary
}
