package evaluator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func evalMethodCallExpression(ctx context.Context, mce *ast.MethodCallExpression, env *object.Environment) object.Object {
	obj := evalWithContext(ctx, mce.Object, env)
	if object.IsError(obj) {
		return obj
	}

	args := evalExpressionsWithContext(ctx, mce.Arguments, env)
	if len(args) == 1 && object.IsError(args[0]) {
		return args[0]
	}

	// Evaluate keyword arguments
	var keywords map[string]object.Object
	if len(mce.Keywords) > 0 {
		keywords = make(map[string]object.Object, len(mce.Keywords))
		for k, v := range mce.Keywords {
			val := evalWithContext(ctx, v, env)
			if object.IsError(val) {
				return val
			}
			keywords[k] = val
		}
	}

	// Handle *args unpacking (supports multiple)
	for _, argsUnpackExpr := range mce.ArgsUnpack {
		argsVal := evalWithContext(ctx, argsUnpackExpr, env)
		if object.IsError(argsVal) {
			return argsVal
		}
		unpacked, err := unpackArgsFromIterable(argsVal)
		if err != nil {
			return err
		}
		args = append(args, unpacked...)
	}

	// Handle **kwargs unpacking
	if mce.KwargsUnpack != nil {
		kwargsVal := evalWithContext(ctx, mce.KwargsUnpack, env)
		if object.IsError(kwargsVal) {
			return kwargsVal
		}
		if dict, ok := kwargsVal.(*object.Dict); ok {
			if keywords == nil {
				keywords = make(map[string]object.Object, len(dict.Pairs))
			}
			for _, pair := range dict.Pairs {
				if str, ok := pair.Key.(*object.String); ok {
					keywords[str.Value] = pair.Value
				} else {
					return errors.NewError("keywords must be strings, not %s", pair.Key.Type())
				}
			}
		} else {
			return errors.NewError("argument after ** must be a dictionary, not %s", kwargsVal.Type())
		}
	}

	return callStringMethodWithKeywords(ctx, obj, mce.Method.Value, args, keywords, env)
}

