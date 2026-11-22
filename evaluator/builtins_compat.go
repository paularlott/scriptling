package evaluator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Builtins Python code expects that sit outside the core table: object,
// issubclass over builtin types, ascii, locals, globals and Ellipsis.

// objectClass is the class of the bare instances object() returns.
var objectClass = &object.Class{Name: "object", Methods: map[string]object.Object{}}

// objectBuiltin is the builtin object: the root of every type. Calling it
// gives a new featureless instance (the unique-sentinel idiom), class C(object)
// means class C, and isinstance(x, object) / issubclass(T, object) are True.
var objectBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) > 0 || kwargs.Len() > 0 {
			return errors.NewTypeErrorTagged("object() takes no arguments")
		}
		return object.NewInstance(objectClass)
	},
	HelpText: `object() - The base of every type; object() returns a new unique instance

  _MISSING = object()          # a sentinel no other value is
  if value is _MISSING: ...

class C(object) is the same as class C. isinstance(x, object) is always True.
object.__setattr__(obj, name, value) sets an attribute directly, bypassing any
__setattr__ the class defines.`,
	Repr: "<class 'object'>",
	Attributes: map[string]object.Object{
		"__setattr__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 3); err != nil {
					return err
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeErrorTagged("object.__setattr__() requires an instance, not '%s'", getTypeName(args[0]))
				}
				name, ok := args[1].(*object.String)
				if !ok {
					return errors.NewTypeErrorTagged("attribute name must be string, not '%s'", getTypeName(args[1]))
				}
				inst.SetField(name.StringValue(), args[2])
				inst.InvalidateBoundMethod(name.StringValue())
				return NULL
			},
			HelpText: `object.__setattr__(obj, name, value) - Set obj.name directly, bypassing the class's __setattr__`,
			Repr:     "<slot wrapper '__setattr__' of 'object' objects>",
		},
	},
}

// ellipsisValue is the builtin Ellipsis. builtins holds only callables, so
// evalIdentifier resolves the name after a builtins miss.
var ellipsisValue = object.NewSentinel("Ellipsis", "Ellipsis")

// notImplementedValue is the builtin NotImplemented: a binary dunder that
// returns it declines the operation, so the reflected dunder on the other
// operand is tried, then the default (identity for ==, else TypeError).
var notImplementedValue = object.NewSentinel("NotImplemented", "NotImplemented")

func init() {
	builtins["object"] = objectBuiltin
	typeBuiltins[objectBuiltin] = "object"
	// range, map and the other lazy iterables are types too: type(r) is
	// range and isinstance(m, map) hold, and they print as classes.
	for _, name := range []string{"range", "map", "filter", "zip", "enumerate", "reversed"} {
		if b, ok := builtins[name]; ok {
			typeBuiltins[b] = name
			b.Repr = "<class '" + name + "'>"
		}
	}
	builtins["ascii"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("ascii", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			rendered, rerr := renderConvertedValue(ctx, args[0], "a", GetEnvFromContext(ctx))
			if rerr != nil {
				return rerr
			}
			return object.NewString(rendered)
		},
		HelpText: `ascii(object) - repr(object) with non-ASCII characters escaped

  ascii("café")   # "'caf\\xe9'"`,
	}
	builtins["locals"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("locals", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			return scopeDict(GetEnvFromContext(ctx))
		},
		HelpText: `locals() - A dict of the current scope's variables

A snapshot: assigning into the dict does not change the variables.`,
	}
	builtins["globals"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("globals", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			return scopeDict(GetEnvFromContext(ctx).GetGlobal())
		},
		HelpText: `globals() - A dict of the module's global variables

A snapshot: assigning into the dict does not change the variables.`,
	}
}

// scopeDict is a snapshot of env's own bindings, in definition order.
func scopeDict(env *object.Environment) *object.Dict {
	d := object.NewDict()
	if env == nil {
		return d
	}
	names, values := env.Bindings()
	for i, name := range names {
		if values[i] == nil || name == "__class__" {
			continue
		}
		d.SetByString(name, values[i])
	}
	return d
}

// asciiEscape escapes the non-ASCII characters of s as Python's ascii() does.
func asciiEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&b, "\\x%02x", r)
		case r <= 0xffff:
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			fmt.Fprintf(&b, "\\U%08x", r)
		}
	}
	return b.String()
}

// isClassLike reports whether o is a class or a builtin type, for issubclass.
func isClassLike(o object.Object) bool {
	switch v := o.(type) {
	case *object.Class:
		return true
	case *object.Builtin:
		if v == objectBuiltin || v == object.DefaultDictType || v == object.OrderedDictType {
			return true
		}
		if _, ok := typeBuiltins[v]; ok {
			return true
		}
		if _, ok := object.BuiltinTypeClasses(v); ok {
			return true
		}
		if syntheticTypeName(v) != "" {
			return true
		}
		_, ok := exceptionClassName(v)
		return ok
	}
	return false
}

// exceptionNameOf is the built-in exception type a class-like value is or
// derives from, "" when it is not an exception type.
func exceptionNameOf(o object.Object) string {
	switch v := o.(type) {
	case *object.Class:
		for c := v; c != nil; c = c.BaseClass {
			if c.ExceptionBase != "" {
				return c.ExceptionBase
			}
		}
	case *object.Builtin:
		name, _ := exceptionClassName(v)
		return name
	}
	return ""
}

