package stdlib

import (
	"encoding/base64"
	"github.com/paularlott/scriptling/object"
)

func GetBase64Library() map[string]*object.Builtin {
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
				encoded := base64.StdEncoding.EncodeToString([]byte(str.Value))
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
				decoded, err := base64.StdEncoding.DecodeString(str.Value)
				if err != nil {
					return &object.Error{Message: "decode() invalid base64 string"}
				}
				return &object.String{Value: string(decoded)}
			},
		},
	}
}
