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

var StringLibrary = object.NewLibrary(StringLibraryName, 
	nil, // No functions, only constants
	map[string]object.Object{
		"ascii_letters":   object.NewString(asciiLetters),
		"ascii_lowercase": object.NewString(asciiLowercase),
		"ascii_uppercase": object.NewString(asciiUppercase),
		"digits":          object.NewString(digits),
		"hexdigits":       object.NewString(hexdigits),
		"octdigits":       object.NewString(octdigits),
		"punctuation":     object.NewString(punctuation),
		"whitespace":      object.NewString(whitespace),
		"printable":       object.NewString(printable),
	},
	"String constants for character classification",
)
