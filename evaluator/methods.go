package evaluator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func dictCallableMethodExists(dict *object.Dict, method string) bool {
	pair, ok := dict.GetByString(method)
	if !ok {
		return false
	}

	switch pair.Value.(type) {
	case *object.Builtin, *object.Function, *object.LambdaFunction, *object.Class:
		return true
	default:
		return false
	}
}

func fastStringUpper(s string) string {
	// First pass: check for any non-ASCII byte. If present, defer to the
	// Unicode-aware implementation so characters like "é" are uppercased
	// correctly even when they appear after an ASCII lowercase letter.
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			// Go maps case 1:1, so ß stays ß; Python's full case mapping
			// uppercases it to SS.
			return strings.ReplaceAll(strings.ToUpper(s), "ß", "SS")
		}
	}
	// Pure ASCII fast path.
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'a' <= c && c <= 'z' {
			buf := make([]byte, len(s))
			copy(buf, s[:i])
			buf[i] = c - ('a' - 'A')
			for j := i + 1; j < len(s); j++ {
				c = s[j]
				if 'a' <= c && c <= 'z' {
					buf[j] = c - ('a' - 'A')
				} else {
					buf[j] = c
				}
			}
			return string(buf)
		}
	}
	return s
}

func fastStringLower(s string) string {
	// First pass: check for any non-ASCII byte. If present, defer to the
	// Unicode-aware implementation so characters like "É" are lowercased
	// correctly even when they appear after an ASCII uppercase letter.
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return strings.ToLower(s)
		}
	}
	// Pure ASCII fast path.
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			buf := make([]byte, len(s))
			copy(buf, s[:i])
			buf[i] = c + ('a' - 'A')
			for j := i + 1; j < len(s); j++ {
				c = s[j]
				if 'A' <= c && c <= 'Z' {
					buf[j] = c + ('a' - 'A')
				} else {
					buf[j] = c
				}
			}
			return string(buf)
		}
	}
	return s
}

func callStringMethodWithKeywords(ctx context.Context, obj object.Object, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Handle universal methods
	switch method {
	case "type":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(keywords) > 0 {
			return errors.NewError("type() does not accept keyword arguments")
		}
		return object.NewString(obj.Type().String())
	}

	// Handle library method calls (dictionaries)
	if obj.Type() == object.DICT_OBJ {
		return callDictMethod(ctx, obj.(*object.Dict), method, args, keywords, env)
	}

	// Handle integer methods
	if obj.Type() == object.INTEGER_OBJ {
		return callIntegerMethod(obj.(*object.Integer), method, args)
	}

	// Handle float methods
	if obj.Type() == object.FLOAT_OBJ {
		return callFloatMethod(obj.(*object.Float), method, args)
	}

	// Handle list methods
	if obj.Type() == object.LIST_OBJ {
		return callListMethod(ctx, obj.(*object.List), method, args, keywords, env)
	}

	// Handle set methods
	if obj.Type() == object.SET_OBJ {
		return callSetMethod(ctx, obj.(*object.Set), method, args, keywords, env)
	}

	// Handle tuple methods
	if obj.Type() == object.TUPLE_OBJ {
		return callTupleMethod(ctx, obj.(*object.Tuple), method, args, env)
	}

	// Handle FloatArray methods
	if obj.Type() == object.FLOAT_ARRAY_OBJ {
		return callFloatArrayMethod(obj.(*object.FloatArray), method, args)
	}

	// Handle Instance method calls
	if obj.Type() == object.INSTANCE_OBJ {
		return callInstanceMethod(ctx, obj.(*object.Instance), method, args, keywords, env)
	}

	// Handle Class method calls (e.g. Math.square(4) for staticmethods)
	if obj.Type() == object.CLASS_OBJ {
		cl := obj.(*object.Class)
		if fn, ok := cl.LookupMember(method); ok {
			if sm, ok := fn.(*object.StaticMethod); ok {
				return applyFunctionWithContext(ctx, sm.Fn, args, keywords, env)
			}
			if cm, ok := fn.(*object.ClassMethod); ok {
				newArgs := prependSelf(cl, args)
				return applyFunctionWithContext(ctx, cm.Fn, newArgs, keywords, env)
			}
			return applyFunctionWithContext(ctx, fn, args, keywords, env)
		}
		return attributeError(cl, method)
	}

	// Handle Super method calls
	if obj.Type() == object.SUPER_OBJ {
		return callSuperMethod(ctx, obj.(*object.Super), method, args, keywords, env)
	}

	// Handle Builtin with Attributes (like Promise objects from async library)
	if obj.Type() == object.BUILTIN_OBJ {
		if builtin, ok := obj.(*object.Builtin); ok && builtin.Attributes != nil {
			if attr, exists := builtin.Attributes[method]; exists {
				return applyFunctionWithContext(ctx, attr, args, keywords, env)
			}
		}
	}

	// Default to string methods if object is a string
	if obj.Type() == object.STRING_OBJ {
		return callStringMethod(ctx, obj.(*object.String), method, args, keywords, env)
	}

	// Bytes methods
	if obj.Type() == object.BYTES_OBJ {
		return callBytesMethod(ctx, obj.(*object.Bytes), method, args, keywords, env)
	}

	return attributeError(obj, method)
}

func callSuperMethod(ctx context.Context, super *object.Super, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	if super.Class.BaseClass != nil {
		if fn, ok := super.Class.BaseClass.LookupMember(method); ok {
			// Bind 'self' for all callable types (Function, Builtin, LambdaFunction, etc.)
			newArgs := prependSelf(super.Instance, args)
			return applyFunctionWithContext(ctx, fn, newArgs, keywords, env)
		}
	}

	return attributeError(super, method)
}

// prependSelf returns args with self inserted at the front.
//
// It allocates: the result is handed to applyFunction, which passes it on, so a
// local buffer would escape anyway — and sizing it to the argument count beats
// rounding every call up to a fixed buffer. Hot instance method calls avoid this
// entirely via tryEvalInstanceMethodFast.
func prependSelf(self object.Object, args []object.Object) []object.Object {
	newArgs := make([]object.Object, len(args)+1)
	newArgs[0] = self
	copy(newArgs[1:], args)
	return newArgs
}

func callInstanceMethod(ctx context.Context, instance *object.Instance, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// First check if it's an instance field (which might be a callable)
	if val, ok := instance.GetField(method); ok {
		// If it's callable, call it without prepending self
		switch fn := val.(type) {
		case *object.Function, *object.LambdaFunction, *object.Builtin, *object.BoundMethod:
			return applyFunctionWithContext(ctx, fn, args, keywords, env)
		}
		return notCallableError(val)
	}

	if fn, ok := instance.Class.LookupMember(method); ok {
		// StaticMethod: call without self
		if sm, ok := fn.(*object.StaticMethod); ok {
			return applyFunctionWithContext(ctx, sm.Fn, args, keywords, env)
		}
		// ClassMethod: call with class as first arg
		if cm, ok := fn.(*object.ClassMethod); ok {
			newArgs := prependSelf(instance.Class, args)
			return applyFunctionWithContext(ctx, cm.Fn, newArgs, keywords, env)
		}
		// Property: calling a property raises an error (not callable)
		if _, ok := fn.(*object.Property); ok {
			return errors.NewError("'property' object is not callable")
		}
		// Bind 'self'
		newArgs := prependSelf(instance, args)
		return applyFunctionWithContext(ctx, fn, newArgs, keywords, env)
	}

	// As in Python, __getattr__ supplies attributes normal lookup misses.
	if getattr, ok := instance.Class.LookupMember("__getattr__"); ok {
		attr := applyFunctionWithContext(ctx, getattr, []object.Object{instance, object.NewString(method)}, nil, env)
		if propagates(attr) {
			return attr
		}
		return applyFunctionWithContext(ctx, attr, args, keywords, env)
	}
	return attributeError(instance, method)
}

