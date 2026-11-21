package evaluator

import (
	"context"
	"fmt"
	"math"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Dunder methods of built-in values and types: "a".__len__(), (1).__add__(2),
// d.__getitem__(k), int.__add__(1, 2), list.__iadd__(xs, ys),
// object.__repr__(x). Each maps to the operator or builtin that implements
// it. They are looked up only once a value's own attributes and methods have
// missed (the attribute-error paths), so a successful lookup does no extra
// work.

// dunderArity is the number of arguments (after self) each supported dunder
// takes: {min, max}.
var dunderArity = map[string][2]int{
	"__eq__": {1, 1}, "__ne__": {1, 1}, "__lt__": {1, 1}, "__le__": {1, 1}, "__gt__": {1, 1}, "__ge__": {1, 1},
	"__repr__": {0, 0}, "__str__": {0, 0}, "__format__": {1, 1}, "__hash__": {0, 0}, "__bool__": {0, 0},
	"__len__": {0, 0}, "__next__": {0, 0}, "__contains__": {1, 1}, "__iter__": {0, 0}, "__reversed__": {0, 0},
	"__getitem__": {1, 1}, "__setitem__": {2, 2}, "__delitem__": {1, 1},
	"__add__": {1, 1}, "__radd__": {1, 1}, "__sub__": {1, 1}, "__rsub__": {1, 1},
	"__mul__": {1, 1}, "__rmul__": {1, 1}, "__truediv__": {1, 1}, "__rtruediv__": {1, 1},
	"__floordiv__": {1, 1}, "__rfloordiv__": {1, 1}, "__mod__": {1, 1}, "__rmod__": {1, 1},
	"__pow__": {1, 1}, "__rpow__": {1, 1}, "__divmod__": {1, 1}, "__rdivmod__": {1, 1},
	"__and__": {1, 1}, "__rand__": {1, 1}, "__or__": {1, 1}, "__ror__": {1, 1}, "__xor__": {1, 1}, "__rxor__": {1, 1},
	"__lshift__": {1, 1}, "__rlshift__": {1, 1}, "__rshift__": {1, 1}, "__rrshift__": {1, 1},
	"__iadd__": {1, 1}, "__imul__": {1, 1}, "__isub__": {1, 1}, "__iand__": {1, 1}, "__ior__": {1, 1}, "__ixor__": {1, 1},
	"__neg__": {0, 0}, "__pos__": {0, 0}, "__abs__": {0, 0}, "__invert__": {0, 0},
	"__int__": {0, 0}, "__float__": {0, 0}, "__index__": {0, 0}, "__round__": {0, 1},
	"__trunc__": {0, 0}, "__floor__": {0, 0}, "__ceil__": {0, 0},
}

var dunderBinaryOps = map[string]ast.Op{
	"__add__": ast.OpAdd, "__sub__": ast.OpSub, "__mul__": ast.OpMul, "__truediv__": ast.OpDiv,
	"__floordiv__": ast.OpFloorDiv, "__mod__": ast.OpMod, "__pow__": ast.OpPow,
	"__and__": ast.OpBitAnd, "__or__": ast.OpBitOr, "__xor__": ast.OpBitXor,
	"__lshift__": ast.OpLShift, "__rshift__": ast.OpRShift,
	"__lt__": ast.OpLt, "__le__": ast.OpLte, "__gt__": ast.OpGt, "__ge__": ast.OpGte,
}

var dunderReflectedOps = map[string]ast.Op{
	"__radd__": ast.OpAdd, "__rsub__": ast.OpSub, "__rmul__": ast.OpMul, "__rtruediv__": ast.OpDiv,
	"__rfloordiv__": ast.OpFloorDiv, "__rmod__": ast.OpMod, "__rpow__": ast.OpPow,
	"__rand__": ast.OpBitAnd, "__ror__": ast.OpBitOr, "__rxor__": ast.OpBitXor,
	"__rlshift__": ast.OpLShift, "__rrshift__": ast.OpRShift,
}

// builtinByName reads the builtins map; set in init, as the map's own
// initialiser reaches this file through the method dispatchers.
var builtinByName func(name string) *object.Builtin

func init() {
	builtinByName = func(name string) *object.Builtin { return builtins[name] }
}

type dunderKind int

const (
	kindOther dunderKind = iota
	kindInt
	kindFloat
	kindStr
	kindBytes
	kindList
	kindTuple
	kindDict
	kindSet
	kindIter
)

func dunderKindOf(obj object.Object) dunderKind {
	switch o := obj.(type) {
	case *object.Integer, *object.Boolean:
		return kindInt
	case *object.Float:
		return kindFloat
	case *object.String:
		return kindStr
	case *object.Bytes:
		return kindBytes
	case *object.List:
		return kindList
	case *object.Tuple:
		return kindTuple
	case *object.Dict:
		if o.Module != "" {
			return kindOther
		}
		return kindDict
	case *object.Set:
		return kindSet
	case *object.Iterator:
		return kindIter
	}
	return kindOther
}

// dunderApplies reports whether a value of kind k has the dunder name.
func dunderApplies(k dunderKind, name string) bool {
	num := k == kindInt || k == kindFloat
	seq := k == kindStr || k == kindBytes || k == kindList || k == kindTuple
	switch name {
	case "__eq__", "__ne__", "__repr__", "__str__", "__format__":
		return true
	case "__hash__":
		return k != kindList && k != kindDict && !(k == kindSet)
	case "__lt__", "__le__", "__gt__", "__ge__":
		return num || seq || k == kindSet
	case "__bool__", "__radd__", "__rsub__", "__truediv__", "__rtruediv__", "__floordiv__", "__rfloordiv__",
		"__rmod__", "__pow__", "__rpow__", "__divmod__", "__rdivmod__", "__neg__", "__pos__", "__abs__",
		"__int__", "__float__", "__round__", "__trunc__", "__floor__", "__ceil__":
		return num
	case "__iter__":
		return seq || k == kindDict || k == kindSet || k == kindIter
	case "__next__":
		return k == kindIter
	case "__len__", "__contains__":
		return seq || k == kindDict || k == kindSet
	case "__getitem__":
		return seq || k == kindDict
	case "__setitem__", "__delitem__", "__reversed__":
		return k == kindList || k == kindDict
	case "__add__", "__mul__", "__rmul__":
		return num || seq
	case "__mod__":
		return num || k == kindStr || k == kindBytes
	case "__sub__":
		return num || k == kindSet
	case "__and__", "__rand__", "__xor__", "__rxor__":
		return k == kindInt || k == kindSet
	case "__or__", "__ror__":
		return k == kindInt || k == kindSet || k == kindDict
	case "__lshift__", "__rlshift__", "__rshift__", "__rrshift__", "__invert__", "__index__":
		return k == kindInt
	case "__iadd__", "__imul__":
		return k == kindList
	case "__isub__", "__iand__", "__ixor__":
		return k == kindSet
	case "__ior__":
		return k == kindSet || k == kindDict
	}
	return false
}

// builtinDunderMethod returns obj.name bound to obj when obj is a built-in
// value with that dunder, else nil.
func builtinDunderMethod(obj object.Object, name string) object.Object {
	if _, ok := dunderArity[name]; !ok || !dunderApplies(dunderKindOf(obj), name) {
		return nil
	}
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return callBuiltinDunder(ctx, obj, name, args, kwargs)
		},
		HelpText: fmt.Sprintf("%s.%s - bound method", getTypeName(obj), name),
		Repr:     fmt.Sprintf("<method-wrapper '%s' of %s object>", name, getTypeName(obj)),
	}
}