func callStringMethodWithKeywords(ctx context.Context, obj object.Object, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Handle universal methods
	switch method {
	case "type":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(keywords) > 0 {
			return errors.NewError("type() does not accept keyword arguments")
		}
		return &object.String{Value: obj.Type().String()}
	}

	// Handle library method calls (dictionaries)
	if obj.Type() == object.DICT_OBJ {
		return callDictMethod(ctx, obj.(*object.Dict), method, args, keywords, env)
	}

	// Handle list methods
	if obj.Type() == object.LIST_OBJ {
		return callListMethod(ctx, obj.(*object.List), method, args, keywords, env)
	}

	// Handle set methods
	if obj.Type() == object.SET_OBJ {
		return callSetMethod(ctx, obj.(*object.Set), method, args, keywords, env)
	}

	// Handle Instance method calls
	if obj.Type() == object.INSTANCE_OBJ {
		return callInstanceMethod(ctx, obj.(*object.Instance), method, args, keywords, env)
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

	return errors.NewError("object %s has no method %s", obj.Type(), method)
}

func callSuperMethod(ctx context.Context, super *object.Super, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Start searching from the base class of the class specified in super
	currentClass := super.Class.BaseClass

	for currentClass != nil {
		if fn, ok := currentClass.Methods[method]; ok {
			// Bind 'self' for all callable types (Function, Builtin, LambdaFunction, etc.)
			newArgs := prependSelf(super.Instance, args)
			return applyFunctionWithContext(ctx, fn, newArgs, keywords, env)
		}
		currentClass = currentClass.BaseClass
	}

	return errors.NewError("super object has no method %s", method)
}

// prependSelf prepends self to args using a stack buffer for small arg lists to avoid heap allocation.
func prependSelf(self object.Object, args []object.Object) []object.Object {
	n := len(args) + 1
	if n <= 8 {
		var buf [8]object.Object
		buf[0] = self
		copy(buf[1:], args)
		return buf[:n]
	}
	newArgs := make([]object.Object, n)
	newArgs[0] = self
	copy(newArgs[1:], args)
	return newArgs
}

func callInstanceMethod(ctx context.Context, instance *object.Instance, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// First check if it's an instance field (which might be a callable)
	if val, ok := instance.Fields[method]; ok {
		// If it's callable, call it without prepending self
		switch fn := val.(type) {
		case *object.Function, *object.LambdaFunction, *object.Builtin, *object.BoundMethod:
			return applyFunctionWithContext(ctx, fn, args, keywords, env)
		}
		// If not callable and being called, that's an error
		return errors.NewError("'%s' object is not callable", val.Type())
	}

	// Walk up the inheritance chain to find the method
	currentClass := instance.Class
	for currentClass != nil {
		if fn, ok := currentClass.Methods[method]; ok {
			// Bind 'self'
			newArgs := prependSelf(instance, args)
			return applyFunctionWithContext(ctx, fn, newArgs, keywords, env)
		}
		currentClass = currentClass.BaseClass
	}

	return errors.NewError("instance has no method %s", method)
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
	}

	// Check for dict instance methods
	switch method {
	case "keys":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(keywords) > 0 {
			return errors.NewError("keys() does not accept keyword arguments")
		}
		if builtin, ok := builtins["keys"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, object.NewKwargs(nil), dict)
		}
	case "values":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(keywords) > 0 {
			return errors.NewError("values() does not accept keyword arguments")
		}
		if builtin, ok := builtins["values"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, object.NewKwargs(nil), dict)
		}
	case "items":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
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
		key := object.DictKey(args[0])
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
		key := object.DictKey(args[0])
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
					dict.Pairs[object.DictKey(pair[0])] = object.DictPair{Key: pair[0], Value: pair[1]}
				}
			default:
				return errors.NewTypeError("DICT or LIST of pairs", args[0].Type().String())
			}
		}
		return NULL
	case "clear":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(keywords) > 0 {
			return errors.NewError("clear() does not accept keyword arguments")
		}
		dict.Pairs = make(map[string]object.DictPair)
		return NULL
	case "copy":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
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
		key := object.DictKey(args[0])
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
				key := object.DictKey(elem)
				newPairs[key] = object.DictPair{Key: elem, Value: defaultVal}
			}
		case *object.Tuple:
			for _, elem := range iter.Elements {
				key := object.DictKey(elem)
				newPairs[key] = object.DictPair{Key: elem, Value: defaultVal}
			}
		case *object.String:
			for _, ch := range iter.Value {
				s := string(ch)
				key := object.DictKey(&object.String{Value: s})
				newPairs[key] = object.DictPair{Key: &object.String{Value: s}, Value: defaultVal}
			}
		default:
			return errors.NewTypeError("iterable (LIST, TUPLE, STRING)", args[0].Type().String())
		}
		return &object.Dict{Pairs: newPairs}
	}

	// Check for non-callable dict values (for accessing dict attributes)
	dictKey := object.DictKey(&object.String{Value: method})
	if pair, ok := dict.Pairs[dictKey]; ok {
		// If it's not a callable, just return the value
		if len(args) == 0 && len(keywords) == 0 {
			return pair.Value
		}
		return errors.NewError("%s: %s is not callable", errors.ErrIdentifierNotFound, method)
	}
	return errors.NewError("%s: method %s not found in library", errors.ErrIdentifierNotFound, method)
}