func callDictMethod(ctx context.Context, dict *object.Dict, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// First check for library methods (callable functions stored in dict)
	// This takes priority over dict instance methods like get, pop, etc.
	if pair, ok := dict.GetByString(method); ok {
		switch fn := pair.Value.(type) {
		case *object.Builtin:
			ctxWithEnv := SetEnvInContext(ctx, env)
			return fn.Fn(ctxWithEnv, object.NewKwargs(keywords), args...)
		case *object.Function:
			return applyFunctionWithContext(ctx, fn, args, keywords, env)
		case *object.LambdaFunction:
			return applyFunctionWithContext(ctx, fn, args, keywords, env)
		case *object.Class:
			return applyFunctionWithContext(ctx, fn, args, keywords, env)
		}
		// If it's not a callable, fall through to dict instance methods
		if dict.Module != "" {
			// Documented Scriptling convenience: a module constant called
			// with no arguments returns itself (os.environ()).
			if len(args) == 0 && len(keywords) == 0 {
				return pair.Value
			}
			return notCallableError(pair.Value)
		}
	}
	// A module has only its members: math.keys() is an AttributeError.
	if dict.Module != "" {
		return attributeError(dict, method)
	}

	// Check for dict instance methods
	switch method {
	case "keys":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(keywords) > 0 {
			return errors.NewError("keys() does not accept keyword arguments")
		}
		if builtin, ok := builtins["keys"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, object.NewKwargs(nil), dict)
		}
	case "values":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(keywords) > 0 {
			return errors.NewError("values() does not accept keyword arguments")
		}
		if builtin, ok := builtins["values"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, object.NewKwargs(nil), dict)
		}
	case "items":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(keywords) > 0 {
			return errors.NewError("items() does not accept keyword arguments")
		}
		if builtin, ok := builtins["items"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, object.NewKwargs(nil), dict)
		}
	case "get":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("get() takes 1-2 arguments (%d given)", len(args))
		}
		if len(keywords) > 0 {
			return errors.NewError("get() does not accept keyword arguments")
		}
		key, rerr := evalHashKeyChecked(ctx, args[0])
		if rerr != nil {
			return rerr
		}
		if pair, ok := dict.Pairs[key]; ok {
			return pair.Value
		}
		if len(args) == 2 {
			return args[1]
		}
		return NULL
	case "pop":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("pop() takes 1-2 arguments (%d given)", len(args))
		}
		if len(keywords) > 0 {
			return errors.NewError("pop() does not accept keyword arguments")
		}
		key, rerr := evalHashKeyChecked(ctx, args[0])
		if rerr != nil {
			return rerr
		}
		if pair, ok := dict.Pairs[key]; ok {
			delete(dict.Pairs, key)
			return pair.Value
		}
		if len(args) == 2 {
			return args[1]
		}
		return errors.NewError("key '%s' not found", key)
	case "update":
		if len(args) > 1 {
			return errors.NewError("update() takes at most 1 argument (%d given)", len(args))
		}
		// Handle kwargs
		for k, v := range keywords {
			dict.SetByString(k, v)
		}
		// Handle positional argument (another dict or list of pairs)
		if len(args) == 1 {
			switch other := args[0].(type) {
			case *object.Dict:
				for k, v := range other.Pairs {
					dict.Pairs[k] = v
				}
			case *object.List:
				for _, elem := range other.Elements {
					var pair []object.Object
					switch p := elem.(type) {
					case *object.List:
						pair = p.Elements
					case *object.Tuple:
						pair = p.Elements
					default:
						return errors.NewError("dictionary update sequence element must be [key, value] pair")
					}
					if len(pair) != 2 {
						return errors.NewError("dictionary update sequence element must be [key, value] pair")
					}
					hk, rerr := evalHashKeyChecked(ctx, pair[0])
					if rerr != nil {
						return rerr
					}
					dict.Pairs[hk] = object.DictPair{Key: pair[0], Value: pair[1]}
				}
			default:
				return errors.NewTypeError("DICT or LIST of pairs", args[0].Type().String())
			}
		}
		return NULL
	case "clear":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(keywords) > 0 {
			return errors.NewError("clear() does not accept keyword arguments")
		}
		dict.Pairs = make(map[string]object.DictPair)
		return NULL
	case "copy":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(keywords) > 0 {
			return errors.NewError("copy() does not accept keyword arguments")
		}
		newPairs := make(map[string]object.DictPair, len(dict.Pairs))
		for k, v := range dict.Pairs {
			newPairs[k] = v
		}
		return &object.Dict{Pairs: newPairs}
	case "setdefault":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("setdefault() takes 1-2 arguments (%d given)", len(args))
		}
		if len(keywords) > 0 {
			return errors.NewError("setdefault() does not accept keyword arguments")
		}
		key, rerr := evalHashKeyChecked(ctx, args[0])
		if rerr != nil {
			return rerr
		}
		if pair, ok := dict.Pairs[key]; ok {
			return pair.Value
		}
		var defaultVal object.Object = NULL
		if len(args) == 2 {
			defaultVal = args[1]
		}
		dict.Pairs[key] = object.DictPair{Key: args[0], Value: defaultVal}
		return defaultVal
	case "fromkeys":
		// dict.fromkeys(iterable[, value]) - create new dict with keys from iterable
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("fromkeys() takes 1-2 arguments (%d given)", len(args))
		}
		if len(keywords) > 0 {
			return errors.NewError("fromkeys() does not accept keyword arguments")
		}
		var defaultVal object.Object = NULL
		if len(args) == 2 {
			defaultVal = args[1]
		}
		newPairs := make(map[string]object.DictPair)
		switch iter := args[0].(type) {
		case *object.List:
			for _, elem := range iter.Elements {
				key, rerr := evalHashKeyChecked(ctx, elem)
				if rerr != nil {
					return rerr
				}
				newPairs[key] = object.DictPair{Key: elem, Value: defaultVal}
			}
		case *object.Tuple:
			for _, elem := range iter.Elements {
				key, rerr := evalHashKeyChecked(ctx, elem)
				if rerr != nil {
					return rerr
				}
				newPairs[key] = object.DictPair{Key: elem, Value: defaultVal}
			}
		case *object.String:
			for _, ch := range iter.StringValue() {
				s := string(ch)
				key := object.DictKey(object.NewString(s))
				newPairs[key] = object.DictPair{Key: object.NewString(s), Value: defaultVal}
			}
		default:
			return errors.NewTypeError("iterable (LIST, TUPLE, STRING)", args[0].Type().String())
		}
		return &object.Dict{Pairs: newPairs}
	}

	// Check for non-callable dict values (for accessing dict attributes)
	dictKey := object.DictKey(object.NewString(method))
	if pair, ok := dict.Pairs[dictKey]; ok {
		// If it's not a callable, just return the value
		if len(args) == 0 && len(keywords) == 0 {
			return pair.Value
		}
		return notCallableError(pair.Value)
	}
	return attributeError(dict, method)
}