// builtinDunderCall calls obj.name(*args) for a built-in value's dunder;
// ok is false when obj has no such dunder.
func builtinDunderCall(ctx context.Context, obj object.Object, name string, args []object.Object, kwargs object.Kwargs) (object.Object, bool) {
	if _, ok := dunderArity[name]; !ok || !dunderApplies(dunderKindOf(obj), name) {
		return nil, false
	}
	return callBuiltinDunder(ctx, obj, name, args, kwargs), true
}

// typeDunderMethod is the unbound form of a type's dunder: int.__add__(1, 2),
// list.__iadd__(xs, ys), object.__repr__(x). nil when the type has none.
func typeDunderMethod(typ *object.Builtin, name string) object.Object {
	typeName, ok := typeBuiltins[typ]
	if !ok {
		return nil
	}
	if typeName == "object" {
		return objectDunder(name)
	}
	if _, ok := dunderArity[name]; !ok {
		return nil
	}
	kind := map[string]dunderKind{
		"int": kindInt, "bool": kindInt, "float": kindFloat, "str": kindStr, "bytes": kindBytes,
		"list": kindList, "tuple": kindTuple, "dict": kindDict, "set": kindSet, "frozenset": kindSet,
	}[typeName]
	if kind == kindOther || !dunderApplies(kind, name) {
		return nil
	}
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 || dunderKindOf(args[0]) != kind {
				got := "nothing"
				if len(args) > 0 {
					got = "a '" + getTypeName(args[0]) + "' object"
				}
				return &object.Exception{
					Message:       fmt.Sprintf("descriptor '%s' requires a '%s' object but received %s", name, typeName, got),
					ExceptionType: object.ExceptionTypeTypeError,
					Raised:        true,
				}
			}
			return callBuiltinDunder(ctx, args[0], name, args[1:], kwargs)
		},
		HelpText: fmt.Sprintf("%s.%s(self, ...)", typeName, name),
		Repr:     fmt.Sprintf("<slot wrapper '%s' of '%s' objects>", name, typeName),
	}
}

