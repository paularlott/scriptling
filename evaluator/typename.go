package evaluator

import (
	"context"
	"sync"

	"github.com/paularlott/scriptling/errors"

	"github.com/paularlott/scriptling/object"
)

// typeObjectOf is type(obj): the type as a value, as in Python. An
// instance gives its class, int gives int, and a value whose type has no
// builtin name (None, a function, an iterator) gives a type object that
// prints as <class 'NoneType'>, has __name__, and works with is, == and
// isinstance.
func typeObjectOf(obj object.Object) object.Object {
	switch o := obj.(type) {
	case *object.Integer:
		return typeObjInt
	case *object.String:
		return typeObjStr
	case *object.Float:
		return typeObjFloat
	case *object.Boolean:
		return typeObjBool
	case *object.Null:
		return typeObjNone
	case *object.List:
		return typeObjList
	case *object.Tuple:
		return typeObjTuple
	case *object.Instance:
		if b, ok := object.BuiltinTypeFor(o.Class); ok {
			return b
		}
		return o.Class
	case *object.Exception:
		return exceptionTypeObject(o)
	case *object.Class:
		return typeTypeBuiltin
	case *object.Builtin:
		if o == typeTypeBuiltin || isClassLike(o) {
			return typeTypeBuiltin
		}
	case *object.Dict:
		if o.Module == "" {
			if o.Factory() != nil && object.DefaultDictType != nil {
				return object.DefaultDictType
			}
			if o.IsOrderedDict() && object.OrderedDictType != nil {
				return object.OrderedDictType
			}
		}
	}
	t := typeOf(obj)
	if s, ok := t.(*object.String); ok {
		return typeObjectNamed(s.StringValue())
	}
	return typeObjectNamed(t.Inspect())
}

// The types of the common values, set once the builtins exist.
var typeObjInt, typeObjStr, typeObjFloat, typeObjBool, typeObjNone, typeObjList, typeObjTuple object.Object

func initTypeObjects() {
	typeObjInt = typeObjectNamed("int")
	typeObjStr = typeObjectNamed("str")
	typeObjFloat = typeObjectNamed("float")
	typeObjBool = typeObjectNamed("bool")
	typeObjNone = typeObjectNamed("NoneType")
	typeObjList = typeObjectNamed("list")
	typeObjTuple = typeObjectNamed("tuple")
}

// syntheticTypes holds the type objects for names with no builtin, made on
// first use so type(None) is type(None).
var syntheticTypes sync.Map // name -> *object.Builtin

// typeObjectNamed is the type object for a type name: the builtin for int,
// str and the other builtin types, else a synthetic type.
func typeObjectNamed(name string) object.Object {
	if b, ok := builtins[name]; ok {
		if _, isType := typeBuiltins[b]; isType {
			return b
		}
		if excName, isExc := exceptionClassName(b); isExc && excName == name {
			return b
		}
	}
	if t, ok := syntheticTypes.Load(name); ok {
		return t.(*object.Builtin)
	}
	b := &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if name == "NoneType" && len(args) == 0 && kwargs.Len() == 0 {
				return NULL // type(None)() is None
			}
			return errors.NewTypeErrorTagged("cannot create '%s' instances", name)
		},
		Repr:       "<class '" + name + "'>",
		Attributes: map[string]object.Object{"__name__": object.NewString(name), "__qualname__": object.NewString(name)},
	}
	t, _ := syntheticTypes.LoadOrStore(name, b)
	return t.(*object.Builtin)
}

// syntheticTypeName is the name of a type object typeObjectNamed made, ""
// for any other value.
func syntheticTypeName(b *object.Builtin) string {
	n, ok := b.Attributes["__name__"].(*object.String)
	if !ok {
		return ""
	}
	if t, ok := syntheticTypes.Load(n.StringValue()); ok && t == b {
		return n.StringValue()
	}
	return ""
}