func callListMethod(ctx context.Context, list *object.List, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch method {
	case "append":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		list.Elements = append(list.Elements, args[0])
		return NULL
	case "extend":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		elements, err := args[0].AsList()
		if err != nil {
			return errors.ParameterError("iterable", err)
		}
		list.Elements = append(list.Elements, elements...)
		return NULL
	case "index":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("index() takes 1-3 arguments (%d given)", len(args))
		}
		value := args[0]
		start := 0
		end := len(list.Elements)
		if len(args) >= 2 {
			s, errObj := args[1].AsInt()
			if errObj != nil {
				return errors.ParameterError("start", errObj)
			}
			start = int(s)
			if start < 0 {
				start = len(list.Elements) + start
				if start < 0 {
					start = 0
				}
			}
		}
		if len(args) == 3 {
			e, errObj := args[2].AsInt()
			if errObj != nil {
				return errors.ParameterError("end", errObj)
			}
			end = int(e)
			if end < 0 {
				end = len(list.Elements) + end
			}
		}
		if start > len(list.Elements) {
			start = len(list.Elements)
		}
		if end > len(list.Elements) {
			end = len(list.Elements)
		}
		for i := start; i < end; i++ {
			eq, rerr := evalObjectsEqualChecked(ctx, list.Elements[i], value, env)
			if rerr != nil {
				return rerr
			}
			if eq {
				return object.NewInteger(int64(i))
			}
		}
		return errors.NewError("value not in list")
	case "count":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		value := args[0]
		count := int64(0)
		for _, elem := range list.Elements {
			eq, rerr := evalObjectsEqualChecked(ctx, elem, value, env)
			if rerr != nil {
				return rerr
			}
			if eq {
				count++
			}
		}
		return object.NewInteger(count)
	case "pop":
		if len(args) > 1 {
			return errors.NewError("pop() takes at most 1 argument (%d given)", len(args))
		}
		if len(list.Elements) == 0 {
			return errors.NewError("pop from empty list")
		}
		idx := len(list.Elements) - 1
		if len(args) == 1 {
			i, errObj := args[0].AsInt()
			if errObj != nil {
				return errors.ParameterError("index", errObj)
			}
			idx = int(i)
			if idx < 0 {
				idx = len(list.Elements) + idx
			}
			if idx < 0 || idx >= len(list.Elements) {
				return errors.NewError("pop index out of range")
			}
		}
		result := list.Elements[idx]
		list.Elements = append(list.Elements[:idx], list.Elements[idx+1:]...)
		return result
	case "insert":
		if err := errors.ExactArgs(args, 2); err != nil {
			return err
		}
		idx, errObj := args[0].AsInt()
		if errObj != nil {
			return errors.ParameterError("index", errObj)
		}
		i := int(idx)
		if i < 0 {
			// Python behavior: negative index inserts at len + i (e.g., -1 inserts before last element)
			i = len(list.Elements) + i
			if i < 0 {
				i = 0
			}
		}
		if i > len(list.Elements) {
			i = len(list.Elements)
		}
		// Optimized insert: avoid intermediate slice allocation
		list.Elements = append(list.Elements, nil)
		copy(list.Elements[i+1:], list.Elements[i:])
		list.Elements[i] = args[1]
		return NULL
	case "remove":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		value := args[0]
		for i, elem := range list.Elements {
			eq, rerr := evalObjectsEqualChecked(ctx, elem, value, env)
			if rerr != nil {
				return rerr
			}
			if eq {
				list.Elements = append(list.Elements[:i], list.Elements[i+1:]...)
				return NULL
			}
		}
		return errors.NewError("value not in list")
	case "clear":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		list.Elements = []object.Object{}
		return NULL
	case "copy":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		elements := make([]object.Object, len(list.Elements))
		copy(elements, list.Elements)
		return &object.List{Elements: elements}
	case "reverse":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		for i, j := 0, len(list.Elements)-1; i < j; i, j = i+1, j-1 {
			list.Elements[i], list.Elements[j] = list.Elements[j], list.Elements[i]
		}
		return NULL
	case "sort":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		// Check for key and reverse kwargs
		var keyFunc object.Object
		reverse := false
		if keywords != nil {
			if kf, ok := keywords["key"]; ok {
				keyFunc = kf
			}
			if rev, ok := keywords["reverse"]; ok {
				if b, err := rev.AsBool(); err == nil {
					reverse = b
				}
			}
		}
		// Sort in place using Go's efficient sort (O(n log n))
		n := len(list.Elements)
		if n > 1 {
			// Pre-compute keys if key function is provided
			var keys []object.Object
			if keyFunc != nil {
				keys = make([]object.Object, n)
				for i, elem := range list.Elements {
					key := applyFunctionWithContext(ctx, keyFunc, []object.Object{elem}, nil, env)
					if object.IsError(key) {
						return key
					}
					keys[i] = key
				}
			}
			// Create index array to track original positions
			indices := make([]int, n)
			for i := range indices {
				indices[i] = i
			}
			// Sort indices with the same comparator sorted() uses: numbers
			// order together, strings lexicographically, tuples/lists
			// element-wise, instances through their dunders, and anything
			// incomparable is an error rather than a silent no-op (which is
			// what compareObjects' 0-for-incomparable would do here).
			var sortErr object.Object
			sort.SliceStable(indices, func(i, j int) bool {
				var left, right object.Object
				if keys != nil {
					left, right = keys[indices[i]], keys[indices[j]]
				} else {
					left, right = list.Elements[indices[i]], list.Elements[indices[j]]
				}
				cmp, cerr := compareForSort(ctx, left, right, env)
				if cerr != nil {
					sortErr = cerr
					return false
				}
				if reverse {
					return cmp > 0
				}
				return cmp < 0
			})
			if sortErr != nil {
				return sortErr
			}
			// Reorder elements according to sorted indices
			newElements := make([]object.Object, n)
			for i, idx := range indices {
				newElements[i] = list.Elements[idx]
			}
			copy(list.Elements, newElements)
		}
		return NULL
	default:
		return attributeError(list, method)
	}
}

// callBytesMethod handles method dispatch on Bytes values. The method set is
// intentionally minimal — decode/hex/base64 cover the round-trips users
// actually need in glue scripts.
func callBytesMethod(ctx context.Context, b *object.Bytes, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch method {
	case "split":
		// Python bytes.split(sep=None, maxsplit=-1): whitespace runs when
		// no separator, else byte-separator occurrences.
		if len(args) > 2 {
			return errors.NewError("split() takes at most 2 arguments (%d given)", len(args))
		}
		data := b.BytesValue()
		maxSplit := -1
		if len(args) == 2 {
			if n, err := args[1].AsInt(); err == nil {
				maxSplit = int(n)
			}
		}
		var parts [][]byte
		if len(args) == 0 || args[0].Type() == object.NULL_OBJ {
			parts = bytesFields(data)
		} else {
			sepObj, ok := args[0].(*object.Bytes)
			if !ok {
				return errors.NewTypeError("bytes", args[0].Type().String())
			}
			sep := sepObj.BytesValue()
			if len(sep) == 0 {
				return errors.NewError("empty separator")
			}
			if maxSplit < 0 {
				parts = bytes.Split(data, sep)
			} else {
				parts = bytes.SplitN(data, sep, maxSplit+1)
			}
		}
		elems := make([]object.Object, len(parts))
		for i, p := range parts {
			elems[i] = object.NewBytes(p)
		}
		return &object.List{Elements: elems}
	case "decode":
		// Python signature: decode(encoding="utf-8", errors="strict"),
		// positional or keyword.
		if len(args) > 2 {
			return errors.NewError("decode() takes at most 2 arguments (%d given)", len(args))
		}
		encoding := "utf-8"
		errorsMode := "strict"
		if len(args) >= 1 {
			s, err := args[0].AsString()
			if err != nil {
				return err
			}
			encoding = s
		} else if keywords != nil {
			if encObj, ok := keywords["encoding"]; ok {
				s, err := encObj.AsString()
				if err != nil {
					return err
				}
				encoding = s
			}
		}
		if len(args) >= 2 {
			s, err := args[1].AsString()
			if err != nil {
				return err
			}
			errorsMode = s
		} else if keywords != nil {
			if errObj, ok := keywords["errors"]; ok {
				s, err := errObj.AsString()
				if err != nil {
					return err
				}
				errorsMode = s
			}
		}
		if errorsMode != "strict" && errorsMode != "ignore" && errorsMode != "replace" {
			return errors.NewError("decode: unknown error handler %q (use strict, ignore or replace)", errorsMode)
		}
		decoded, errObj := decodeBytes(b.BytesValue(), encoding, errorsMode)
		if errObj != nil {
			return errObj
		}
		return object.NewString(decoded)
	case "hex":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		return object.NewString(hex.EncodeToString(b.BytesValue()))
	case "base64":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		return object.NewString(base64.StdEncoding.EncodeToString(b.BytesValue()))
	case "length":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		return object.NewInteger(int64(b.Len()))
	}
	return attributeError(b, method)
}

