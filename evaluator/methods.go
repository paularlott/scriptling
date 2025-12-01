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
)

func evalMethodCallExpression(ctx context.Context, mce *ast.MethodCallExpression, env *object.Environment) object.Object {
	obj := evalWithContext(ctx, mce.Object, env)
	if isError(obj) {
		return obj
	}

	args := evalExpressionsWithContext(ctx, mce.Arguments, env)
	if len(args) == 1 && isError(args[0]) {
		return args[0]
	}

	// Evaluate keyword arguments
	keywords := make(map[string]object.Object)
	for k, v := range mce.Keywords {
		val := evalWithContext(ctx, v, env)
		if isError(val) {
			return val
		}
		keywords[k] = val
	}

	return callStringMethodWithKeywords(ctx, obj, mce.Method.Value, args, keywords, env)
}

func callStringMethodWithKeywords(ctx context.Context, obj object.Object, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Handle universal methods
	switch method {
	case "type":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(keywords) > 0 {
			return errors.NewError("type() does not accept keyword arguments")
		}
		return &object.String{Value: obj.Type().String()}
	}

	// Handle library method calls (dictionaries)
	if obj.Type() == object.DICT_OBJ {
		return callDictMethod(ctx, obj.(*object.Dict), method, args, keywords, env)
	}

	// Handle datetime methods
	if obj.Type() == object.DATETIME_OBJ {
		return callDatetimeMethod(ctx, obj.(*object.Datetime), method, args, keywords, env)
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
			// If it's a function, bind 'self' to the instance
			if f, ok := fn.(*object.Function); ok {
				newArgs := append([]object.Object{super.Instance}, args...)
				return applyFunctionWithContext(ctx, f, newArgs, keywords, env)
			}
			// If it's a builtin, call it (builtins in classes are usually static-like or expect explicit self if they are methods)
			// But for now, let's assume if it's in a class, it might be a method.
			// However, our builtins don't support 'self' binding automatically unless wrapped.
			// If it's just a value (like a class variable), we can't "call" it here because this is evalMethodCallExpression.
			// But evalMethodCallExpression expects the result to be the result of the call.
			// If fn is not callable, we should probably error or try to call it if it has a __call__?
			// For now, let's just try to apply it.
			return applyFunctionWithContext(ctx, fn, args, keywords, env)
		}
		currentClass = currentClass.BaseClass
	}

	return errors.NewError("super object has no method %s", method)
}

func callInstanceMethod(ctx context.Context, instance *object.Instance, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Check class methods
	if fn, ok := instance.Class.Methods[method]; ok {
		// Bind 'self'
		newArgs := append([]object.Object{instance}, args...)
		return applyFunctionWithContext(ctx, fn, newArgs, keywords, env)
	}

	return errors.NewError("instance has no method %s", method)
}

func callDictMethod(ctx context.Context, dict *object.Dict, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// First check for library methods (callable functions stored in dict)
	// This takes priority over dict instance methods like get, pop, etc.
	if pair, ok := dict.Pairs[method]; ok {
		switch fn := pair.Value.(type) {
		case *object.Builtin:
			ctxWithEnv := SetEnvInContext(ctx, env)
			return fn.Fn(ctxWithEnv, keywords, args...)
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(keywords) > 0 {
			return errors.NewError("keys() does not accept keyword arguments")
		}
		if builtin, ok := builtins["keys"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, dict)
		}
	case "values":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(keywords) > 0 {
			return errors.NewError("values() does not accept keyword arguments")
		}
		if builtin, ok := builtins["values"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, dict)
		}
	case "items":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(keywords) > 0 {
			return errors.NewError("items() does not accept keyword arguments")
		}
		if builtin, ok := builtins["items"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, dict)
		}
	case "get":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("get() takes 1-2 arguments (%d given)", len(args))
		}
		if len(keywords) > 0 {
			return errors.NewError("get() does not accept keyword arguments")
		}
		key := args[0].Inspect()
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
		key := args[0].Inspect()
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
			dict.Pairs[k] = object.DictPair{Key: &object.String{Value: k}, Value: v}
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
					dict.Pairs[pair[0].Inspect()] = object.DictPair{Key: pair[0], Value: pair[1]}
				}
			default:
				return errors.NewTypeError("DICT or LIST of pairs", args[0].Type().String())
			}
		}
		return NULL
	case "clear":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(keywords) > 0 {
			return errors.NewError("clear() does not accept keyword arguments")
		}
		dict.Pairs = make(map[string]object.DictPair)
		return NULL
	case "copy":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
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
		key := args[0].Inspect()
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
				key := elem.Inspect()
				newPairs[key] = object.DictPair{Key: elem, Value: defaultVal}
			}
		case *object.Tuple:
			for _, elem := range iter.Elements {
				key := elem.Inspect()
				newPairs[key] = object.DictPair{Key: elem, Value: defaultVal}
			}
		case *object.String:
			for _, ch := range iter.Value {
				s := string(ch)
				newPairs[s] = object.DictPair{Key: &object.String{Value: s}, Value: defaultVal}
			}
		default:
			return errors.NewTypeError("iterable (LIST, TUPLE, STRING)", args[0].Type().String())
		}
		return &object.Dict{Pairs: newPairs}
	}

	// Check for non-callable dict values (for accessing dict attributes)
	if pair, ok := dict.Pairs[method]; ok {
		// If it's not a callable, just return the value
		if len(args) == 0 && len(keywords) == 0 {
			return pair.Value
		}
		return errors.NewError("%s: %s is not callable", errors.ErrIdentifierNotFound, method)
	}
	return errors.NewError("%s: method %s not found in library", errors.ErrIdentifierNotFound, method)
}

