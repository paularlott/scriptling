package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/object"
)

// EnumLibrary provides enum.Enum / enum.IntEnum bases and enum.auto. The
// bases are plain Class singletons: the evaluator's class compiler
// recognizes them by name and transforms deriving classes (members, value
// lookup, iteration); stdlib stays free of evaluator dependencies.

var enumBaseClass = &object.Class{Name: "Enum"}
var intEnumBaseClass = &object.Class{Name: "IntEnum"}

// AutoMarker is the sentinel returned by auto(); the class compiler
// replaces it with a sequential value.
var AutoMarker = object.NewSentinel("auto", "auto")

var EnumLibrary = object.NewLibrary(EnumLibraryName, nil, map[string]object.Object{
	"Enum":    enumBaseClass,
	"IntEnum": intEnumBaseClass,
	"auto": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return AutoMarker
		},
		HelpText: `auto() - Placeholder for an automatically numbered enum member value

Valid only inside an enum class body: RED = auto(), BLUE = auto().`,
	},
}, "Enumeration support (enum.Enum, enum.IntEnum, enum.auto)")
