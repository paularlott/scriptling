package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// TypingLibrary is a placeholder module, not a type system: annotations are
// parsed and discarded by scriptling, so typing names exist only to make
// annotated code run. Each name subscripting (List[int], Optional[List[int]])
// returns itself, so arbitrary annotation expressions resolve. Generic and
// Protocol (and TypedDict) are classes to derive from (class
// Box(Generic[T])), cast and the decorators return their argument, and
// NamedTuple called as a function builds a namedtuple class.

var typingAliasClass = &object.Class{
	Name: "typing-alias",
	Methods: map[string]object.Object{
		"__getitem__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("__getitem__", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				// Optional[X] is X | None and Union[X, Y] is X | Y, which
				// isinstance accepts; other aliases subscript to themselves.
				self, _ := args[0].(*object.Instance)
				if self == nil {
					return args[0]
				}
				if name, ok := self.Field("__alias_name__").(*object.String); ok {
					switch name.StringValue() {
					case "Optional":
						return object.NewTypeUnion([]object.Object{args[1], &object.Null{}})
					case "Union":
						if t, ok := args[1].(*object.Tuple); ok {
							return object.NewTypeUnion(t.Elements)
						}
						return args[1]
					}
				}
				return args[0]
			},
			HelpText: "__getitem__(item) - Optional[X] and Union[X, Y] make a union; other aliases accept any subscript",
		},
		"__call__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return args[0]
			},
			HelpText: "__call__(...) - TypeVar('T'), NewType('Id', int) and the like make an alias",
		},
	},
}

func typingAlias(name string) *object.Instance {
	inst := object.NewInstance(typingAliasClass)
	inst.SetField("__alias_name__", object.NewString(name))
	inst.SetField("__name__", object.NewString(name))
	return inst
}

// selfSubscript is __class_getitem__ returning the class: Generic[T].
var selfSubscript = &object.Builtin{
	Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if err := errors.NoKwargs("__class_getitem__", kwargs); err != nil {
			return err
		}
		if err := errors.ExactArgs(args, 2); err != nil {
			return err
		}
		return args[0]
	},
	HelpText: "__class_getitem__(params) - The class itself",
}

func typingBaseClass(name string) *object.Class {
	return &object.Class{Name: name, Methods: map[string]object.Object{"__class_getitem__": selfSubscript}}
}

// identity returns its first argument: cast(T, x) is x is handled apart;
// overload, final, runtime_checkable and no_type_check decorate as no-ops.
func identityBuiltin(name, help string) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs(name, kwargs); err != nil {
				return err
			}
			if len(args) != 1 {
				return errors.NewTypeErrorTagged("%s() takes exactly one argument (%d given)", name, len(args))
			}
			return args[0]
		},
		HelpText: help,
	}
}

