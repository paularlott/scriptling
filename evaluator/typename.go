package evaluator

import "github.com/paularlott/scriptling/object"

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
		if _, ok := exceptionBuiltins[o]; ok {
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