func callListMethod(ctx context.Context, list *object.List, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch method {
	case "append":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		list.Elements = append(list.Elements, args[0])
		return NULL
	case "extend":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
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
			if objectsEqual(list.Elements[i], value) {
				return object.NewInteger(int64(i))
			}
		}
		return errors.NewError("value not in list")
	case "count":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		value := args[0]
		count := int64(0)
		for _, elem := range list.Elements {
			if objectsEqual(elem, value) {
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
		if err := errors.ExactArgs(args, 2); err != nil { return err }
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
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		value := args[0]
		for i, elem := range list.Elements {
			if objectsEqual(elem, value) {
				list.Elements = append(list.Elements[:i], list.Elements[i+1:]...)
				return NULL
			}
		}
		return errors.NewError("value not in list")
	case "clear":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		list.Elements = []object.Object{}
		return NULL
	case "copy":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		elements := make([]object.Object, len(list.Elements))
		copy(elements, list.Elements)
		return &object.List{Elements: elements}
	case "reverse":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		for i, j := 0, len(list.Elements)-1; i < j; i, j = i+1, j-1 {
			list.Elements[i], list.Elements[j] = list.Elements[j], list.Elements[i]
		}
		return NULL
	case "sort":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
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
			// Sort indices based on element/key values
			sort.Slice(indices, func(i, j int) bool {
				var left, right object.Object
				if keys != nil {
					left, right = keys[indices[i]], keys[indices[j]]
				} else {
					left, right = list.Elements[indices[i]], list.Elements[indices[j]]
				}
				cmp := compareObjects(left, right)
				if reverse {
					return cmp > 0
				}
				return cmp < 0
			})
			// Reorder elements according to sorted indices
			newElements := make([]object.Object, n)
			for i, idx := range indices {
				newElements[i] = list.Elements[idx]
			}
			copy(list.Elements, newElements)
		}
		return NULL
	default:
		return errors.NewError("%s: list method %s not found", errors.ErrIdentifierNotFound, method)
	}
}

