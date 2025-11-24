package stdlib

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"github.com/paularlott/scriptling/object"
)

func GetHashlibLibrary() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"sha256": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "sha256() takes 1 argument"}
				}
				str, ok := args[0].(*object.String)
				if !ok {
					return &object.Error{Message: "sha256() argument must be string"}
				}
				hash := sha256.Sum256([]byte(str.Value))
				return &object.String{Value: hex.EncodeToString(hash[:])}
			},
		},
		"sha1": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "sha1() takes 1 argument"}
				}
				str, ok := args[0].(*object.String)
				if !ok {
					return &object.Error{Message: "sha1() argument must be string"}
				}
				hash := sha1.Sum([]byte(str.Value))
				return &object.String{Value: hex.EncodeToString(hash[:])}
			},
		},
		"md5": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "md5() takes 1 argument"}
				}
				str, ok := args[0].(*object.String)
				if !ok {
					return &object.Error{Message: "md5() argument must be string"}
				}
				hash := md5.Sum([]byte(str.Value))
				return &object.String{Value: hex.EncodeToString(hash[:])}
			},
		},
	}
}
