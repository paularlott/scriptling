package stdlib

import (
	"github.com/paularlott/scriptling/object"
	"net/url"
	"strings"
)

func GetURLLibrary() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"encode": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "encode() takes 1 argument"}
				}
				str, ok := args[0].(*object.String)
				if !ok {
					return &object.Error{Message: "encode() argument must be string"}
				}
				encoded := url.QueryEscape(str.Value)
				return &object.String{Value: encoded}
			},
		},
		"decode": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "decode() takes 1 argument"}
				}
				str, ok := args[0].(*object.String)
				if !ok {
					return &object.Error{Message: "decode() argument must be string"}
				}
				decoded, err := url.QueryUnescape(str.Value)
				if err != nil {
					return &object.Error{Message: "decode() invalid URL encoding"}
				}
				return &object.String{Value: decoded}
			},
		},
		"parse": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "parse() takes 1 argument"}
				}
				str, ok := args[0].(*object.String)
				if !ok {
					return &object.Error{Message: "parse() argument must be string"}
				}
				u, err := url.Parse(str.Value)
				if err != nil {
					return &object.Error{Message: "parse() invalid URL"}
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
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "build() takes 1 argument"}
				}
				dict, ok := args[0].(*object.Dict)
				if !ok {
					return &object.Error{Message: "build() argument must be dict"}
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
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.Error{Message: "join() takes 2 arguments"}
				}
				base, ok := args[0].(*object.String)
				if !ok {
					return &object.Error{Message: "join() first argument must be string"}
				}
				ref, ok := args[1].(*object.String)
				if !ok {
					return &object.Error{Message: "join() second argument must be string"}
				}
				
				baseURL, err := url.Parse(base.Value)
				if err != nil {
					return &object.Error{Message: "join() invalid base URL"}
				}
				
				refURL, err := url.Parse(ref.Value)
				if err != nil {
					return &object.Error{Message: "join() invalid reference URL"}
				}
				
				joined := baseURL.ResolveReference(refURL)
				return &object.String{Value: joined.String()}
			},
		},
		"query_parse": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "query_parse() takes 1 argument"}
				}
				str, ok := args[0].(*object.String)
				if !ok {
					return &object.Error{Message: "query_parse() argument must be string"}
				}
				
				values, err := url.ParseQuery(str.Value)
				if err != nil {
					return &object.Error{Message: "query_parse() invalid query string"}
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
	}
}