// objectDunder is object.__repr__ and friends: the defaults every value
// inherits, whatever the value's own type overrides.
func objectDunder(name string) object.Object {
	switch name {
	case "__repr__", "__str__", "__eq__", "__ne__", "__hash__", "__format__":
	case "__getattribute__":
		return &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs(name, kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				if _, ok := args[1].(*object.String); !ok {
					return errors.NewTypeErrorTagged("attribute name must be string, not '%s'", getTypeName(args[1]))
				}
				return getAttribute(ctx, args[0], args[1].(*object.String).StringValue())
			},
			HelpText: "object.__getattribute__(obj, name) - Read obj.name",
			Repr:     "<slot wrapper '__getattribute__' of 'object' objects>",
		}
	default:
		return nil
	}
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := dunderArgCheck(name, args, kwargs, 1); err != nil {
				return err
			}
			self, rest := args[0], args[1:]
			switch name {
			case "__repr__":
				if inst, ok := self.(*object.Instance); ok {
					return object.NewString(inst.Inspect())
				}
				return object.NewString(fmt.Sprintf("<%s object at %p>", getTypeName(self), self))
			case "__str__":
				return builtinByName("repr").Fn(ctx, object.Kwargs{}, self)
			case "__eq__":
				if self == rest[0] {
					return TRUE
				}
				return notImplementedValue
			case "__ne__":
				if self == rest[0] {
					return FALSE
				}
				return notImplementedValue
			case "__hash__":
				// The identity hash, as Python's id(x) >> 4.
				id, ok := builtinByName("id").Fn(ctx, object.Kwargs{}, self).(*object.Integer)
				if !ok {
					return object.NewInteger(0)
				}
				return object.NewInteger(id.IntValue() >> 4)
			case "__format__":
				if s, ok := rest[0].(*object.String); !ok || s.StringValue() != "" {
					return &object.Exception{
						Message:       fmt.Sprintf("unsupported format string passed to %s.__format__", getTypeName(self)),
						ExceptionType: object.ExceptionTypeTypeError,
						Raised:        true,
					}
				}
				return builtinByName("str").Fn(ctx, object.Kwargs{}, self)
			}
			return NULL
		},
		HelpText: fmt.Sprintf("object.%s(self, ...)", name),
		Repr:     fmt.Sprintf("<slot wrapper '%s' of 'object' objects>", name),
	}
}

// dunderArgCheck checks a dunder's argument count (n is the number of
// arguments it takes, self included when called unbound).
func dunderArgCheck(name string, args []object.Object, kwargs object.Kwargs, n int) object.Object {
	if kwargs.Len() > 0 {
		return &object.Exception{
			Message:       fmt.Sprintf("%s() takes no keyword arguments", name),
			ExceptionType: object.ExceptionTypeTypeError,
			Raised:        true,
		}
	}
	a := dunderArity[name]
	if len(args) < n+a[0] || len(args) > n+a[1] {
		want := fmt.Sprintf("%d", a[0])
		if a[0] != a[1] {
			want = fmt.Sprintf("from %d to %d", a[0], a[1])
		}
		return &object.Exception{
			Message:       fmt.Sprintf("expected %s arguments, got %d", want, len(args)-n),
			ExceptionType: object.ExceptionTypeTypeError,
			Raised:        true,
		}
	}
	return nil
}

// dunderComparable reports whether self's own __eq__ handles other (else
// Python returns NotImplemented): the same kind of value, or a number
// against an int.
func dunderComparable(self, other object.Object) bool {
	ks, ko := dunderKindOf(self), dunderKindOf(other)
	if ks == kindOther {
		return false
	}
	if ks == ko {
		return true
	}
	return ks == kindFloat && ko == kindInt
}

// notImplementedOnTypeError turns an unsupported-operand TypeError into
// NotImplemented, which is what Python's dunders return.
func notImplementedOnTypeError(r object.Object) object.Object {
	switch e := r.(type) {
	case *object.Exception:
		if e.ExceptionType == object.ExceptionTypeTypeError {
			return notImplementedValue
		}
	case *object.Error:
		if e.ExceptionType == object.ExceptionTypeTypeError {
			return notImplementedValue
		}
	}
	return r
}