func callStringMethod(ctx context.Context, str *object.String, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch method {
	case "upper":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		return object.NewString(fastStringUpper(str.StringValue()))
	case "lower":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		return object.NewString(fastStringLower(str.StringValue()))
	case "split":
		if err := errors.MaxArgs(args, 2); err != nil {
			return err
		}
		// If no argument, split on whitespace
		if len(args) == 0 {
			parts := strings.Fields(str.StringValue())
			elements := make([]object.Object, len(parts))
			for i, part := range parts {
				elements[i] = object.NewString(part)
			}
			return &object.List{Elements: elements}
		}
		// With separator argument
		sep, errObj := args[0].AsString()
		if errObj != nil {
			return errors.ParameterError("sep", errObj)
		}

		var parts []string
		if len(args) == 1 {
			parts = strings.Split(str.StringValue(), sep)
		} else {
			maxsplitObj := args[1]
			maxsplit, err := maxsplitObj.AsInt()
			if err != nil {
				return errors.ParameterError("maxsplit", err)
			}
			n := int(maxsplit + 1)
			if maxsplit < 0 {
				n = -1
			}
			parts = strings.SplitN(str.StringValue(), sep, n)
		}

		elements := make([]object.Object, len(parts))
		for i, part := range parts {
			elements[i] = object.NewString(part)
		}
		return &object.List{Elements: elements}
	case "rsplit":
		if err := errors.MaxArgs(args, 2); err != nil {
			return err
		}
		// rsplit works from the right; maxsplit limits splits counted from
		// the end, so the leftmost element keeps the un-split remainder.
		if len(args) == 0 || args[0].Type() == object.NULL_OBJ {
			maxsplit := int64(-1)
			if len(args) == 2 {
				n, err := args[1].AsInt()
				if err != nil {
					return errors.ParameterError("maxsplit", err)
				}
				maxsplit = n
			}
			parts := rsplitFields(str.StringValue(), maxsplit)
			elements := make([]object.Object, len(parts))
			for i, part := range parts {
				elements[i] = object.NewString(part)
			}
			return &object.List{Elements: elements}
		}
		sep, errObj := args[0].AsString()
		if errObj != nil {
			return errors.ParameterError("sep", errObj)
		}
		var parts []string
		if len(args) == 1 {
			parts = strings.Split(str.StringValue(), sep)
		} else {
			maxsplit, err := args[1].AsInt()
			if err != nil {
				return errors.ParameterError("maxsplit", err)
			}
			parts = rsplitSep(str.StringValue(), sep, maxsplit)
		}
		elements := make([]object.Object, len(parts))
		for i, part := range parts {
			elements[i] = object.NewString(part)
		}
		return &object.List{Elements: elements}
	case "replace":
		if len(args) < 2 || len(args) > 3 {
			return errors.NewError("replace() takes 2-3 arguments (%d given)", len(args))
		}
		old, err := args[0].AsString()
		if err != nil {
			return err
		}
		newVal, err := args[1].AsString()
		if err != nil {
			return err
		}
		if len(args) == 3 {
			count, err := args[2].AsInt()
			if err != nil {
				return errors.ParameterError("count", err)
			}
			return object.NewString(strings.Replace(str.StringValue(), old, newVal, int(count)))
		}
		return object.NewString(strings.ReplaceAll(str.StringValue(), old, newVal))
	case "join":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		// Accept any iterable of strings (list, tuple, set, dict, dict views,
		// string, iterator, class instances with __iter__) — matching Python's
		// str.join. A raise from the iterator protocol propagates.
		elements, ok, rerr := iterableToSliceChecked(ctx, args[0], env)
		if rerr != nil {
			return rerr
		}
		if !ok {
			return errors.NewTypeError("iterable", args[0].Type().String())
		}
		parts := make([]string, len(elements))
		for i, elem := range elements {
			// Elements convert with str() semantics: instances dispatch
			// __str__ and exceptions use their message (a raise propagates
			// from renderFormatArg); plain strings are used as-is.
			if inst, isInst := elem.(*object.Instance); isInst {
				rendered, rerr := strInstanceChecked(ctx, inst, env)
				if rerr != nil {
					return rerr
				}
				parts[i] = rendered
				continue
			}
			if s, err := elem.AsString(); err == nil {
				parts[i] = s
			} else {
				parts[i] = elem.Inspect()
			}
		}
		return object.NewString(strings.Join(parts, str.StringValue()))
	case "capitalize":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(str.StringValue()) == 0 {
			return str
		}
		runes := []rune(str.StringValue())
		// Use strings.Builder for efficient string building
		var builder strings.Builder
		builder.Grow(len(runes))
		builder.WriteRune(unicode.ToUpper(runes[0]))
		for _, r := range runes[1:] {
			builder.WriteRune(unicode.ToLower(r))
		}
		return object.NewString(builder.String())
	case "title":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		return object.NewString(cases.Title(language.Und).String(str.StringValue()))
	case "strip":
		if len(args) > 1 {
			return errors.NewError("strip() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			chars, errObj := args[0].AsString()
			if errObj != nil {
				return errors.ParameterError("chars", errObj)
			}
			return object.NewString(strings.Trim(str.StringValue(), chars))
		}
		return object.NewString(strings.TrimSpace(str.StringValue()))
	case "lstrip":
		if len(args) > 1 {
			return errors.NewError("lstrip() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			chars, errObj := args[0].AsString()
			if errObj != nil {
				return errors.ParameterError("chars", errObj)
			}
			return object.NewString(strings.TrimLeft(str.StringValue(), chars))
		}
		return object.NewString(strings.TrimLeft(str.StringValue(), " \t\n\r\v\f"))
	case "rstrip":
		if len(args) > 1 {
			return errors.NewError("rstrip() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			chars, errObj := args[0].AsString()
			if errObj != nil {
				return errors.ParameterError("chars", errObj)
			}
			return object.NewString(strings.TrimRight(str.StringValue(), chars))
		}
		return object.NewString(strings.TrimRight(str.StringValue(), " \t\n\r\v\f"))
	case "startswith", "endswith":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("%s() takes 1-3 arguments (%d given)", method, len(args))
		}
		s := str.StringValue()
		if len(args) > 1 {
			start, err := args[1].AsInt()
			if err != nil {
				return errors.ParameterError("start", err)
			}
			if start < 0 {
				start = 0
			}
			if start > int64(len(s)) {
				start = int64(len(s))
			}
			s = s[start:]
		}
		if len(args) > 2 {
			end, err := args[2].AsInt()
			if err != nil {
				return errors.ParameterError("end", err)
			}
			// end is relative to the original string, matching Python.
			orig := str.StringValue()
			if end < 0 {
				end += int64(len(orig))
				if end < 0 {
					end = 0
				}
			}
			if end > int64(len(orig)) {
				end = int64(len(orig))
			}
			if end < int64(len(orig)-len(s)) {
				end = int64(len(orig) - len(s))
			}
			s = s[:end-int64(len(orig)-len(s))]
		}
		atStart := method == "startswith"
		check := func(prefix string) bool {
			if atStart {
				return strings.HasPrefix(s, prefix)
			}
			return strings.HasSuffix(s, prefix)
		}
		switch prefixes := args[0].(type) {
		case *object.String:
			return nativeBoolToBooleanObject(check(prefixes.StringValue()))
		case *object.Tuple:
			for _, e := range prefixes.Elements {
				prefix, errObj := e.AsString()
				if errObj != nil {
					return errors.ParameterError(method[:len(method)-3], errObj)
				}
				if check(prefix) {
					return TRUE
				}
			}
			return FALSE
		default:
			name := "prefix"
			if !atStart {
				name = "suffix"
			}
			if prefix, errObj := args[0].AsString(); errObj == nil {
				return nativeBoolToBooleanObject(check(prefix))
			}
			return errors.ParameterError(name, errors.NewError("must be str or tuple"))
		}
	case "find":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("find() takes 1-3 arguments (%d given)", len(args))
		}
		substr, errObj := args[0].AsString()
		if errObj != nil {
			return errors.ParameterError("sub", errObj)
		}
		start := 0
		end := len(str.StringValue())
		if len(args) >= 2 {
			s, errObj2 := args[1].AsInt()
			if errObj2 != nil {
				return errors.ParameterError("start", errObj2)
			}
			start = int(s)
			if start < 0 {
				start = len(str.StringValue()) + start
				if start < 0 {
					start = 0
				}
			}
		}
		if len(args) == 3 {
			e, errObj3 := args[2].AsInt()
			if errObj3 != nil {
				return errors.ParameterError("end", errObj3)
			}
			end = int(e)
			if end < 0 {
				end = len(str.StringValue()) + end
			}
		}
		if start > len(str.StringValue()) {
			start = len(str.StringValue())
		}
		if end > len(str.StringValue()) {
			end = len(str.StringValue())
		}
		if start > end {
			return object.NewInteger(-1)
		}
		searchStr := str.StringValue()[start:end]
		idx := strings.Index(searchStr, substr)
		if idx == -1 {
			return object.NewInteger(-1)
		}
		return object.NewInteger(int64(start + idx))
	case "rfind":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("rfind() takes 1-3 arguments (%d given)", len(args))
		}
		substr, errObj := args[0].AsString()
		if errObj != nil {
			return errors.ParameterError("sub", errObj)
		}
		start := 0
		end := len(str.StringValue())
		if len(args) >= 2 {
			s, errObj2 := args[1].AsInt()
			if errObj2 != nil {
				return errors.ParameterError("start", errObj2)
			}
			start = int(s)
			if start < 0 {
				start = len(str.StringValue()) + start
				if start < 0 {
					start = 0
				}
			}
		}
		if len(args) == 3 {
			e, errObj3 := args[2].AsInt()
			if errObj3 != nil {
				return errors.ParameterError("end", errObj3)
			}
			end = int(e)
			if end < 0 {
				end = len(str.StringValue()) + end
			}
		}
		if start > len(str.StringValue()) {
			start = len(str.StringValue())
		}
		if end > len(str.StringValue()) {
			end = len(str.StringValue())
		}
		if start > end {
			return object.NewInteger(-1)
		}
		searchStr := str.StringValue()[start:end]
		idx := strings.LastIndex(searchStr, substr)
		if idx == -1 {
			return object.NewInteger(-1)
		}
		return object.NewInteger(int64(start + idx))
	case "rindex":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("rindex() takes 1-3 arguments (%d given)", len(args))
		}
		substr, errObj := args[0].AsString()
		if errObj != nil {
			return errors.ParameterError("sub", errObj)
		}
		start := 0
		end := len(str.StringValue())
		if len(args) >= 2 {
			s, errObj2 := args[1].AsInt()
			if errObj2 != nil {
				return errors.ParameterError("start", errObj2)
			}
			start = int(s)
			if start < 0 {
				start = len(str.StringValue()) + start
				if start < 0 {
					start = 0
				}
			}
		}
		if len(args) == 3 {
			e, errObj3 := args[2].AsInt()
			if errObj3 != nil {
				return errors.ParameterError("end", errObj3)
			}
			end = int(e)
			if end < 0 {
				end = len(str.StringValue()) + end
			}
		}
		if start > len(str.StringValue()) {
			start = len(str.StringValue())
		}
		if end > len(str.StringValue()) {
			end = len(str.StringValue())
		}
		if start > end {
			return errors.NewError("substring not found")
		}
		searchStr := str.StringValue()[start:end]
		idx := strings.LastIndex(searchStr, substr)
		if idx == -1 {
			return errors.NewError("substring not found")
		}
		return object.NewInteger(int64(start + idx))
	case "index":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("index() takes 1-3 arguments (%d given)", len(args))
		}
		substr, errObj := args[0].AsString()
		if errObj != nil {
			return errors.ParameterError("sub", errObj)
		}
		start := 0
		end := len(str.StringValue())
		if len(args) >= 2 {
			s, errObj2 := args[1].AsInt()
			if errObj2 != nil {
				return errors.ParameterError("start", errObj2)
			}
			start = int(s)
			if start < 0 {
				start = len(str.StringValue()) + start
				if start < 0 {
					start = 0
				}
			}
		}
		if len(args) == 3 {
			e, errObj3 := args[2].AsInt()
			if errObj3 != nil {
				return errors.ParameterError("end", errObj3)
			}
			end = int(e)
			if end < 0 {
				end = len(str.StringValue()) + end
			}
		}
		if start > len(str.StringValue()) {
			start = len(str.StringValue())
		}
		if end > len(str.StringValue()) {
			end = len(str.StringValue())
		}
		if start > end {
			return errors.NewError("substring not found")
		}
		searchStr := str.StringValue()[start:end]
		idx := strings.Index(searchStr, substr)
		if idx == -1 {
			return errors.NewError("substring not found")
		}
		return object.NewInteger(int64(start + idx))
	case "count":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("count() takes 1-3 arguments (%d given)", len(args))
		}
		substr, errObj := args[0].AsString()
		if errObj != nil {
			return errors.ParameterError("sub", errObj)
		}
		start := 0
		end := len(str.StringValue())
		if len(args) >= 2 {
			s, errObj2 := args[1].AsInt()
			if errObj2 != nil {
				return errors.ParameterError("start", errObj2)
			}
			start = int(s)
			if start < 0 {
				start = len(str.StringValue()) + start
				if start < 0 {
					start = 0
				}
			}
		}
		if len(args) == 3 {
			e, errObj3 := args[2].AsInt()
			if errObj3 != nil {
				return errors.ParameterError("end", errObj3)
			}
			end = int(e)
			if end < 0 {
				end = len(str.StringValue()) + end
			}
		}
		if start > len(str.StringValue()) {
			start = len(str.StringValue())
		}
		if end > len(str.StringValue()) {
			end = len(str.StringValue())
		}
		if start > end {
			return object.NewInteger(0)
		}
		searchStr := str.StringValue()[start:end]
		return object.NewInteger(int64(strings.Count(searchStr, substr)))
	case "format":
		// Python-style formatting: {}, {0}, {name}, {{ }} escapes and
		// optional :format specs. Positional fields draw on the arguments,
		// named fields on the keyword arguments.
		result, ferr := evalStringFormatMethod(ctx, str.StringValue(), args, keywords, env)
		if ferr != nil {
			return ferr
		}
		return object.NewString(result)
	case "isdigit":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(str.StringValue()) == 0 {
			return FALSE
		}
		for _, ch := range str.StringValue() {
			if ch < '0' || ch > '9' {
				return FALSE
			}
		}
		return TRUE
	case "isalpha":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(str.StringValue()) == 0 {
			return FALSE
		}
		for _, ch := range str.StringValue() {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) {
				return FALSE
			}
		}
		return TRUE
	case "isalnum":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(str.StringValue()) == 0 {
			return FALSE
		}
		for _, ch := range str.StringValue() {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
				return FALSE
			}
		}
		return TRUE
	case "isspace":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(str.StringValue()) == 0 {
			return FALSE
		}
		for _, ch := range str.StringValue() {
			if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' && ch != '\v' && ch != '\f' {
				return FALSE
			}
		}
		return TRUE
	case "isupper":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		hasUpper := false
		for _, ch := range str.StringValue() {
			if ch >= 'a' && ch <= 'z' {
				return FALSE
			}
			if ch >= 'A' && ch <= 'Z' {
				hasUpper = true
			}
		}
		if hasUpper {
			return TRUE
		}
		return FALSE
	case "islower":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		hasLower := false
		for _, ch := range str.StringValue() {
			if ch >= 'A' && ch <= 'Z' {
				return FALSE
			}
			if ch >= 'a' && ch <= 'z' {
				hasLower = true
			}
		}
		if hasLower {
			return TRUE
		}
		return FALSE
	case "zfill":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		width, errObj := args[0].AsInt()
		if errObj != nil {
			return errors.ParameterError("width", errObj)
		}
		w := int(width)
		if w <= len(str.StringValue()) {
			return str
		}
		// Handle negative sign
		if len(str.StringValue()) > 0 && (str.StringValue()[0] == '-' || str.StringValue()[0] == '+') {
			var builder strings.Builder
			builder.Grow(w)
			builder.WriteByte(str.StringValue()[0])
			builder.WriteString(strings.Repeat("0", w-len(str.StringValue())))
			builder.WriteString(str.StringValue()[1:])
			return object.NewString(builder.String())
		}
		var builder strings.Builder
		builder.Grow(w)
		builder.WriteString(strings.Repeat("0", w-len(str.StringValue())))
		builder.WriteString(str.StringValue())
		return object.NewString(builder.String())
	case "center":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("center() takes 1-2 arguments (%d given)", len(args))
		}
		width, errObj := args[0].AsInt()
		if errObj != nil {
			return errors.ParameterError("width", errObj)
		}
		w := int(width)
		if w <= len(str.StringValue()) {
			return str
		}
		fillChar := " "
		if len(args) == 2 {
			fill, errObj2 := args[1].AsString()
			if errObj2 != nil {
				return errors.ParameterError("fillchar", errObj2)
			}
			if len(fill) != 1 {
				return errors.NewError("fill character must be exactly one character")
			}
			fillChar = fill
		}
		padding := w - len(str.StringValue())
		// CPython's center: left = padding/2 + (padding & width & 1).
		// The extra character alternates with the parity of the target
		// width: 'ab'.center(7, '*') is '***ab**' but 'x'.center(6) is
		// '  x   '.
		leftPad := padding/2 + (padding & w & 1)
		rightPad := padding - leftPad
		// Use strings.Builder for efficient concatenation
		var builder strings.Builder
		builder.Grow(w)
		builder.WriteString(strings.Repeat(fillChar, leftPad))
		builder.WriteString(str.StringValue())
		builder.WriteString(strings.Repeat(fillChar, rightPad))
		return object.NewString(builder.String())
	case "ljust":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("ljust() takes 1-2 arguments (%d given)", len(args))
		}
		width, errObj := args[0].AsInt()
		if errObj != nil {
			return errors.ParameterError("width", errObj)
		}
		w := int(width)
		if w <= len(str.StringValue()) {
			return str
		}
		fillChar := " "
		if len(args) == 2 {
			fill, errObj2 := args[1].AsString()
			if errObj2 != nil {
				return errors.ParameterError("fillchar", errObj2)
			}
			if len(fill) != 1 {
				return errors.NewError("fill character must be exactly one character")
			}
			fillChar = fill
		}
		// Use strings.Builder for efficient concatenation
		var builder strings.Builder
		builder.Grow(w)
		builder.WriteString(str.StringValue())
		builder.WriteString(strings.Repeat(fillChar, w-len(str.StringValue())))
		return object.NewString(builder.String())
	case "rjust":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("rjust() takes 1-2 arguments (%d given)", len(args))
		}
		width, errObj := args[0].AsInt()
		if errObj != nil {
			return errors.ParameterError("width", errObj)
		}
		w := int(width)
		if w <= len(str.StringValue()) {
			return str
		}
		fillChar := " "
		if len(args) == 2 {
			fill, errObj2 := args[1].AsString()
			if errObj2 != nil {
				return errors.ParameterError("fillchar", errObj2)
			}
			if len(fill) != 1 {
				return errors.NewError("fill character must be exactly one character")
			}
			fillChar = fill
		}
		// Use strings.Builder for efficient concatenation
		var builder strings.Builder
		builder.Grow(w)
		builder.WriteString(strings.Repeat(fillChar, w-len(str.StringValue())))
		builder.WriteString(str.StringValue())
		return object.NewString(builder.String())
	case "splitlines":
		keepends := false
		if len(args) > 1 {
			return errors.NewError("splitlines() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			b, errObj := args[0].AsBool()
			if errObj != nil {
				return errors.ParameterError("keepends", errObj)
			}
			keepends = b
		} else if keywords != nil {
			if kw := keywords["keepends"]; kw != nil {
				b, errObj := kw.AsBool()
				if errObj != nil {
					return errors.ParameterError("keepends", errObj)
				}
				keepends = b
			}
		}
		lines := []object.Object{}
		text := str.StringValue()
		start := 0
		for i := 0; i < len(text); i++ {
			if text[i] == '\n' {
				if keepends {
					lines = append(lines, object.NewString(text[start:i+1]))
				} else {
					lines = append(lines, object.NewString(text[start:i]))
				}
				start = i + 1
			} else if text[i] == '\r' {
				end := i
				if i+1 < len(text) && text[i+1] == '\n' {
					i++
				}
				if keepends {
					lines = append(lines, object.NewString(text[start:i+1]))
				} else {
					lines = append(lines, object.NewString(text[start:end]))
				}
				start = i + 1
			}
		}
		if start < len(text) {
			lines = append(lines, object.NewString(text[start:]))
		}
		return &object.List{Elements: lines}
	case "swapcase":
		if len(args) != 0 {
			return errors.NewError("swapcase() takes no arguments (%d given)", len(args))
		}
		result := make([]rune, len(str.StringValue()))
		for i, r := range str.StringValue() {
			if r >= 'A' && r <= 'Z' {
				result[i] = r + 32
			} else if r >= 'a' && r <= 'z' {
				result[i] = r - 32
			} else {
				result[i] = r
			}
		}
		return object.NewString(string(result))
	case "partition":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		sep, err := args[0].AsString()
		if err != nil {
			return err
		}
		idx := strings.Index(str.StringValue(), sep)
		if idx < 0 {
			return &object.Tuple{Elements: []object.Object{
				str,
				object.NewString(""),
				object.NewString(""),
			}}
		}
		return &object.Tuple{Elements: []object.Object{
			object.NewString(str.StringValue()[:idx]),
			object.NewString(sep),
			object.NewString(str.StringValue()[idx+len(sep):]),
		}}
	case "rpartition":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		sep, err := args[0].AsString()
		if err != nil {
			return err
		}
		idx := strings.LastIndex(str.StringValue(), sep)
		if idx < 0 {
			return &object.Tuple{Elements: []object.Object{
				object.NewString(""),
				object.NewString(""),
				str,
			}}
		}
		return &object.Tuple{Elements: []object.Object{
			object.NewString(str.StringValue()[:idx]),
			object.NewString(sep),
			object.NewString(str.StringValue()[idx+len(sep):]),
		}}
	case "removeprefix":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		prefix, err := args[0].AsString()
		if err != nil {
			return err
		}
		if strings.HasPrefix(str.StringValue(), prefix) {
			return object.NewString(str.StringValue()[len(prefix):])
		}
		return str
	case "removesuffix":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		suffix, err := args[0].AsString()
		if err != nil {
			return err
		}
		if strings.HasSuffix(str.StringValue(), suffix) {
			return object.NewString(str.StringValue()[:len(str.StringValue())-len(suffix)])
		}
		return str
	case "encode":
		// Python signature: encode(encoding="utf-8", errors="strict").
		if len(args) > 2 {
			return errors.NewError("encode() takes at most 2 arguments (%d given)", len(args))
		}
		encoding := "utf-8"
		errorsMode := "strict"
		if len(args) >= 1 {
			enc, errObj := args[0].AsString()
			if errObj != nil {
				return errors.ParameterError("encoding", errObj)
			}
			encoding = enc
		} else if keywords != nil {
			if encObj, ok := keywords["encoding"]; ok {
				enc, errObj := encObj.AsString()
				if errObj != nil {
					return errors.ParameterError("encoding", errObj)
				}
				encoding = enc
			}
		}
		if len(args) >= 2 {
			em, errObj := args[1].AsString()
			if errObj != nil {
				return errors.ParameterError("errors", errObj)
			}
			errorsMode = em
		} else if keywords != nil {
			if emObj, ok := keywords["errors"]; ok {
				em, errObj := emObj.AsString()
				if errObj != nil {
					return errors.ParameterError("errors", errObj)
				}
				errorsMode = em
			}
		}
		if errorsMode != "strict" && errorsMode != "ignore" && errorsMode != "replace" {
			return errors.NewError("encode: unknown error handler %q (use strict, ignore or replace)", errorsMode)
		}
		out, errObj := encodeStr(str.StringValue(), encoding, errorsMode)
		if errObj != nil {
			return errObj
		}
		return object.NewBytes(out)
	case "expandtabs":
		tabsize := 8
		if len(args) > 1 {
			return errors.NewError("expandtabs() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			ts, errObj := args[0].AsInt()
			if errObj != nil {
				return errors.ParameterError("tabsize", errObj)
			}
			tabsize = int(ts)
		}
		var result strings.Builder
		col := 0
		for _, ch := range str.StringValue() {
			if ch == '\t' {
				spaces := tabsize - (col % tabsize)
				result.WriteString(strings.Repeat(" ", spaces))
				col += spaces
			} else if ch == '\n' || ch == '\r' {
				result.WriteRune(ch)
				col = 0
			} else {
				result.WriteRune(ch)
				col++
			}
		}
		return object.NewString(result.String())
	case "casefold":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		// casefold is more aggressive than lower() for Unicode
		// For ASCII, it's equivalent to lower()
		return object.NewString(strings.ToLower(str.StringValue()))
	case "maketrans":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("maketrans() takes 1, 2, or 3 arguments (%d given)", len(args))
		}
		transMap := &object.Dict{Pairs: make(map[string]object.DictPair)}
		if len(args) == 1 {
			// Single argument: must be a dict
			d, err := args[0].AsDict()
			if err != nil {
				return errors.ParameterError("table", err)
			}
			for k, v := range d {
				transMap.Pairs[object.DictKey(object.NewString(k))] = object.DictPair{Key: object.NewString(k), Value: v}
			}
			return transMap
		}
		// Two arguments: from and to strings
		from, errFrom := args[0].AsString()
		if errFrom != nil {
			return errors.ParameterError("from", errFrom)
		}
		to, errTo := args[1].AsString()
		if errTo != nil {
			return errors.ParameterError("to", errTo)
		}
		fromRunes := []rune(from)
		toRunes := []rune(to)
		if len(fromRunes) != len(toRunes) {
			return errors.NewError("maketrans() arguments must have equal length")
		}
		for i, ch := range fromRunes {
			key := object.DictKey(object.NewString(string(ch)))
			transMap.Pairs[key] = object.DictPair{
				Key:   object.NewString(string(ch)),
				Value: object.NewString(string(toRunes[i])),
			}
		}
		// Third argument: characters to delete
		if len(args) == 3 {
			del, errDel := args[2].AsString()
			if errDel != nil {
				return errors.ParameterError("deletechars", errDel)
			}
			for _, ch := range del {
				key := object.DictKey(object.NewString(string(ch)))
				transMap.Pairs[key] = object.DictPair{
					Key:   object.NewString(string(ch)),
					Value: NULL,
				}
			}
		}
		return transMap
	case "translate":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		transMap, ok := args[0].(*object.Dict)
		if !ok {
			return errors.NewTypeError("DICT", args[0].Type().String())
		}
		var result strings.Builder
		for _, ch := range str.StringValue() {
			key := object.DictKey(object.NewString(string(ch)))
			if pair, exists := transMap.Pairs[key]; exists {
				if pair.Value == NULL || pair.Value.Type() == object.NULL_OBJ {
					// Delete character
					continue
				}
				if s, err := pair.Value.AsString(); err == nil {
					result.WriteString(s)
				} else {
					result.WriteRune(ch)
				}
			} else {
				result.WriteRune(ch)
			}
		}
		return object.NewString(result.String())
	case "isnumeric":
		// Returns True if all characters are numeric (0-9, superscripts, fractions, etc.)
		// For simplicity, we check for Unicode numeric characters
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(str.StringValue()) == 0 {
			return FALSE
		}
		for _, ch := range str.StringValue() {
			// Check if character is in Unicode numeric categories
			if !unicode.IsNumber(ch) {
				return FALSE
			}
		}
		return TRUE
	case "isdecimal":
		// Returns True if all characters are decimal digits (0-9)
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(str.StringValue()) == 0 {
			return FALSE
		}
		for _, ch := range str.StringValue() {
			if ch < '0' || ch > '9' {
				return FALSE
			}
		}
		return TRUE
	case "istitle":
		// Returns True if string is titlecased
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(str.StringValue()) == 0 {
			return FALSE
		}
		// Title case: first char of each word is uppercase, rest are lowercase
		prevCased := false
		hasCased := false
		for _, ch := range str.StringValue() {
			isUpper := ch >= 'A' && ch <= 'Z'
			isLower := ch >= 'a' && ch <= 'z'
			isCased := isUpper || isLower
			if isCased {
				hasCased = true
				if prevCased {
					// Previous char was cased, this one should be lowercase
					if !isLower {
						return FALSE
					}
				} else {
					// Previous char was not cased, this one should be uppercase
					if !isUpper {
						return FALSE
					}
				}
			}
			prevCased = isCased
		}
		if hasCased {
			return TRUE
		}
		return FALSE
	case "isidentifier":
		// Returns True if string is a valid identifier
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(str.StringValue()) == 0 {
			return FALSE
		}
		for i, ch := range str.StringValue() {
			if i == 0 {
				// First character must be letter or underscore
				if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_') {
					return FALSE
				}
			} else {
				// Subsequent characters can also be digits
				if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
					return FALSE
				}
			}
		}
		return TRUE
	case "isprintable":
		// Returns True if all characters are printable
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		// Empty string is considered printable
		for _, ch := range str.StringValue() {
			if !unicode.IsPrint(ch) && ch != ' ' {
				return FALSE
			}
		}
		return TRUE
	default:
		return attributeError(str, method)
	}
}
func callTupleMethod(ctx context.Context, tuple *object.Tuple, method string, args []object.Object, env *object.Environment) object.Object {
	switch method {
	case "count":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		count := int64(0)
		for _, elem := range tuple.Elements {
			eq, rerr := evalObjectsEqualChecked(ctx, elem, args[0], env)
			if rerr != nil {
				return rerr
			}
			if eq {
				count++
			}
		}
		return object.NewInteger(count)
	case "index":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("index() takes 1-3 arguments (%d given)", len(args))
		}
		start, end := 0, len(tuple.Elements)
		if len(args) >= 2 {
			s, err := args[1].AsInt()
			if err != nil {
				return errors.ParameterError("start", err)
			}
			start = int(s)
			if start < 0 {
				start = len(tuple.Elements) + start
			}
			if start < 0 {
				start = 0
			}
		}
		if len(args) == 3 {
			e, err := args[2].AsInt()
			if err != nil {
				return errors.ParameterError("end", err)
			}
			end = int(e)
			if end < 0 {
				end = len(tuple.Elements) + end
			}
		}
		if start > len(tuple.Elements) {
			start = len(tuple.Elements)
		}
		if end > len(tuple.Elements) {
			end = len(tuple.Elements)
		}
		for i := start; i < end; i++ {
			eq, rerr := evalObjectsEqualChecked(ctx, tuple.Elements[i], args[0], env)
			if rerr != nil {
				return rerr
			}
			if eq {
				return object.NewInteger(int64(i))
			}
		}
		return errors.NewError("value not in tuple")
	}
	return attributeError(tuple, method)
}

