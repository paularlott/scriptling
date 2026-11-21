package evaluator

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// anyKeyword marks a method that accepts arbitrary keyword arguments.
const anyKeyword = "*"

// methodKeywords lists the keyword arguments each built-in type method reads.
// A method in builtinMethodNames that is not listed here takes no keywords.
var methodKeywords = map[object.ObjectType]map[string][]string{
	object.STRING_OBJ: {
		"split":      {"sep", "maxsplit"},
		"rsplit":     {"sep", "maxsplit"},
		"splitlines": {"keepends"},
		"encode":     {"encoding", "errors"},
		"expandtabs": {"tabsize"},
		"replace":    {"count"},
		"format":     {anyKeyword}, // "{name}".format(name=...)
	},
	object.BYTES_OBJ: {
		"split":  {"sep", "maxsplit"},
		"rsplit": {"sep", "maxsplit"},
		"decode": {"encoding", "errors"},
	},
	object.LIST_OBJ: {
		"sort": {"key", "reverse"},
	},
	object.DICT_OBJ: {
		"update":      {anyKeyword}, // d.update(a=1) stores the keywords
		"popitem":     {"last"},
		"move_to_end": {"last"},
	},
	object.INTEGER_OBJ: {
		"to_bytes":   {"length", "byteorder", "signed"},
		"from_bytes": {"bytes", "byteorder", "signed"},
	},
	object.BOOLEAN_OBJ: {
		"to_bytes": {"length", "byteorder", "signed"},
	},
}

// checkMethodKeywords returns a TypeError when a call to the built-in method
// passes a keyword the method does not read. Callers test keywords.Len()
// first, so a call without keywords does no work here. A name that is not a
// method of the type is left for the dispatcher's AttributeError.
func checkMethodKeywords(obj object.Object, method string, keywords object.Kwargs) object.Object {
	t := obj.Type()
	allowed := methodKeywords[t][method]
	if len(allowed) == 0 {
		if !hasBuiltinMethod(t, method) {
			return nil
		}
		// Python names the type here: dict.get() takes no keyword arguments.
		return errors.NoKwargs(getTypeName(obj)+"."+method, keywords)
	}
	if len(allowed) == 1 && allowed[0] == anyKeyword {
		return nil
	}
	return errors.CheckKwargs(method, keywords, allowed...)
}

// foldKeywords binds keyword arguments to the positional parameters named in
// params, returning the positional arguments in order. A keyword naming a
// parameter already given positionally is a TypeError; keywords not in params
// are ignored (the caller has validated them). A parameter left unfilled
// before one that is filled takes its default from defaults (nil: required).
func foldKeywords(fn string, args []object.Object, keywords object.Kwargs, params []string, defaults []object.Object) ([]object.Object, object.Object) {
	last := len(args) - 1
	for i, name := range params {
		if keywords.Has(name) {
			if i < len(args) {
				return nil, errors.NewTypeErrorTagged("%s() got multiple values for argument '%s'", fn, name)
			}
			last = max(last, i)
		}
	}
	if last < len(args) {
		return args, nil
	}
	out := make([]object.Object, last+1)
	copy(out, args)
	for i := len(args); i <= last; i++ {
		if v, ok := keywords.Lookup(params[i]); ok {
			out[i] = v
		} else if i < len(defaults) && defaults[i] != nil {
			out[i] = defaults[i]
		} else {
			return nil, errors.NewTypeErrorTagged("%s() missing required argument '%s'", fn, params[i])
		}
	}
	return out, nil
}

// callBytesKeywordDecode is str(obj, encoding=..., errors=...): it decodes a
// bytes value with the bytes decode method, and is a TypeError for anything
// else, as in Python.
func callBytesKeywordDecode(ctx context.Context, obj object.Object, keywords object.Kwargs) object.Object {
	b, ok := obj.(*object.Bytes)
	if !ok {
		return errors.NewTypeErrorTagged("decoding to str: need a bytes-like object, %s found", getTypeName(obj))
	}
	return bytesDecodeFn(ctx, b, nil, keywords)
}

// bytesDecodeFn is b.decode(**keywords), set in init to break the
// initialization cycle between the builtins map and the method dispatchers.
var bytesDecodeFn func(ctx context.Context, b *object.Bytes, args []object.Object, keywords object.Kwargs) object.Object

func init() {
	bytesDecodeFn = func(ctx context.Context, b *object.Bytes, args []object.Object, keywords object.Kwargs) object.Object {
		return checkResult(callBytesMethod(ctx, b, "decode", args, keywords, GetEnvFromContext(ctx)))
	}
}