// issubclassOne is issubclass(cls, parent) for one parent.
func issubclassOne(cls, parent object.Object) bool {
	if cls == parent || parent == objectBuiltin {
		return true
	}
	if p, ok := parent.(*object.Class); ok {
		if c, ok := cls.(*object.Class); ok {
			for ; c != nil; c = c.BaseClass {
				if c == p {
					return true
				}
			}
		}
		return false
	}
	pb := parent.(*object.Builtin)
	if pExc, ok := exceptionClassName(pb); ok {
		cExc := exceptionNameOf(cls)
		return cExc != "" && matchesNamedExceptionTypeChain(cExc, pExc, nil)
	}
	if c, ok := cls.(*object.Class); ok {
		// A class whose chain reaches a builtin type's stand-in base (a
		// namedtuple class derives from one named tuple), matched by name
		// as isinstance does.
		if name := typeBuiltins[pb]; name != "" {
			for c = c.BaseClass; c != nil; c = c.BaseClass {
				if c.Name == name {
					return true
				}
			}
		}
		return false
	}
	cb, ok := cls.(*object.Builtin)
	if !ok {
		return false
	}
	switch typeBuiltins[pb] {
	case "int":
		return typeBuiltins[cb] == "bool"
	case "dict":
		return cb == object.DefaultDictType || cb == object.OrderedDictType
	}
	return false
}

// issubclassFn is issubclass(cls, classinfo): classinfo is a class, a
// builtin type or a tuple of them.
func issubclassFn(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.NoKwargs("issubclass", kwargs); err != nil {
		return err
	}
	if err := errors.ExactArgs(args, 2); err != nil {
		return err
	}
	cls := args[0]
	if !isClassLike(cls) {
		return errors.NewTypeErrorTagged("issubclass() arg 1 must be a class")
	}
	parents := []object.Object{args[1]}
	if t, ok := args[1].(*object.Tuple); ok {
		parents = t.Elements
	}
	for _, p := range parents {
		if members, isUnion := object.TypeUnionMembers(p); isUnion {
			for _, m := range members {
				if _, none := m.(*object.Null); !none && isClassLike(m) && issubclassOne(cls, m) {
					return TRUE
				}
			}
			continue
		}
		if !isClassLike(p) {
			return errors.NewTypeErrorTagged("issubclass() arg 2 must be a class, a tuple of classes, or a union")
		}
		if issubclassOne(cls, p) {
			return TRUE
		}
	}
	return FALSE
}

// abstractMethodNames lists, sorted, the members of an ABC class still
// marked abstract (inherited ones included: Methods holds them flattened),
// or nil when there are none.
func abstractMethodNames(class *object.Class) []string {
	var names []string
	for name, m := range class.Methods {
		if isAbstractMember(m) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func isAbstractMember(m object.Object) bool {
	switch v := m.(type) {
	case *object.Function:
		return v.Abstract
	case *object.Property:
		return isAbstractMember(v.Getter)
	case *object.StaticMethod:
		return isAbstractMember(v.Fn)
	case *object.ClassMethod:
		return isAbstractMember(v.Fn)
	}
	return false
}

// abstractInstantiationError is Python's TypeError for instantiating a
// class with unimplemented abstract methods.
func abstractInstantiationError(class *object.Class) object.Object {
	quoted := make([]string, len(class.AbstractMethods))
	for i, n := range class.AbstractMethods {
		quoted[i] = "'" + n + "'"
	}
	plural := ""
	if len(quoted) > 1 {
		plural = "s"
	}
	return &object.Exception{
		Message: fmt.Sprintf("Can't instantiate abstract class %s without an implementation for abstract method%s %s",
			class.Name, plural, strings.Join(quoted, ", ")),
		ExceptionType: object.ExceptionTypeTypeError,
		Raised:        true,
	}
}

// numberAttribute is the real, imag, numerator and denominator attributes of
// an int, bool or float, nil for anything else.
func numberAttribute(v object.Object, attr string) object.Object {
	switch n := v.(type) {
	case *object.Integer:
		switch attr {
		case "real", "numerator":
			return n
		case "imag":
			return object.NewInteger(0)
		case "denominator":
			return object.NewInteger(1)
		}
	case *object.Boolean:
		return numberAttribute(object.NewInteger(boolToInt64(n.BoolValue())), attr)
	case *object.Float:
		switch attr {
		case "real":
			return n
		case "imag":
			return object.NewFloat(0)
		}
	}
	return nil
}

// exceptionCause converts the value of `raise X from c` to the exception
// stored as __cause__: nil for None.
func exceptionCause(ctx context.Context, c object.Object, env *object.Environment) (*object.Exception, object.Object) {
	switch v := c.(type) {
	case *object.Null:
		return nil, nil
	case *object.Exception:
		return v, nil
	case *object.Instance:
		if exc, chain, args := exceptionFromClassInstance(ctx, v, env); exc != nil {
			exc.TypeChain = chain
			exc.Args = args
			return exc, nil
		}
	case *object.Builtin:
		if _, ok := exceptionClassName(v); ok {
			if exc, ok := v.Fn(SetEnvInContext(ctx, env), object.NewKwargs(nil)).(*object.Exception); ok {
				return exc, nil
			}
		}
	case *object.Class:
		if v.ExceptionBase != "" {
			return &object.Exception{ExceptionType: v.Name, TypeChain: exceptionClassChain(v)}, nil
		}
	}
	return nil, &object.Exception{Message: "exception causes must derive from BaseException", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
}