func callFloatArrayMethod(fa *object.FloatArray, method string, args []object.Object) object.Object {
	switch method {
	case "tolist":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		return fa.ToList()
	case "shape":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		elems := make([]object.Object, len(fa.Shape))
		for i, v := range fa.Shape {
			elems[i] = object.NewInteger(int64(v))
		}
		return &object.List{Elements: elems}
	default:
		return attributeError(fa, method)
	}
}

func callSetMethod(ctx context.Context, set *object.Set, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Frozen sets have no mutating methods at all, exactly as Python's
	// frozenset: calling one is an AttributeError naming the method.
	if set.Frozen {
		switch method {
		case "add", "remove", "discard", "pop", "clear", "update",
			"intersection_update", "difference_update", "symmetric_difference_update":
			return &object.Exception{
				Message:       fmt.Sprintf("'frozenset' object has no attribute '%s'", method),
				ExceptionType: object.ExceptionTypeAttributeError,
				Raised:        true,
			}
		}
	}
	switch method {
	case "add":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		if err := evalSetAdd(ctx, set, args[0]); err != nil {
			return err
		}
		return NULL
	case "remove":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		key, rerr := evalHashKeyChecked(ctx, args[0])
		if rerr != nil {
			return rerr
		}
		if !set.ContainsKeyed(key) {
			return errors.NewError("KeyError: %s", args[0].Inspect())
		}
		delete(set.Elements, key)
		return NULL
	case "discard":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		key, rerr := evalHashKeyChecked(ctx, args[0])
		if rerr != nil {
			return rerr
		}
		delete(set.Elements, key)
		return NULL
	case "pop":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		if len(set.Elements) == 0 {
			return errors.NewError("pop from an empty set")
		}
		// Go map iteration order is random, which matches Python's arbitrary pop
		for k, elem := range set.Elements {
			delete(set.Elements, k)
			return elem
		}
	case "clear":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		set.Elements = make(map[string]object.Object)
		return NULL
	case "copy":
		if err := errors.ExactArgs(args, 0); err != nil {
			return err
		}
		return set.Copy()
	case "update", "intersection_update", "difference_update", "symmetric_difference_update":
		// In-place variants accepting any number of arguments; non-set
		// iterables are materialized first (Python accepts any iterable).
		if len(args) == 0 {
			return errors.NewError("%s() takes at least 1 argument (0 given)", method)
		}
		sets := make([]*object.Set, 0, len(args))
		for _, arg := range args {
			other, errObj := iterableToSet(ctx, arg, env)
			if errObj != nil {
				return errObj
			}
			sets = append(sets, other)
		}
		for _, other := range sets {
			switch method {
			case "update":
				set.InPlaceUnion(other)
			case "intersection_update":
				set.InPlaceIntersection(other)
			case "difference_update":
				set.InPlaceDifference(other)
			case "symmetric_difference_update":
				set.InPlaceSymmetricDifference(other)
			}
		}
		return NULL
	case "union", "intersection", "difference":
		// Python takes any number of arguments, each any iterable.
		if len(args) == 0 {
			return errors.NewError("%s() takes at least 1 argument (0 given)", method)
		}
		result := set
		for _, arg := range args {
			other, errObj := iterableToSet(ctx, arg, env)
			if errObj != nil {
				return errObj
			}
			switch method {
			case "union":
				result = result.Union(other)
			case "intersection":
				result = result.Intersection(other)
			case "difference":
				result = result.Difference(other)
			}
		}
		return result
	case "symmetric_difference":
		// Python's symmetric_difference takes exactly one other set/iterable.
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		other, errObj := iterableToSet(ctx, args[0], env)
		if errObj != nil {
			return errObj
		}
		return set.SymmetricDifference(other)
	case "issubset":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		other, errObj := iterableToSet(ctx, args[0], env)
		if errObj != nil {
			return errObj
		}
		return nativeBoolToBooleanObject(set.IsSubset(other))
	case "isdisjoint":
		// Python: true when the two sets share no element.
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		other, errObj := iterableToSet(ctx, args[0], env)
		if errObj != nil {
			return errObj
		}
		for key := range set.Elements {
			if other.ContainsKeyed(key) {
				return FALSE
			}
		}
		return TRUE
	case "issuperset":
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		other, errObj := iterableToSet(ctx, args[0], env)
		if errObj != nil {
			return errObj
		}
		return nativeBoolToBooleanObject(set.IsSuperset(other))
	default:
		return attributeError(set, method)
	}
	return NULL
}

