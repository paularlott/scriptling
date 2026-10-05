package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// TypingLibrary is a placeholder module, not a type system: annotations are
// parsed and discarded by scriptling, so typing names exist only to make
// annotated code run. Each name subscripting (List[int], Optional[List[int]])
// returns itself, so arbitrary annotation expressions resolve.

var typingAliasClass = &object.Class{
	Name: "typing-alias",
	Methods: map[string]object.Object{
		"__getitem__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return args[0]
			},
			HelpText: "__getitem__(item) - Typing aliases accept any subscript",
		},
	},
}

func typingAlias(name string) *object.Instance {
	inst := object.NewInstance(typingAliasClass)
	inst.SetField("__alias_name__", object.NewString(name))
	return inst
}

var TypingLibrary = object.NewLibrary(TypingLibraryName, nil, map[string]object.Object{
	"Any":       typingAlias("Any"),
	"Callable":  typingAlias("Callable"),
	"Dict":      typingAlias("Dict"),
	"Iterable":  typingAlias("Iterable"),
	"Iterator":  typingAlias("Iterator"),
	"List":      typingAlias("List"),
	"Literal":   typingAlias("Literal"),
	"Mapping":   typingAlias("Mapping"),
	"Optional":  typingAlias("Optional"),
	"Sequence":  typingAlias("Sequence"),
	"Set":       typingAlias("Set"),
	"Tuple":     typingAlias("Tuple"),
	"Type":      typingAlias("Type"),
	"Union":     typingAlias("Union"),
}, "Typing placeholders: annotations are accepted and discarded, as always")

var _ = errors.NewError
var _ = context.Background
