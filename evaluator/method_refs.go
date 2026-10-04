package evaluator

import (
	"context"
	"fmt"
	"sort"

	"github.com/paularlott/scriptling/object"
)

// builtinMethodNames lists the methods each built-in type answers to, so a
// method can be referenced without calling it (cb = items.append), found by
// hasattr and dir, and used unbound through its type (key=str.lower). Calls
// still go straight to the per-type dispatchers in methods.go; these names
// must match their cases, which TestBuiltinMethodTablesMatchDispatch checks.
var builtinMethodNames = map[object.ObjectType][]string{
	object.STRING_OBJ: {
		"capitalize", "casefold", "center", "count", "encode", "endswith", "expandtabs", "find",
		"format", "index", "isalnum", "isalpha", "isdecimal", "isdigit", "isidentifier", "islower",
		"isnumeric", "isprintable", "isspace", "istitle", "isupper", "join", "ljust", "lower",
		"lstrip", "maketrans", "partition", "removeprefix", "removesuffix", "replace", "rfind",
		"rindex", "rjust", "rpartition", "rsplit", "rstrip", "split", "splitlines", "startswith",
		"strip", "swapcase", "title", "translate", "upper", "zfill",
	},
	object.LIST_OBJ: {
		"append", "clear", "copy", "count", "extend", "index", "insert", "pop", "remove", "reverse", "sort",
	},
	object.DICT_OBJ: {
		"clear", "copy", "fromkeys", "get", "items", "keys", "pop", "setdefault", "update", "values",
	},
	object.TUPLE_OBJ: {"count", "index"},
	object.SET_OBJ: {
		"add", "clear", "copy", "difference", "difference_update", "discard", "intersection",
		"intersection_update", "issubset", "issuperset", "pop", "remove", "symmetric_difference",
		"symmetric_difference_update", "union", "update",
	},
	object.BYTES_OBJ:       {"base64", "decode", "hex", "length"},
	object.FLOAT_ARRAY_OBJ: {"shape", "tolist"},
	object.INTEGER_OBJ:     {"bit_count", "bit_length", "is_integer"},
	object.FLOAT_OBJ:       {"as_integer_ratio", "fromhex", "hex", "is_integer"},
}

// builtinMethodSet is builtinMethodNames indexed for lookup.
var builtinMethodSet = func() map[object.ObjectType]map[string]bool {
	sets := make(map[object.ObjectType]map[string]bool, len(builtinMethodNames))
	for t, names := range builtinMethodNames {
		set := make(map[string]bool, len(names))
		for _, n := range names {
			set[n] = true
		}
		sets[t] = set
	}
	return sets
}()

// staticTypeMethods are called through the type without an instance, as in
// Python: dict.fromkeys(keys), str.maketrans(x, y).
var staticTypeMethods = map[object.ObjectType]map[string]object.Object{
	object.DICT_OBJ:   {"fromkeys": &object.Dict{Pairs: map[string]object.DictPair{}}},
	object.STRING_OBJ: {"maketrans": object.NewString("")},
	object.FLOAT_OBJ:  {"fromhex": object.NewFloat(0)},
}

// hasBuiltinMethod reports whether a value of type t has the named method.
func hasBuiltinMethod(t object.ObjectType, name string) bool {
	return builtinMethodSet[t][name]
}

// builtinMethodRef returns obj.name as a bound method, or nil when obj's type
// has no such method.
func builtinMethodRef(obj object.Object, name string) object.Object {
	if !hasBuiltinMethod(obj.Type(), name) {
		return nil
	}
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return callStringMethodWithKeywords(ctx, obj, name, args, kwargs.Kwargs, GetEnvFromContext(ctx))
		},
		HelpText: fmt.Sprintf("%s.%s - bound method", getTypeName(obj), name),
		Repr:     fmt.Sprintf("<built-in method %s of %s object>", name, getTypeName(obj)),
	}
}

// unboundTypeMethod returns the type-level form of a method, such as
// str.lower, which takes the instance as its first argument.
func unboundTypeMethod(t object.ObjectType, typeName, name string) object.Object {
	if receiver, ok := staticTypeMethods[t][name]; ok {
		return &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return callStringMethodWithKeywords(ctx, receiver, name, args, kwargs.Kwargs, GetEnvFromContext(ctx))
			},
			HelpText: fmt.Sprintf("%s.%s", typeName, name),
			Repr:     fmt.Sprintf("<built-in method %s of type object>", name),
		}
	}
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 || args[0].Type() != t {
				got := "nothing"
				if len(args) > 0 {
					got = "a '" + getTypeName(args[0]) + "' object"
				}
				return &object.Exception{
					Message:       fmt.Sprintf("descriptor '%s' for '%s' objects doesn't apply to %s", name, typeName, got),
					ExceptionType: object.ExceptionTypeTypeError,
					Raised:        true,
				}
			}
			return callStringMethodWithKeywords(ctx, args[0], name, args[1:], kwargs.Kwargs, GetEnvFromContext(ctx))
		},
		HelpText: fmt.Sprintf("%s.%s(self, ...)", typeName, name),
		Repr:     fmt.Sprintf("<method '%s' of '%s' objects>", name, typeName),
	}
}

// attachTypeMethods gives the type builtins (str, list, ...) their methods
// as attributes, so str.lower or list.append can be passed as functions.
// Called from init(): the builtins map cannot refer to the dispatchers
// directly without an initialization cycle.
func attachTypeMethods() {
	for typeName, t := range map[string]object.ObjectType{
		"str": object.STRING_OBJ, "list": object.LIST_OBJ, "dict": object.DICT_OBJ,
		"tuple": object.TUPLE_OBJ, "set": object.SET_OBJ, "bytes": object.BYTES_OBJ,
		"int": object.INTEGER_OBJ, "float": object.FLOAT_OBJ,
	} {
		b := builtins[typeName]
		if b == nil {
			continue
		}
		if b.Attributes == nil {
			b.Attributes = make(map[string]object.Object, len(builtinMethodNames[t]))
		}
		for _, name := range builtinMethodNames[t] {
			if _, exists := b.Attributes[name]; !exists {
				b.Attributes[name] = unboundTypeMethod(t, typeName, name)
			}
		}
	}
}

// builtinMethodDir is dir() of a built-in value: its method names, sorted.
func builtinMethodDir(t object.ObjectType) []string {
	names := append([]string(nil), builtinMethodNames[t]...)
	sort.Strings(names)
	return names
}