// Type names type() returns, shared so type() does not allocate.
var (
	typeNameInt         = object.NewString("int")
	typeNameFloat       = object.NewString("float")
	typeNameStr         = object.NewString("str")
	typeNameBool        = object.NewString("bool")
	typeNameNone        = object.NewString("NoneType")
	typeNameBytes       = object.NewString("bytes")
	typeNameList        = object.NewString("list")
	typeNameTuple       = object.NewString("tuple")
	typeNameDict        = object.NewString("dict")
	typeNameDefaultDict = object.NewString("defaultdict")
	typeNameOrderedDict = object.NewString("OrderedDict")
	typeNameModule      = object.NewString("module")
	typeNameSet         = object.NewString("set")
	typeNameFrozenSet   = object.NewString("frozenset")
	typeNameSlice       = object.NewString("slice")
	typeNameDictKeys    = object.NewString("dict_keys")
	typeNameDictValues  = object.NewString("dict_values")
	typeNameDictItems   = object.NewString("dict_items")
	typeNameFunction    = object.NewString("function")
	typeNameMethod      = object.NewString("method")
	typeNameBuiltin     = object.NewString("builtin_function_or_method")
	typeNameType        = object.NewString("type")
	typeNameException   = object.NewString("Exception")
	typeNameSuper       = object.NewString("super")
	typeNameProperty    = object.NewString("property")
	typeNameStatic      = object.NewString("staticmethod")
	typeNameClassMethod = object.NewString("classmethod")
	typeNameFloatArray  = object.NewString("FloatArray")
	typeNameSentinel    = object.NewString("sentinel")

	// Iterators from iter(x) and reversed(x), named by what x is, as CPython
	// names them.
	iterTypeNameList       = object.NewString("list_iterator")
	iterTypeNameTuple      = object.NewString("tuple_iterator")
	iterTypeNameStr        = object.NewString("str_ascii_iterator")
	iterTypeNameBytes      = object.NewString("bytes_iterator")
	iterTypeNameSet        = object.NewString("set_iterator")
	iterTypeNameDictKeys   = object.NewString("dict_keyiterator")
	iterTypeNameDictValues = object.NewString("dict_valueiterator")
	iterTypeNameDictItems  = object.NewString("dict_itemiterator")
	iterTypeNameRange      = object.NewString("range_iterator")
	iterTypeNameMap        = object.NewString("map")
	iterTypeNameFilter     = object.NewString("filter")
	iterTypeNameReversed   = object.NewString("reversed")
	iterTypeNameListRev    = object.NewString("list_reverseiterator")
	iterTypeNameDictRev    = object.NewString("dict_reversekeyiterator")
	iterTypeNameCallable   = object.NewString("callable_iterator")
	typeNameEllipsis       = object.NewString("ellipsis")
)