func init() {
	fns := TypingLibrary.Functions()
	fns["cast"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("cast", kwargs, "typ", "val"); err != nil {
					return err
				}
				if v := kwargs.Get("val"); v != nil {
					return v
				}
			}
			if len(args) != 2 {
				return errors.NewTypeErrorTagged("cast() takes exactly 2 arguments (%d given)", len(args))
			}
			return args[1]
		},
		HelpText: "cast(typ, val) - val, unchanged",
	}
	for _, n := range []string{"overload", "final", "runtime_checkable", "no_type_check", "override"} {
		fns[n] = identityBuiltin(n, n+"(f) - Returns f unchanged")
	}
	fns["assert_never"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("assert_never", kwargs); err != nil {
				return err
			}
			return errors.NewTypeErrorTagged("Expected code to be unreachable")
		},
		HelpText: "assert_never(arg) - Raises: the code should be unreachable",
	}
	fns["reveal_type"] = identityBuiltin("reveal_type", "reveal_type(obj) - Returns obj")
	fns["get_type_hints"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("get_type_hints", kwargs, "globalns", "localns", "include_extras"); err != nil {
					return err
				}
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			return object.NewDict()
		},
		HelpText: "get_type_hints(obj) - {}: annotations are not kept",
	}
	fns["NamedTuple"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// NamedTuple('P', [('x', int), ('y', int)]) is namedtuple('P', ['x', 'y']).
			if err := errors.NoKwargs("NamedTuple", kwargs); err != nil {
				return err
			}
			if len(args) < 1 {
				return errors.NewTypeErrorTagged("NamedTuple() missing required argument: 'typename'")
			}
			var names []object.Object
			if len(args) > 1 {
				fields, errObj := iterableElements(ctx, args[1])
				if errObj != nil {
					return errObj
				}
				for _, f := range fields {
					pair, errObj := iterableElements(ctx, f)
					if errObj != nil || len(pair) != 2 {
						return errors.NewTypeErrorTagged("NamedTuple fields must be (name, type) pairs")
					}
					names = append(names, pair[0])
				}
			}
			return CollectionsLibrary.Functions()["namedtuple"].Fn(ctx, object.NewKwargs(nil), args[0], &object.List{Elements: names})
		},
		HelpText: "NamedTuple(typename, [(name, type), ...]) - A namedtuple class",
	}
	object.ClassBaseHook = typingClassBaseHook
	consts := TypingLibrary.Constants()
	for _, n := range []string{"Generic", "Protocol", "TypedDict"} {
		consts[n] = typingBaseClass(n)
	}
	consts["TYPE_CHECKING"] = object.NewBoolean(false)
	for _, n := range []string{"TypeVar", "NewType", "ParamSpec", "TypeVarTuple", "Final", "ClassVar", "Annotated",
		"NoReturn", "Never", "Self", "FrozenSet", "DefaultDict", "Deque", "Counter", "OrderedDict", "ChainMap",
		"Generator", "AsyncGenerator", "AsyncIterator", "AsyncIterable", "Awaitable", "Coroutine", "Hashable", "Sized",
		"Collection", "Container", "Reversible", "MutableMapping", "MutableSequence", "MutableSet", "AbstractSet",
		"KeysView", "ValuesView", "ItemsView", "ContextManager", "AbstractContextManager", "AnyStr", "Text",
		"Pattern", "Match", "IO", "TextIO", "BinaryIO", "SupportsInt", "SupportsFloat", "SupportsIndex", "SupportsAbs",
		"SupportsRound", "Concatenate", "TypeAlias", "TypeGuard", "TypeIs", "Required", "NotRequired", "ReadOnly",
		"Unpack", "LiteralString", "ByteString", "NamedTupleMeta"} {
		if _, taken := consts[n]; !taken {
			consts[n] = typingAlias(n)
		}
	}
}

var TypingLibrary = object.NewLibrary(TypingLibraryName, map[string]*object.Builtin{}, map[string]object.Object{
	"Any":      typingAlias("Any"),
	"Callable": typingAlias("Callable"),
	"Dict":     typingAlias("Dict"),
	"Iterable": typingAlias("Iterable"),
	"Iterator": typingAlias("Iterator"),
	"List":     typingAlias("List"),
	"Literal":  typingAlias("Literal"),
	"Mapping":  typingAlias("Mapping"),
	"Optional": typingAlias("Optional"),
	"Sequence": typingAlias("Sequence"),
	"Set":      typingAlias("Set"),
	"Tuple":    typingAlias("Tuple"),
	"Type":     typingAlias("Type"),
	"Union":    typingAlias("Union"),
}, "Typing placeholders: annotations are accepted and discarded, as always")

// typingClassBaseHook makes class P(NamedTuple) a namedtuple class.
func typingClassBaseHook(base *object.Builtin) func(context.Context, *object.Class) object.Object {
	if base == TypingLibrary.Functions()["NamedTuple"] {
		return namedTupleFromClass
	}
	return nil
}

// namedTupleFromClass builds the namedtuple class for class P(NamedTuple):
// the annotated names are the fields, their assigned values the defaults,
// and the body's other members (methods, class attributes, the docstring)
// are kept.
func namedTupleFromClass(ctx context.Context, cls *object.Class) object.Object {
	names := make([]object.Object, len(cls.FieldNames))
	var defaults []object.Object
	isField := make(map[string]bool, len(cls.FieldNames))
	for i, f := range cls.FieldNames {
		names[i] = object.NewString(f)
		isField[f] = true
		if v, ok := cls.Methods[f]; ok {
			defaults = append(defaults, v)
		} else if len(defaults) > 0 {
			return errors.NewTypeErrorTagged("Non-default namedtuple field %s cannot follow default field %s", f, cls.FieldNames[i-1])
		}
	}
	kw := map[string]object.Object{}
	if len(defaults) > 0 {
		kw["defaults"] = &object.Tuple{Elements: defaults}
	}
	made := CollectionsLibrary.Functions()["namedtuple"].Fn(ctx, object.NewKwargs(kw), object.NewString(cls.Name), &object.List{Elements: names})
	nt, ok := made.(*object.Class)
	if !ok {
		return made
	}
	for name, m := range cls.Methods {
		if !isField[name] {
			nt.Methods[name] = m
		}
	}
	return nt
}
