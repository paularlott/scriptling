package object

// Iterator type names: what type() reports for an iterator ("range", "map").
// An iterator that does not set its own reports "iterator".
var (
	IteratorTypeName  = NewString("iterator")
	rangeTypeName     = NewString("range")
	zipTypeName       = NewString("zip")
	enumerateTypeName = NewString("enumerate")
)

// TypeName returns the iterator's type name.
func (it *Iterator) TypeName() *String {
	if it.typeName != nil {
		return it.typeName
	}
	return IteratorTypeName
}

// SetTypeName sets the iterator's type name, Python's name for it ("map",
// "list_iterator"), and returns the iterator. Share one string per name
// rather than allocating one per iterator.
func (it *Iterator) SetTypeName(name *String) *Iterator {
	it.typeName = name
	return it
}

// PythonTypeName is the name Python gives obj's type, for messages such as
// "unhashable type: 'deque'": int, str, NoneType, the class name of an
// instance, the exception type of an exception, an iterator's own name.
func PythonTypeName(obj Object) string {
	switch obj.Type() {
	case INTEGER_OBJ:
		return "int"
	case FLOAT_OBJ:
		return "float"
	case STRING_OBJ:
		return "str"
	case BOOLEAN_OBJ:
		return "bool"
	case LIST_OBJ:
		return "list"
	case DICT_OBJ:
		if obj.(*Dict).Module != "" {
			return "module"
		}
		return "dict"
	case TUPLE_OBJ:
		return "tuple"
	case SET_OBJ:
		if obj.(*Set).Frozen {
			return "frozenset"
		}
		return "set"
	case NULL_OBJ:
		return "NoneType"
	case BYTES_OBJ:
		return "bytes"
	case DICT_KEYS_OBJ:
		return "dict_keys"
	case DICT_VALUES_OBJ:
		return "dict_values"
	case DICT_ITEMS_OBJ:
		return "dict_items"
	case FUNCTION_OBJ, LAMBDA_OBJ:
		return "function"
	case BUILTIN_OBJ:
		return "builtin_function_or_method"
	case CLASS_OBJ:
		return "type"
	case INSTANCE_OBJ:
		return obj.(*Instance).Class.Name
	case EXCEPTION_OBJ:
		if exc, ok := obj.(*Exception); ok && exc.ExceptionType != "" {
			return exc.ExceptionType
		}
		return "Exception"
	case SUPER_OBJ:
		return "super"
	case ITERATOR_OBJ:
		if it, ok := obj.(*Iterator); ok {
			return it.TypeName().StringValue()
		}
		return "iterator"
	case SLICE_OBJ:
		return "slice"
	case PROPERTY_OBJ:
		return "property"
	case STATICMETHOD_OBJ:
		return "staticmethod"
	case CLASSMETHOD_OBJ:
		return "classmethod"
	case FLOAT_ARRAY_OBJ:
		return "FloatArray"
	case SENTINEL_OBJ:
		return "sentinel"
	default:
		return obj.Type().String()
	}
}