// typeOf is type(obj): the type's Python name.
func typeOf(obj object.Object) object.Object {
	switch o := obj.(type) {
	case *object.Instance:
		return object.NewString(o.Class.Name)
	case *object.Exception:
		// An exception reports the class it was raised as ("ValueError"):
		// except blocks need to tell them apart without message sniffing.
		if o.ExceptionType != "" {
			return object.NewString(o.ExceptionType)
		}
		return typeNameException
	case *object.Integer:
		return typeNameInt
	case *object.Float:
		return typeNameFloat
	case *object.String:
		return typeNameStr
	case *object.Boolean:
		return typeNameBool
	case *object.Null:
		return typeNameNone
	case *object.Bytes:
		return typeNameBytes
	case *object.List:
		return typeNameList
	case *object.Tuple:
		return typeNameTuple
	case *object.Dict:
		if o.Module != "" {
			return typeNameModule
		}
		if o.Factory() != nil {
			return typeNameDefaultDict
		}
		if o.IsOrderedDict() {
			return typeNameOrderedDict
		}
		return typeNameDict
	case *object.Set:
		if o.Frozen {
			return typeNameFrozenSet
		}
		return typeNameSet
	case *object.Iterator:
		return o.TypeName()
	case *object.Function:
		return typeNameFunction
	case *object.LambdaFunction:
		return typeNameFunction
	case *object.BoundMethod:
		return typeNameMethod
	case *object.Builtin:
		// int, str, ValueError, ... are types: type(int) is type.
		if _, ok := typeBuiltins[o]; ok {
			return typeNameType
		}
		if _, ok := exceptionClassName(o); ok {
			return typeNameType
		}
		return typeNameBuiltin
	case *object.Library:
		return typeNameModule
	case *object.Class:
		return typeNameType
	}
	switch obj.Type() {
	case object.SLICE_OBJ:
		return typeNameSlice
	case object.DICT_KEYS_OBJ:
		return typeNameDictKeys
	case object.DICT_VALUES_OBJ:
		return typeNameDictValues
	case object.DICT_ITEMS_OBJ:
		return typeNameDictItems
	case object.SUPER_OBJ:
		return typeNameSuper
	case object.PROPERTY_OBJ:
		return typeNameProperty
	case object.STATICMETHOD_OBJ:
		return typeNameStatic
	case object.CLASSMETHOD_OBJ:
		return typeNameClassMethod
	case object.FLOAT_ARRAY_OBJ:
		return typeNameFloatArray
	case object.SENTINEL_OBJ:
		if obj == ellipsisValue {
			return typeNameEllipsis
		}
		if obj == notImplementedValue {
			return object.NewString("NotImplementedType")
		}
		return typeNameSentinel
	}
	return object.NewString(obj.Type().String())
}

// iterTypeNameOf is the type name of iter(src) for a built-in iterable src,
// nil when src is not one of them.
func iterTypeNameOf(src object.Object) *object.String {
	switch o := src.(type) {
	case *object.List:
		return iterTypeNameList
	case *object.Tuple:
		return iterTypeNameTuple
	case *object.String:
		return iterTypeNameStr
	case *object.Bytes:
		return iterTypeNameBytes
	case *object.Set:
		return iterTypeNameSet
	case *object.Dict:
		if o.Module == "" {
			return iterTypeNameDictKeys
		}
	case *object.Iterator:
		if _, _, _, ok := o.RangeRemaining(); ok {
			return iterTypeNameRange
		}
	}
	switch src.Type() {
	case object.DICT_KEYS_OBJ:
		return iterTypeNameDictKeys
	case object.DICT_VALUES_OBJ:
		return iterTypeNameDictValues
	case object.DICT_ITEMS_OBJ:
		return iterTypeNameDictItems
	}
	return nil
}

// reversedTypeNameOf is the type name of reversed(src).
func reversedTypeNameOf(src object.Object) *object.String {
	switch o := src.(type) {
	case *object.List:
		return iterTypeNameListRev
	case *object.Dict:
		return iterTypeNameDictRev
	case *object.Iterator:
		if _, _, _, ok := o.RangeRemaining(); ok {
			return iterTypeNameRange
		}
	}
	return iterTypeNameReversed
}

// withIterTypeName names r, when it is an iterator, name.
func withIterTypeName(r object.Object, name *object.String) object.Object {
	if it, ok := r.(*object.Iterator); ok {
		it.SetTypeName(name)
	}
	return r
}

// isinstanceOfTypeName is isinstance(obj, name) for a type name string: obj's
// type is name or, as in Python, a subtype of it (bool of int, defaultdict of
// dict, a class of its bases).
func isinstanceOfTypeName(obj object.Object, name string) bool {
	switch o := obj.(type) {
	case *object.Instance:
		for c := o.Class; c != nil; c = c.BaseClass {
			if c.Name == name {
				return true
			}
		}
		// An IntEnum or StrEnum member is an int or a str.
		if name == "int" || name == "str" {
			if v, ok := valueMember(o); ok {
				return isinstanceOfTypeName(v, name)
			}
		}
		return false
	case *object.Boolean:
		return name == "bool" || name == "int"
	case *object.Dict:
		if name == "dict" && o.Module == "" {
			return true
		}
	}
	if t, ok := typeOf(obj).(*object.String); ok {
		return t.StringValue() == name
	}
	return false
}
