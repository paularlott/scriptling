package stdlib

import (
	"github.com/paularlott/scriptling/object"
)

// String constants matching Python's string module
const (
	asciiLowercase = "abcdefghijklmnopqrstuvwxyz"
	asciiUppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	asciiLetters   = asciiLowercase + asciiUppercase
	digits         = "0123456789"
	hexdigits      = "0123456789abcdefABCDEF"
	octdigits      = "01234567"
	punctuation    = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	whitespace     = " \t\n\r\v\f"
	printable      = digits + asciiLetters + punctuation + whitespace
)

var StringLibrary = object.NewLibrary(
	nil, // No functions, only constants
	map[string]object.Object{
		"ascii_letters":   &object.String{Value: asciiLetters},
		"ascii_lowercase": &object.String{Value: asciiLowercase},
		"ascii_uppercase": &object.String{Value: asciiUppercase},
		"digits":          &object.String{Value: digits},
		"hexdigits":       &object.String{Value: hexdigits},
		"octdigits":       &object.String{Value: octdigits},
		"punctuation":     &object.String{Value: punctuation},
		"whitespace":      &object.String{Value: whitespace},
		"printable":       &object.String{Value: printable},
	},
	"String constants for character classification",
)
