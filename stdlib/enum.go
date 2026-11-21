package stdlib

import (
	"context"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// EnumLibrary provides enum.Enum / enum.IntEnum bases and enum.auto. The
// bases are plain Class singletons: the evaluator's class compiler
// recognizes them by name and transforms deriving classes (members, value
// lookup, iteration); stdlib stays free of evaluator dependencies.

// The bases derive from one another as in Python (IntEnum, StrEnum and
// Flag from Enum, IntFlag from Flag), so isinstance(m, Enum) holds for
// every member.
var enumBaseClass = &object.Class{Name: "Enum"}
var intEnumBaseClass = &object.Class{Name: "IntEnum", BaseClass: enumBaseClass}
var strEnumBaseClass = &object.Class{Name: "StrEnum", BaseClass: enumBaseClass}
var flagBaseClass = &object.Class{Name: "Flag", BaseClass: enumBaseClass}
var intFlagBaseClass = &object.Class{Name: "IntFlag", BaseClass: flagBaseClass}

// AutoMarker is the sentinel returned by auto(); the class compiler
// replaces it with a sequential value.
var AutoMarker = object.NewSentinel("auto", "auto")

var EnumLibrary = object.NewLibrary(EnumLibraryName, map[string]*object.Builtin{
	"unique": {
		Fn:       enumUnique,
		HelpText: "unique(enum_class) - Class decorator: raise ValueError if any member is an alias (shares another's value)",
	},
}, map[string]object.Object{
	"Enum":    enumBaseClass,
	"IntEnum": intEnumBaseClass,
	"StrEnum": strEnumBaseClass,
	"Flag":    flagBaseClass,
	"IntFlag": intFlagBaseClass,
	"auto": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("auto", kwargs); err != nil {
				return err
			}
			return AutoMarker
		},
		HelpText: `auto() - Placeholder for an automatically numbered enum member value

Valid only inside an enum class body: RED = auto(), BLUE = auto().`,
	},
}, "Enumeration support (enum.Enum, enum.IntEnum, enum.StrEnum, enum.Flag, enum.IntFlag, enum.auto)")

// enumUnique is @enum.unique: an alias (a name whose value an earlier
// member already has) is a ValueError naming each alias and its member.
func enumUnique(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.NoKwargs("unique", kwargs); err != nil {
		return err
	}
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}
	cls, ok := args[0].(*object.Class)
	if !ok || !cls.IsEnum {
		return errors.NewTypeErrorTagged("unique() argument must be an Enum class, not %s", object.PythonTypeName(args[0]))
	}
	var dups []string
	for _, n := range cls.AssignNames {
		m, ok := cls.Methods[n].(*object.Instance)
		if !ok || m.Class != cls {
			continue
		}
		if mn, ok := m.Field("name").(*object.String); ok && mn.StringValue() != n {
			dups = append(dups, n+" -> "+mn.StringValue())
		}
	}
	if len(dups) > 0 {
		return errors.NewValueError("duplicate values found in <enum '%s'>: %s", cls.Name, strings.Join(dups, ", "))
	}
	return cls
}