// evalStringFormatMethod implements str.format: substitution fields are {}
// (auto-numbered), {0} (explicit index) or {name} (keyword argument), each
// with an optional ":spec" format spec applied via formatWithSpec. "{{" and
// "}}" are literal braces. Values render the way str() would: exceptions use
// their message, instances dispatch __str__ (whose raise propagates).
func evalStringFormatMethod(ctx context.Context, format string, args []object.Object, keywords map[string]object.Object, env *object.Environment) (string, object.Object) {
	var b strings.Builder
	auto := 0
	for i := 0; i < len(format); {
		c := format[i]
		if c == '{' {
			if i+1 < len(format) && format[i+1] == '{' {
				b.WriteByte('{')
				i += 2
				continue
			}
			// Field extends to the brace closing at depth 0, so specs may
			// contain nested fields ({x:>{w}}) without ending early.
			depth := 0
			end := -1
			for j := i + 1; j < len(format); j++ {
				switch format[j] {
				case '{':
					depth++
				case '}':
					if depth == 0 {
						end = j - i - 1
					} else {
						depth--
					}
				}
				if end >= 0 {
					break
				}
			}
			if end < 0 {
				return "", errors.NewError("single '{' in format string")
			}
			field := format[i+1 : i+1+end]
			i += end + 2

			name := field
			spec := ""
			conv := ""
			if colon := strings.IndexByte(field, ':'); colon >= 0 {
				name, spec = field[:colon], field[colon+1:]
			}
			if bang := strings.IndexByte(name, '!'); bang >= 0 {
				conv = name[bang+1:]
				name = name[:bang]
				if conv != "r" && conv != "s" && conv != "a" {
					return "", errors.NewError("unknown conversion '%s'", conv)
				}
			}

			var val object.Object
			switch {
			case name == "":
				if auto >= len(args) {
					return "", errors.NewError("not enough arguments for format string")
				}
				val = args[auto]
				auto++
			case name[0] >= '0' && name[0] <= '9':
				idx, err := strconv.Atoi(name)
				if err != nil || idx < 0 || idx >= len(args) {
					return "", errors.NewError("format index %s out of range (%d arguments)", name, len(args))
				}
				val = args[idx]
			default:
				kw, ok := keywords[name]
				if !ok {
					return "", errors.NewError("format field '%s' has no matching keyword argument", name)
				}
				val = kw
			}

			// Nested spec fields ({x:>{w}}) resolve against the same
			// argument sources as top-level fields.
			if strings.Contains(spec, "{") {
				expanded, serr := expandFormatSpecArgs(ctx, spec, args, keywords, env)
				if serr != nil {
					return "", serr
				}
				spec = expanded
			}
			rendered, rerr := renderFormatArg(ctx, val, spec, conv, env)
			if rerr != nil {
				return "", rerr
			}
			b.WriteString(rendered)
			continue
		}
		if c == '}' {
			if i+1 < len(format) && format[i+1] == '}' {
				b.WriteByte('}')
				i += 2
				continue
			}
			return "", errors.NewError("single '}' in format string")
		}
		b.WriteByte(c)
		i++
	}
	return b.String(), nil
}