func callStringMethod(ctx context.Context, str *object.String, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch method {
	case "upper":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		return &object.String{Value: strings.ToUpper(str.Value)}
	case "lower":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		return &object.String{Value: strings.ToLower(str.Value)}
	case "split":
		if err := errors.MaxArgs(args, 2); err != nil { return err }
		// If no argument, split on whitespace
		if len(args) == 0 {
			parts := strings.Fields(str.Value)
			elements := make([]object.Object, len(parts))
			for i, part := range parts {
				elements[i] = &object.String{Value: part}
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
			// No maxsplit specified, use Split (splits all occurrences)
			parts = strings.Split(str.Value, sep)
		} else {
			// maxsplit specified - convert from scriptling Object to int
			maxsplitObj := args[1]
			maxsplit, err := maxsplitObj.AsInt()
			if err != nil {
				return errors.ParameterError("maxsplit", err)
			}
			// strings.SplitN takes n as max number of parts (maxsplit + 1 in Python terms)
			// If maxsplit is -1 (Python's default for unlimited), use -1
			n := int(maxsplit + 1)
			if maxsplit < 0 {
				n = -1
			}
			parts = strings.SplitN(str.Value, sep, n)
		}

		elements := make([]object.Object, len(parts))
		for i, part := range parts {
			elements[i] = &object.String{Value: part}
		}
		return &object.List{Elements: elements}
	case "replace":
		if err := errors.ExactArgs(args, 2); err != nil { return err }
		old, err := args[0].AsString()
		if err != nil {
			return err
		}
		newVal, err := args[1].AsString()
		if err != nil {
			return err
		}
		return &object.String{Value: strings.ReplaceAll(str.Value, old, newVal)}
	case "join":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		var elements []object.Object
		switch iter := args[0].(type) {
		case *object.List:
			elements = iter.Elements
		case *object.Tuple:
			elements = iter.Elements
		default:
			return errors.NewTypeError("LIST or TUPLE", args[0].Type().String())
		}
		parts := make([]string, len(elements))
		for i, elem := range elements {
			if s, err := elem.AsString(); err == nil {
				parts[i] = s
			} else {
				parts[i] = elem.Inspect()
			}
		}
		return &object.String{Value: strings.Join(parts, str.Value)}
	case "capitalize":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(str.Value) == 0 {
			return str
		}
		runes := []rune(str.Value)
		// Use strings.Builder for efficient string building
		var builder strings.Builder
		builder.Grow(len(runes))
		builder.WriteRune(unicode.ToUpper(runes[0]))
		for _, r := range runes[1:] {
			builder.WriteRune(unicode.ToLower(r))
		}
		return &object.String{Value: builder.String()}
	case "title":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		return &object.String{Value: cases.Title(language.Und).String(str.Value)}
	case "strip":
		if len(args) > 1 {
			return errors.NewError("strip() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			chars, errObj := args[0].AsString()
			if errObj != nil {
				return errors.ParameterError("chars", errObj)
			}
			return &object.String{Value: strings.Trim(str.Value, chars)}
		}
		return &object.String{Value: strings.TrimSpace(str.Value)}
	case "lstrip":
		if len(args) > 1 {
			return errors.NewError("lstrip() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			chars, errObj := args[0].AsString()
			if errObj != nil {
				return errors.ParameterError("chars", errObj)
			}
			return &object.String{Value: strings.TrimLeft(str.Value, chars)}
		}
		return &object.String{Value: strings.TrimLeft(str.Value, " \t\n\r\v\f")}
	case "rstrip":
		if len(args) > 1 {
			return errors.NewError("rstrip() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			chars, errObj := args[0].AsString()
			if errObj != nil {
				return errors.ParameterError("chars", errObj)
			}
			return &object.String{Value: strings.TrimRight(str.Value, chars)}
		}
		return &object.String{Value: strings.TrimRight(str.Value, " \t\n\r\v\f")}
	case "startswith":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		prefix, errObj := args[0].AsString()
		if errObj != nil {
			return errors.ParameterError("prefix", errObj)
		}
		return nativeBoolToBooleanObject(strings.HasPrefix(str.Value, prefix))
	case "endswith":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		suffix, errObj := args[0].AsString()
		if errObj != nil {
			return errors.ParameterError("suffix", errObj)
		}
		return nativeBoolToBooleanObject(strings.HasSuffix(str.Value, suffix))
	case "find":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("find() takes 1-3 arguments (%d given)", len(args))
		}
		substr, errObj := args[0].AsString()
		if errObj != nil {
			return errors.ParameterError("sub", errObj)
		}
		start := 0
		end := len(str.Value)
		if len(args) >= 2 {
			s, errObj2 := args[1].AsInt()
			if errObj2 != nil {
				return errors.ParameterError("start", errObj2)
			}
			start = int(s)
			if start < 0 {
				start = len(str.Value) + start
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
				end = len(str.Value) + end
			}
		}
		if start > len(str.Value) {
			start = len(str.Value)
		}
		if end > len(str.Value) {
			end = len(str.Value)
		}
		if start > end {
			return object.NewInteger(-1)
		}
		searchStr := str.Value[start:end]
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
		end := len(str.Value)
		if len(args) >= 2 {
			s, errObj2 := args[1].AsInt()
			if errObj2 != nil {
				return errors.ParameterError("start", errObj2)
			}
			start = int(s)
			if start < 0 {
				start = len(str.Value) + start
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
				end = len(str.Value) + end
			}
		}
		if start > len(str.Value) {
			start = len(str.Value)
		}
		if end > len(str.Value) {
			end = len(str.Value)
		}
		if start > end {
			return object.NewInteger(-1)
		}
		searchStr := str.Value[start:end]
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
		end := len(str.Value)
		if len(args) >= 2 {
			s, errObj2 := args[1].AsInt()
			if errObj2 != nil {
				return errors.ParameterError("start", errObj2)
			}
			start = int(s)
			if start < 0 {
				start = len(str.Value) + start
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
				end = len(str.Value) + end
			}
		}
		if start > len(str.Value) {
			start = len(str.Value)
		}
		if end > len(str.Value) {
			end = len(str.Value)
		}
		if start > end {
			return errors.NewError("substring not found")
		}
		searchStr := str.Value[start:end]
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
		end := len(str.Value)
		if len(args) >= 2 {
			s, errObj2 := args[1].AsInt()
			if errObj2 != nil {
				return errors.ParameterError("start", errObj2)
			}
			start = int(s)
			if start < 0 {
				start = len(str.Value) + start
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
				end = len(str.Value) + end
			}
		}
		if start > len(str.Value) {
			start = len(str.Value)
		}
		if end > len(str.Value) {
			end = len(str.Value)
		}
		if start > end {
			return errors.NewError("substring not found")
		}
		searchStr := str.Value[start:end]
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
		end := len(str.Value)
		if len(args) >= 2 {
			s, errObj2 := args[1].AsInt()
			if errObj2 != nil {
				return errors.ParameterError("start", errObj2)
			}
			start = int(s)
			if start < 0 {
				start = len(str.Value) + start
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
				end = len(str.Value) + end
			}
		}
		if start > len(str.Value) {
			start = len(str.Value)
		}
		if end > len(str.Value) {
			end = len(str.Value)
		}
		if start > end {
			return object.NewInteger(0)
		}
		searchStr := str.Value[start:end]
		return object.NewInteger(int64(strings.Count(searchStr, substr)))
	case "format":
		// Simple positional formatting: "{} {}".format("hello", "world")
		result := str.Value
		for i, arg := range args {
			placeholder := fmt.Sprintf("{%d}", i)
			result = strings.Replace(result, placeholder, arg.Inspect(), 1)
			// Also support {} for positional
			result = strings.Replace(result, "{}", arg.Inspect(), 1)
		}
		return &object.String{Value: result}
	case "isdigit":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			if ch < '0' || ch > '9' {
				return FALSE
			}
		}
		return TRUE
	case "isalpha":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) {
				return FALSE
			}
		}
		return TRUE
	case "isalnum":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
				return FALSE
			}
		}
		return TRUE
	case "isspace":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' && ch != '\v' && ch != '\f' {
				return FALSE
			}
		}
		return TRUE
	case "isupper":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		hasUpper := false
		for _, ch := range str.Value {
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
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		hasLower := false
		for _, ch := range str.Value {
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
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		width, errObj := args[0].AsInt()
		if errObj != nil {
			return errors.ParameterError("width", errObj)
		}
		w := int(width)
		if w <= len(str.Value) {
			return str
		}
		// Handle negative sign
		if len(str.Value) > 0 && (str.Value[0] == '-' || str.Value[0] == '+') {
			var builder strings.Builder
			builder.Grow(w)
			builder.WriteByte(str.Value[0])
			builder.WriteString(strings.Repeat("0", w-len(str.Value)))
			builder.WriteString(str.Value[1:])
			return &object.String{Value: builder.String()}
		}
		// Simple case - just pad with zeros
		var builder strings.Builder
		builder.Grow(w)
		builder.WriteString(strings.Repeat("0", w-len(str.Value)))
		builder.WriteString(str.Value)
		return &object.String{Value: builder.String()}
	case "center":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("center() takes 1-2 arguments (%d given)", len(args))
		}
		width, errObj := args[0].AsInt()
		if errObj != nil {
			return errors.ParameterError("width", errObj)
		}
		w := int(width)
		if w <= len(str.Value) {
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
		padding := w - len(str.Value)
		leftPad := padding / 2
		rightPad := padding - leftPad
		// Use strings.Builder for efficient concatenation
		var builder strings.Builder
		builder.Grow(w)
		builder.WriteString(strings.Repeat(fillChar, leftPad))
		builder.WriteString(str.Value)
		builder.WriteString(strings.Repeat(fillChar, rightPad))
		return &object.String{Value: builder.String()}
	case "ljust":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("ljust() takes 1-2 arguments (%d given)", len(args))
		}
		width, errObj := args[0].AsInt()
		if errObj != nil {
			return errors.ParameterError("width", errObj)
		}
		w := int(width)
		if w <= len(str.Value) {
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
		builder.WriteString(str.Value)
		builder.WriteString(strings.Repeat(fillChar, w-len(str.Value)))
		return &object.String{Value: builder.String()}
	case "rjust":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("rjust() takes 1-2 arguments (%d given)", len(args))
		}
		width, errObj := args[0].AsInt()
		if errObj != nil {
			return errors.ParameterError("width", errObj)
		}
		w := int(width)
		if w <= len(str.Value) {
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
		builder.WriteString(strings.Repeat(fillChar, w-len(str.Value)))
		builder.WriteString(str.Value)
		return &object.String{Value: builder.String()}
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
		}
		lines := []object.Object{}
		text := str.Value
		start := 0
		for i := 0; i < len(text); i++ {
			if text[i] == '\n' {
				if keepends {
					lines = append(lines, &object.String{Value: text[start : i+1]})
				} else {
					lines = append(lines, &object.String{Value: text[start:i]})
				}
				start = i + 1
			} else if text[i] == '\r' {
				end := i
				if i+1 < len(text) && text[i+1] == '\n' {
					i++
				}
				if keepends {
					lines = append(lines, &object.String{Value: text[start : i+1]})
				} else {
					lines = append(lines, &object.String{Value: text[start:end]})
				}
				start = i + 1
			}
		}
		if start < len(text) {
			lines = append(lines, &object.String{Value: text[start:]})
		}
		return &object.List{Elements: lines}
	case "swapcase":
		if len(args) != 0 {
			return errors.NewError("swapcase() takes no arguments (%d given)", len(args))
		}
		result := make([]rune, len(str.Value))
		for i, r := range str.Value {
			if r >= 'A' && r <= 'Z' {
				result[i] = r + 32
			} else if r >= 'a' && r <= 'z' {
				result[i] = r - 32
			} else {
				result[i] = r
			}
		}
		return &object.String{Value: string(result)}
	case "partition":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		sep, err := args[0].AsString()
		if err != nil {
			return err
		}
		idx := strings.Index(str.Value, sep)
		if idx < 0 {
			return &object.Tuple{Elements: []object.Object{
				str,
				&object.String{Value: ""},
				&object.String{Value: ""},
			}}
		}
		return &object.Tuple{Elements: []object.Object{
			&object.String{Value: str.Value[:idx]},
			&object.String{Value: sep},
			&object.String{Value: str.Value[idx+len(sep):]},
		}}
	case "rpartition":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		sep, err := args[0].AsString()
		if err != nil {
			return err
		}
		idx := strings.LastIndex(str.Value, sep)
		if idx < 0 {
			return &object.Tuple{Elements: []object.Object{
				&object.String{Value: ""},
				&object.String{Value: ""},
				str,
			}}
		}
		return &object.Tuple{Elements: []object.Object{
			&object.String{Value: str.Value[:idx]},
			&object.String{Value: sep},
			&object.String{Value: str.Value[idx+len(sep):]},
		}}
	case "removeprefix":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		prefix, err := args[0].AsString()
		if err != nil {
			return err
		}
		if strings.HasPrefix(str.Value, prefix) {
			return &object.String{Value: str.Value[len(prefix):]}
		}
		return str
	case "removesuffix":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		suffix, err := args[0].AsString()
		if err != nil {
			return err
		}
		if strings.HasSuffix(str.Value, suffix) {
			return &object.String{Value: str.Value[:len(str.Value)-len(suffix)]}
		}
		return str
	case "encode":
		if len(args) > 1 {
			return errors.NewError("encode() takes at most 1 argument (%d given)", len(args))
		}
		// In Scriptling, encode just returns a list of byte values
		// as we don't have a bytes type
		bytes := []object.Object{}
		for _, b := range []byte(str.Value) {
			bytes = append(bytes, object.NewInteger(int64(b)))
		}
		return &object.List{Elements: bytes}
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
		for _, ch := range str.Value {
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
		return &object.String{Value: result.String()}
	case "casefold":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		// casefold is more aggressive than lower() for Unicode
		// For ASCII, it's equivalent to lower()
		return &object.String{Value: strings.ToLower(str.Value)}
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
				transMap.Pairs[object.DictKey(&object.String{Value: k})] = object.DictPair{Key: &object.String{Value: k}, Value: v}
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
			key := object.DictKey(&object.String{Value: string(ch)})
			transMap.Pairs[key] = object.DictPair{
				Key:   &object.String{Value: string(ch)},
				Value: &object.String{Value: string(toRunes[i])},
			}
		}
		// Third argument: characters to delete
		if len(args) == 3 {
			del, errDel := args[2].AsString()
			if errDel != nil {
				return errors.ParameterError("deletechars", errDel)
			}
			for _, ch := range del {
				key := object.DictKey(&object.String{Value: string(ch)})
				transMap.Pairs[key] = object.DictPair{
					Key:   &object.String{Value: string(ch)},
					Value: NULL,
				}
			}
		}
		return transMap
	case "translate":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		transMap, ok := args[0].(*object.Dict)
		if !ok {
			return errors.NewTypeError("DICT", args[0].Type().String())
		}
		var result strings.Builder
		for _, ch := range str.Value {
			key := object.DictKey(&object.String{Value: string(ch)})
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
		return &object.String{Value: result.String()}
	case "isnumeric":
		// Returns True if all characters are numeric (0-9, superscripts, fractions, etc.)
		// For simplicity, we check for Unicode numeric characters
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			// Check if character is in Unicode numeric categories
			if !unicode.IsNumber(ch) {
				return FALSE
			}
		}
		return TRUE
	case "isdecimal":
		// Returns True if all characters are decimal digits (0-9)
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			if ch < '0' || ch > '9' {
				return FALSE
			}
		}
		return TRUE
	case "istitle":
		// Returns True if string is titlecased
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(str.Value) == 0 {
			return FALSE
		}
		// Title case: first char of each word is uppercase, rest are lowercase
		prevCased := false
		hasCased := false
		for _, ch := range str.Value {
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
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(str.Value) == 0 {
			return FALSE
		}
		for i, ch := range str.Value {
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
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		// Empty string is considered printable
		for _, ch := range str.Value {
			if !unicode.IsPrint(ch) && ch != ' ' {
				return FALSE
			}
		}
		return TRUE
	default:
		return errors.NewError("%s: %s", errors.ErrIdentifierNotFound, method)
	}
}
func callSetMethod(ctx context.Context, set *object.Set, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch method {
	case "add":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		set.Add(args[0])
		return NULL
	case "remove":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		if !set.Remove(args[0]) {
			return errors.NewError("KeyError: %s", args[0].Inspect())
		}
		return NULL
	case "discard":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		set.Remove(args[0])
		return NULL
	case "pop":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		if len(set.Elements) == 0 {
			return errors.NewError("pop from an empty set")
		}
		// Go map iteration order is random, which matches Python's arbitrary pop
		for _, elem := range set.Elements {
			set.Remove(elem)
			return elem
		}
	case "clear":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		set.Elements = make(map[string]object.Object)
		return NULL
	case "copy":
		if err := errors.ExactArgs(args, 0); err != nil { return err }
		return set.Copy()
	case "union":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		if other, ok := args[0].(*object.Set); ok {
			return set.Union(other)
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	case "intersection":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		if other, ok := args[0].(*object.Set); ok {
			return set.Intersection(other)
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	case "difference":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		if other, ok := args[0].(*object.Set); ok {
			return set.Difference(other)
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	case "symmetric_difference":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		if other, ok := args[0].(*object.Set); ok {
			return set.SymmetricDifference(other)
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	case "issubset":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		if other, ok := args[0].(*object.Set); ok {
			return nativeBoolToBooleanObject(set.IsSubset(other))
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	case "issuperset":
		if err := errors.ExactArgs(args, 1); err != nil { return err }
		if other, ok := args[0].(*object.Set); ok {
			return nativeBoolToBooleanObject(set.IsSuperset(other))
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	default:
		return errors.NewError("%s: set method %s not found", errors.ErrIdentifierNotFound, method)
	}
	return NULL
}