func callDatetimeMethod(ctx context.Context, dt *object.Datetime, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch method {
	case "timestamp":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(keywords) > 0 {
			return errors.NewError("timestamp() does not accept keyword arguments")
		}
		return &object.Float{Value: float64(dt.Value.Unix())}
	default:
		return errors.NewError("datetime object has no method %s", method)
	}
}

func callListMethod(ctx context.Context, list *object.List, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch method {
	case "append":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		list.Elements = append(list.Elements, args[0])
		return NULL
	case "extend":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if other, ok := args[0].(*object.List); ok {
			list.Elements = append(list.Elements, other.Elements...)
			return NULL
		}
		return errors.NewTypeError("LIST", args[0].Type().String())
	case "index":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("index() takes 1-3 arguments (%d given)", len(args))
		}
		value := args[0]
		start := 0
		end := len(list.Elements)
		if len(args) >= 2 {
			if s, ok := args[1].(*object.Integer); ok {
				start = int(s.Value)
				if start < 0 {
					start = len(list.Elements) + start
					if start < 0 {
						start = 0
					}
				}
			} else {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
		}
		if len(args) == 3 {
			if e, ok := args[2].(*object.Integer); ok {
				end = int(e.Value)
				if end < 0 {
					end = len(list.Elements) + end
				}
			} else {
				return errors.NewTypeError("INTEGER", args[2].Type().String())
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
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
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
			if i, ok := args[0].(*object.Integer); ok {
				idx = int(i.Value)
				if idx < 0 {
					idx = len(list.Elements) + idx
				}
				if idx < 0 || idx >= len(list.Elements) {
					return errors.NewError("pop index out of range")
				}
			} else {
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}
		}
		result := list.Elements[idx]
		list.Elements = append(list.Elements[:idx], list.Elements[idx+1:]...)
		return result
	case "insert":
		if len(args) != 2 {
			return errors.NewArgumentError(len(args), 2)
		}
		if idx, ok := args[0].(*object.Integer); ok {
			i := int(idx.Value)
			if i < 0 {
				i = len(list.Elements) + i + 1
				if i < 0 {
					i = 0
				}
			}
			if i > len(list.Elements) {
				i = len(list.Elements)
			}
			list.Elements = append(list.Elements[:i], append([]object.Object{args[1]}, list.Elements[i:]...)...)
			return NULL
		}
		return errors.NewTypeError("INTEGER", args[0].Type().String())
	case "remove":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		value := args[0]
		for i, elem := range list.Elements {
			if objectsEqual(elem, value) {
				list.Elements = append(list.Elements[:i], list.Elements[i+1:]...)
				return NULL
			}
		}
		return errors.NewError("value not in list")
	case "clear":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		list.Elements = []object.Object{}
		return NULL
	case "copy":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		elements := make([]object.Object, len(list.Elements))
		copy(elements, list.Elements)
		return &object.List{Elements: elements}
	case "reverse":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		for i, j := 0, len(list.Elements)-1; i < j; i, j = i+1, j-1 {
			list.Elements[i], list.Elements[j] = list.Elements[j], list.Elements[i]
		}
		return NULL
	case "sort":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		// Check for key and reverse kwargs
		var keyFunc object.Object
		reverse := false
		if keywords != nil {
			if kf, ok := keywords["key"]; ok {
				keyFunc = kf
			}
			if rev, ok := keywords["reverse"]; ok {
				if b, ok := rev.(*object.Boolean); ok {
					reverse = b.Value
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
					if isError(key) {
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["upper"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "lower":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["lower"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "split":
		if len(args) > 1 {
			return errors.NewArgumentError(len(args), 1)
		}
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
		if builtin, ok := builtins["split"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str, args[0])
		}
	case "replace":
		if len(args) != 2 {
			return errors.NewArgumentError(len(args), 2)
		}
		if builtin, ok := builtins["replace"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str, args[0], args[1])
		}
	case "join":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if builtin, ok := builtins["join"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			// join builtin expects (list, separator), but method is separator.join(list)
			return builtin.Fn(ctxWithEnv, nil, args[0], str)
		}
	case "capitalize":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["capitalize"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "title":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["title"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "strip":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["strip"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "lstrip":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["lstrip"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "rstrip":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["rstrip"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "startswith":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if builtin, ok := builtins["startswith"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str, args[0])
		}
	case "endswith":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if builtin, ok := builtins["endswith"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str, args[0])
		}
	case "find":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("find() takes 1-3 arguments (%d given)", len(args))
		}
		if substr, ok := args[0].(*object.String); ok {
			start := 0
			end := len(str.Value)
			if len(args) >= 2 {
				if s, ok := args[1].(*object.Integer); ok {
					start = int(s.Value)
					if start < 0 {
						start = len(str.Value) + start
						if start < 0 {
							start = 0
						}
					}
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if len(args) == 3 {
				if e, ok := args[2].(*object.Integer); ok {
					end = int(e.Value)
					if end < 0 {
						end = len(str.Value) + end
					}
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
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
			idx := strings.Index(searchStr, substr.Value)
			if idx == -1 {
				return object.NewInteger(-1)
			}
			return object.NewInteger(int64(start + idx))
		}
		return errors.NewTypeError("STRING", args[0].Type().String())
	case "rfind":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("rfind() takes 1-3 arguments (%d given)", len(args))
		}
		if substr, ok := args[0].(*object.String); ok {
			start := 0
			end := len(str.Value)
			if len(args) >= 2 {
				if s, ok := args[1].(*object.Integer); ok {
					start = int(s.Value)
					if start < 0 {
						start = len(str.Value) + start
						if start < 0 {
							start = 0
						}
					}
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if len(args) == 3 {
				if e, ok := args[2].(*object.Integer); ok {
					end = int(e.Value)
					if end < 0 {
						end = len(str.Value) + end
					}
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
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
			idx := strings.LastIndex(searchStr, substr.Value)
			if idx == -1 {
				return object.NewInteger(-1)
			}
			return object.NewInteger(int64(start + idx))
		}
		return errors.NewTypeError("STRING", args[0].Type().String())
	case "rindex":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("rindex() takes 1-3 arguments (%d given)", len(args))
		}
		if substr, ok := args[0].(*object.String); ok {
			start := 0
			end := len(str.Value)
			if len(args) >= 2 {
				if s, ok := args[1].(*object.Integer); ok {
					start = int(s.Value)
					if start < 0 {
						start = len(str.Value) + start
						if start < 0 {
							start = 0
						}
					}
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if len(args) == 3 {
				if e, ok := args[2].(*object.Integer); ok {
					end = int(e.Value)
					if end < 0 {
						end = len(str.Value) + end
					}
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
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
			idx := strings.LastIndex(searchStr, substr.Value)
			if idx == -1 {
				return errors.NewError("substring not found")
			}
			return object.NewInteger(int64(start + idx))
		}
		return errors.NewTypeError("STRING", args[0].Type().String())
	case "index":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("index() takes 1-3 arguments (%d given)", len(args))
		}
		if substr, ok := args[0].(*object.String); ok {
			start := 0
			end := len(str.Value)
			if len(args) >= 2 {
				if s, ok := args[1].(*object.Integer); ok {
					start = int(s.Value)
					if start < 0 {
						start = len(str.Value) + start
						if start < 0 {
							start = 0
						}
					}
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if len(args) == 3 {
				if e, ok := args[2].(*object.Integer); ok {
					end = int(e.Value)
					if end < 0 {
						end = len(str.Value) + end
					}
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
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
			idx := strings.Index(searchStr, substr.Value)
			if idx == -1 {
				return errors.NewError("substring not found")
			}
			return object.NewInteger(int64(start + idx))
		}
		return errors.NewTypeError("STRING", args[0].Type().String())
	case "count":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("count() takes 1-3 arguments (%d given)", len(args))
		}
		if substr, ok := args[0].(*object.String); ok {
			start := 0
			end := len(str.Value)
			if len(args) >= 2 {
				if s, ok := args[1].(*object.Integer); ok {
					start = int(s.Value)
					if start < 0 {
						start = len(str.Value) + start
						if start < 0 {
							start = 0
						}
					}
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if len(args) == 3 {
				if e, ok := args[2].(*object.Integer); ok {
					end = int(e.Value)
					if end < 0 {
						end = len(str.Value) + end
					}
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
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
			return object.NewInteger(int64(strings.Count(searchStr, substr.Value)))
		}
		return errors.NewTypeError("STRING", args[0].Type().String())
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if width, ok := args[0].(*object.Integer); ok {
			w := int(width.Value)
			if w <= len(str.Value) {
				return str
			}
			// Handle negative sign
			if len(str.Value) > 0 && (str.Value[0] == '-' || str.Value[0] == '+') {
				return &object.String{Value: string(str.Value[0]) + strings.Repeat("0", w-len(str.Value)) + str.Value[1:]}
			}
			return &object.String{Value: strings.Repeat("0", w-len(str.Value)) + str.Value}
		}
		return errors.NewTypeError("INTEGER", args[0].Type().String())
	case "center":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("center() takes 1-2 arguments (%d given)", len(args))
		}
		if width, ok := args[0].(*object.Integer); ok {
			w := int(width.Value)
			if w <= len(str.Value) {
				return str
			}
			fillChar := " "
			if len(args) == 2 {
				if fill, ok := args[1].(*object.String); ok {
					if len(fill.Value) != 1 {
						return errors.NewError("fill character must be exactly one character")
					}
					fillChar = fill.Value
				} else {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
			}
			padding := w - len(str.Value)
			leftPad := padding / 2
			rightPad := padding - leftPad
			return &object.String{Value: strings.Repeat(fillChar, leftPad) + str.Value + strings.Repeat(fillChar, rightPad)}
		}
		return errors.NewTypeError("INTEGER", args[0].Type().String())
	case "ljust":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("ljust() takes 1-2 arguments (%d given)", len(args))
		}
		if width, ok := args[0].(*object.Integer); ok {
			w := int(width.Value)
			if w <= len(str.Value) {
				return str
			}
			fillChar := " "
			if len(args) == 2 {
				if fill, ok := args[1].(*object.String); ok {
					if len(fill.Value) != 1 {
						return errors.NewError("fill character must be exactly one character")
					}
					fillChar = fill.Value
				} else {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
			}
			return &object.String{Value: str.Value + strings.Repeat(fillChar, w-len(str.Value))}
		}
		return errors.NewTypeError("INTEGER", args[0].Type().String())
	case "rjust":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("rjust() takes 1-2 arguments (%d given)", len(args))
		}
		if width, ok := args[0].(*object.Integer); ok {
			w := int(width.Value)
			if w <= len(str.Value) {
				return str
			}
			fillChar := " "
			if len(args) == 2 {
				if fill, ok := args[1].(*object.String); ok {
					if len(fill.Value) != 1 {
						return errors.NewError("fill character must be exactly one character")
					}
					fillChar = fill.Value
				} else {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
			}
			return &object.String{Value: strings.Repeat(fillChar, w-len(str.Value)) + str.Value}
		}
		return errors.NewTypeError("INTEGER", args[0].Type().String())
	case "splitlines":
		keepends := false
		if len(args) > 1 {
			return errors.NewError("splitlines() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			if b, ok := args[0].(*object.Boolean); ok {
				keepends = b.Value
			} else {
				return errors.NewTypeError("BOOLEAN", args[0].Type().String())
			}
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
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		sep, ok := args[0].(*object.String)
		if !ok {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}
		idx := strings.Index(str.Value, sep.Value)
		if idx < 0 {
			return &object.Tuple{Elements: []object.Object{
				str,
				&object.String{Value: ""},
				&object.String{Value: ""},
			}}
		}
		return &object.Tuple{Elements: []object.Object{
			&object.String{Value: str.Value[:idx]},
			sep,
			&object.String{Value: str.Value[idx+len(sep.Value):]},
		}}
	case "rpartition":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		sep, ok := args[0].(*object.String)
		if !ok {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}
		idx := strings.LastIndex(str.Value, sep.Value)
		if idx < 0 {
			return &object.Tuple{Elements: []object.Object{
				&object.String{Value: ""},
				&object.String{Value: ""},
				str,
			}}
		}
		return &object.Tuple{Elements: []object.Object{
			&object.String{Value: str.Value[:idx]},
			sep,
			&object.String{Value: str.Value[idx+len(sep.Value):]},
		}}
	case "removeprefix":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		prefix, ok := args[0].(*object.String)
		if !ok {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}
		if strings.HasPrefix(str.Value, prefix.Value) {
			return &object.String{Value: str.Value[len(prefix.Value):]}
		}
		return str
	case "removesuffix":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		suffix, ok := args[0].(*object.String)
		if !ok {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}
		if strings.HasSuffix(str.Value, suffix.Value) {
			return &object.String{Value: str.Value[:len(str.Value)-len(suffix.Value)]}
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
			if ts, ok := args[0].(*object.Integer); ok {
				tabsize = int(ts.Value)
			} else {
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
			if d, ok := args[0].(*object.Dict); ok {
				for k, v := range d.Pairs {
					transMap.Pairs[k] = v
				}
				return transMap
			}
			return errors.NewTypeError("DICT", args[0].Type().String())
		}
		// Two arguments: from and to strings
		from, okFrom := args[0].(*object.String)
		to, okTo := args[1].(*object.String)
		if !okFrom || !okTo {
			return errors.NewError("maketrans() arguments must be strings")
		}
		fromRunes := []rune(from.Value)
		toRunes := []rune(to.Value)
		if len(fromRunes) != len(toRunes) {
			return errors.NewError("maketrans() arguments must have equal length")
		}
		for i, ch := range fromRunes {
			key := string(ch)
			transMap.Pairs[key] = object.DictPair{
				Key:   &object.String{Value: key},
				Value: &object.String{Value: string(toRunes[i])},
			}
		}
		// Third argument: characters to delete
		if len(args) == 3 {
			if del, ok := args[2].(*object.String); ok {
				for _, ch := range del.Value {
					key := string(ch)
					transMap.Pairs[key] = object.DictPair{
						Key:   &object.String{Value: key},
						Value: NULL,
					}
				}
			} else {
				return errors.NewTypeError("STRING", args[2].Type().String())
			}
		}
		return transMap
	case "translate":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		transMap, ok := args[0].(*object.Dict)
		if !ok {
			return errors.NewTypeError("DICT", args[0].Type().String())
		}
		var result strings.Builder
		for _, ch := range str.Value {
			key := string(ch)
			if pair, exists := transMap.Pairs[key]; exists {
				if pair.Value == NULL || pair.Value.Type() == object.NULL_OBJ {
					// Delete character
					continue
				}
				if s, ok := pair.Value.(*object.String); ok {
					result.WriteString(s.Value)
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
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
	return errors.NewError("%s: %s", errors.ErrIdentifierNotFound, method)
}
func callSetMethod(ctx context.Context, set *object.Set, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch method {
	case "add":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		set.Add(args[0])
		return NULL
	case "remove":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if !set.Remove(args[0]) {
			return errors.NewError("KeyError: %s", args[0].Inspect())
		}
		return NULL
	case "discard":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		set.Remove(args[0])
		return NULL
	case "pop":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(set.Elements) == 0 {
			return errors.NewError("pop from an empty set")
		}
		// Go map iteration order is random, which matches Python's arbitrary pop
		for _, elem := range set.Elements {
			set.Remove(elem)
			return elem
		}
	case "clear":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		set.Elements = make(map[string]object.Object)
		return NULL
	case "copy":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		return set.Copy()
	case "union":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if other, ok := args[0].(*object.Set); ok {
			return set.Union(other)
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	case "intersection":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if other, ok := args[0].(*object.Set); ok {
			return set.Intersection(other)
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	case "difference":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if other, ok := args[0].(*object.Set); ok {
			return set.Difference(other)
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	case "symmetric_difference":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if other, ok := args[0].(*object.Set); ok {
			return set.SymmetricDifference(other)
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	case "issubset":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if other, ok := args[0].(*object.Set); ok {
			return nativeBoolToBooleanObject(set.IsSubset(other))
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	case "issuperset":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if other, ok := args[0].(*object.Set); ok {
			return nativeBoolToBooleanObject(set.IsSuperset(other))
		}
		return errors.NewTypeError("SET", args[0].Type().String())
	default:
		return errors.NewError("%s: set method %s not found", errors.ErrIdentifierNotFound, method)
	}
	return NULL
}