// renderFormatArg renders one format value: an optional !r/!s/!a conversion
// is applied first (repr or str semantics, dunder raises propagating), then
// an optional format spec. Without a conversion, exceptions use their message
// and instances dispatch __str__, while other types keep their typed value so
// numeric specs (.2f, >6d) still apply.
func renderFormatArg(ctx context.Context, val object.Object, spec string, conv string, env *object.Environment) (string, object.Object) {
	if conv != "" {
		rendered, rerr := renderConvertedValue(ctx, val, conv, env)
		if rerr != nil {
			return "", rerr
		}
		if spec == "" {
			return rendered, nil
		}
		return formatWithSpec(object.NewString(rendered), spec)
	}
	return formatValueChecked(ctx, val, spec, env)
}

// expandFormatSpecArgs resolves nested replacement fields inside a str.format
// spec (dynamic widths like "{x:>{w}}") against the positional and keyword
// arguments, mirroring the top-level field resolution.
func expandFormatSpecArgs(ctx context.Context, spec string, args []object.Object, keywords map[string]object.Object, env *object.Environment) (string, object.Object) {
	var b strings.Builder
	for i := 0; i < len(spec); {
		c := spec[i]
		if c != '{' {
			b.WriteByte(c)
			i++
			continue
		}
		end := strings.IndexByte(spec[i:], '}')
		if end < 0 {
			return "", errors.NewError("unbalanced '{' in format spec")
		}
		name := spec[i+1 : i+end]
		i += end + 1
		var val object.Object
		switch {
		case name == "":
			return "", errors.NewError("empty nested format field")
		case name[0] >= '0' && name[0] <= '9':
			idx, err := strconv.Atoi(name)
			if err != nil || idx < 0 || idx >= len(args) {
				return "", errors.NewError("format index %s out of range (%d arguments)", name, len(args))
			}
			val = args[idx]
		default:
			kw, ok := keywords[name]
			if !ok {
				return "", errors.NewError("nested format field '%s' has no matching keyword argument", name)
			}
			val = kw
		}
		rendered, rerr := renderConvertedValue(ctx, val, "s", env)
		if rerr != nil {
			return "", rerr
		}
		b.WriteString(rendered)
	}
	return b.String(), nil
}