func callBuiltinDunder(ctx context.Context, self object.Object, name string, args []object.Object, kwargs object.Kwargs) object.Object {
	if err := dunderArgCheck(name, args, kwargs, 0); err != nil {
		return err
	}
	env := GetEnvFromContext(ctx)
	call := func(fn string, a ...object.Object) object.Object {
		return builtinByName(fn).Fn(ctx, object.Kwargs{}, a...)
	}
	if op, ok := dunderBinaryOps[name]; ok {
		// Ordering is only defined against the same kind of value (an int
		// leaves an int-float comparison to the float).
		if op >= ast.OpLt && op <= ast.OpGte && !dunderComparable(self, args[0]) {
			return notImplementedValue
		}
		return notImplementedOnTypeError(evalInfixExpression(ctx, op, self, args[0], env))
	}
	if op, ok := dunderReflectedOps[name]; ok {
		return notImplementedOnTypeError(evalInfixExpression(ctx, op, args[0], self, env))
	}
	switch name {
	case "__eq__", "__ne__":
		if !dunderComparable(self, args[0]) {
			if self == args[0] {
				return object.NewBoolean(name == "__eq__")
			}
			return notImplementedValue
		}
		op := ast.OpEq
		if name == "__ne__" {
			op = ast.OpNeq
		}
		return evalInfixExpression(ctx, op, self, args[0], env)
	case "__iter__":
		if _, ok := self.(*object.Iterator); ok {
			return self
		}
		return call("iter", self)
	case "__repr__", "__str__", "__hash__", "__len__", "__next__", "__reversed__", "__abs__", "__int__", "__float__", "__bool__":
		return call(name[2:len(name)-2], self)
	case "__format__":
		return call("format", self, args[0])
	case "__round__":
		return call("round", append([]object.Object{self}, args...)...)
	case "__divmod__":
		return notImplementedOnTypeError(call("divmod", self, args[0]))
	case "__rdivmod__":
		return notImplementedOnTypeError(call("divmod", args[0], self))
	case "__contains__":
		return evalInfixExpression(ctx, ast.OpIn, args[0], self, env)
	case "__getitem__":
		return evalIndexExpression(ctx, self, args[0], false)
	case "__setitem__":
		if err := assignIndexValue(ctx, false, self, args[0], args[1]); err != nil {
			return assignErrorToObject(err)
		}
		return NULL
	case "__delitem__":
		return evalAdapter{}.DelItem(ctx, self, args[0])
	case "__neg__":
		return evalPrefixExpression(ctx, ast.OpSub, self, env)
	case "__invert__":
		return evalPrefixExpression(ctx, ast.OpBitNot, self, env)
	case "__pos__":
		return evalAdapter{}.UnaryOp(ctx, "+", self)
	case "__index__":
		return call("int", self)
	case "__trunc__", "__floor__", "__ceil__":
		f, ok := self.(*object.Float)
		if !ok {
			return call("int", self)
		}
		v := f.FloatValue()
		switch name {
		case "__floor__":
			v = math.Floor(v)
		case "__ceil__":
			v = math.Ceil(v)
		}
		return call("int", object.NewFloat(v))
	case "__iadd__":
		if r := callStringMethodWithKeywords(ctx, self, "extend", args, object.Kwargs{}, env); propagates(r) {
			return r
		}
		return self
	case "__imul__":
		r := evalInfixExpression(ctx, ast.OpMul, self, args[0], env)
		if l, ok := r.(*object.List); ok {
			self.(*object.List).Elements = l.Elements
			return self
		}
		return notImplementedOnTypeError(r)
	case "__isub__", "__iand__", "__ixor__", "__ior__":
		method := map[string]string{"__isub__": "difference_update", "__iand__": "intersection_update", "__ixor__": "symmetric_difference_update", "__ior__": "update"}[name]
		if r := callStringMethodWithKeywords(ctx, self, method, args, object.Kwargs{}, env); propagates(r) {
			return r
		}
		return self
	}
	return attributeError(self, name)
}

// missingBuiltinMethod is the method dispatchers' miss: a dunder the value
// answers to through its operator or builtin, else AttributeError.
func missingBuiltinMethod(ctx context.Context, obj object.Object, method string, args []object.Object, kwargs object.Kwargs) object.Object {
	if r, ok := builtinDunderCall(ctx, obj, method, args, kwargs); ok {
		return r
	}
	return attributeError(obj, method)
}