// rsplitSep splits s on sep from the right, performing at most maxsplit
// splits (negative: no limit), like Python's str.rsplit(sep, maxsplit).
func rsplitSep(s, sep string, maxsplit int64) []string {
	if maxsplit < 0 {
		return strings.Split(s, sep)
	}
	parts := strings.Split(s, sep)
	if int64(len(parts)) <= maxsplit+1 {
		return parts
	}
	keep := int64(len(parts)) - maxsplit
	out := make([]string, 0, maxsplit+1)
	out = append(out, strings.Join(parts[:keep], sep))
	return append(out, parts[keep:]...)
}

// rsplitFields splits s on whitespace runs from the right with at most
// maxsplit splits (negative: no limit), like Python's str.rsplit(None, n):
// the leftmost element keeps its interior and leading whitespace, and only
// the separator run consumed by a split (or trailing whitespace at the end
// of the string) is removed.
func rsplitFields(s string, maxsplit int64) []string {
	if maxsplit < 0 {
		return strings.Fields(s)
	}
	var fields []string
	end := len(s)
	for maxsplit > 0 {
		trimmed := strings.TrimRightFunc(s[:end], unicode.IsSpace)
		if trimmed == "" {
			break
		}
		// Scan back over the last field to find its start.
		start := len(trimmed)
		for start > 0 {
			r, size := utf8.DecodeLastRuneInString(trimmed[:start])
			if unicode.IsSpace(r) {
				break
			}
			start -= size
		}
		fields = append([]string{trimmed[start:]}, fields...)
		end = start
		maxsplit--
	}
	if head := strings.TrimRightFunc(s[:end], unicode.IsSpace); head != "" {
		fields = append([]string{head}, fields...)
	}
	return fields
}

// bytesFields splits data on runs of ASCII whitespace, like Python's
// bytes.split() with no separator.
func bytesFields(data []byte) [][]byte {
	var out [][]byte
	start := -1
	for i, c := range data {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f' {
			if start >= 0 {
				out = append(out, data[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, data[start:])
	}
	if out == nil {
		out = [][]byte{}
	}
	return out
}
