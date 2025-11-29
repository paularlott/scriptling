package evaluator

import (
	"context"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

func evalDictLiteralWithContext(ctx context.Context, node *ast.DictLiteral, env *object.Environment) object.Object {
	if len(node.Pairs) == 0 {
		return &object.Dict{Pairs: make(map[string]object.DictPair)}
	}
	pairs := make(map[string]object.DictPair, len(node.Pairs))

	for keyNode, valueNode := range node.Pairs {
		key := evalWithContext(ctx, keyNode, env)
		if isError(key) {
			return key
		}

		value := evalWithContext(ctx, valueNode, env)
		if isError(value) {
			return value
		}

		pairs[key.Inspect()] = object.DictPair{Key: key, Value: value}
	}

	return &object.Dict{Pairs: pairs}
}

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.LIST_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalListIndexExpression(left, index)
	case left.Type() == object.TUPLE_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalTupleIndexExpression(left, index)
	case left.Type() == object.DICT_OBJ:
		return evalDictIndexExpression(left, index)
	case left.Type() == object.STRING_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalStringIndexExpression(left, index)
	case left.Type() == object.INSTANCE_OBJ:
		return evalInstanceIndexExpression(left, index)
	case left.Type() == object.CLASS_OBJ:
		return evalClassIndexExpression(left, index)
	case left.Type() == object.BUILTIN_OBJ:
		return evalBuiltinIndexExpression(left, index)
	default:
		return errors.NewError("index operator not supported: %s", left.Type())
	}
}

func evalDictMemberAccess(dict *object.Dict, member string) object.Object {
	pair, ok := dict.Pairs[member]
	if !ok {
		return NULL
	}
	return pair.Value
}

func evalListIndexExpression(list, index object.Object) object.Object {
	listObject := list.(*object.List)
	idx := index.(*object.Integer).Value
	length := int64(len(listObject.Elements))

	// Handle negative indices
	if idx < 0 {
		idx += length
	}

	if idx < 0 || idx >= length {
		return NULL
	}

	return listObject.Elements[idx]
}

func evalTupleIndexExpression(tuple, index object.Object) object.Object {
	tupleObject := tuple.(*object.Tuple)
	idx := index.(*object.Integer).Value
	length := int64(len(tupleObject.Elements))

	// Handle negative indices
	if idx < 0 {
		idx += length
	}

	if idx < 0 || idx >= length {
		return NULL
	}

	return tupleObject.Elements[idx]
}

func evalDictIndexExpression(dict, index object.Object) object.Object {
	dictObject := dict.(*object.Dict)
	key := index.Inspect()

	pair, ok := dictObject.Pairs[key]
	if !ok {
		return NULL
	}

	return pair.Value
}

func evalStringIndexExpression(str, index object.Object) object.Object {
	strObject := str.(*object.String)
	idx := index.(*object.Integer).Value
	length := int64(len(strObject.Value))

	// Handle negative indices
	if idx < 0 {
		idx += length
	}

	if idx < 0 || idx >= length {
		return NULL
	}

	return &object.String{Value: string(strObject.Value[idx])}
}

func evalInstanceIndexExpression(instance, index object.Object) object.Object {
	if index.Type() != object.STRING_OBJ {
		return errors.NewError("instance index must be string")
	}
	field := index.(*object.String).Value
	inst := instance.(*object.Instance)
	if val, ok := inst.Fields[field]; ok {
		return val
	}
	// Check class methods
	if fn, ok := inst.Class.Methods[field]; ok {
		return fn
	}
	return NULL
}

func evalClassIndexExpression(class, index object.Object) object.Object {
	if index.Type() != object.STRING_OBJ {
		return errors.NewError("class index must be string")
	}
	field := index.(*object.String).Value
	cl := class.(*object.Class)
	if fn, ok := cl.Methods[field]; ok {
		return fn
	}
	return NULL
}

func evalBuiltinIndexExpression(builtin, index object.Object) object.Object {
	if index.Type() != object.STRING_OBJ {
		return errors.NewError("builtin index must be string")
	}
	field := index.(*object.String).Value
	b := builtin.(*object.Builtin)
	if b.Attributes != nil {
		if val, ok := b.Attributes[field]; ok {
			return val
		}
	}
	return NULL
}

func evalSliceExpressionWithContext(ctx context.Context, node *ast.SliceExpression, env *object.Environment) object.Object {
	left := evalWithContext(ctx, node.Left, env)
	if isError(left) {
		return left
	}

	var start, end, step int64
	var hasStart, hasEnd, hasStep bool
	step = 1 // default step

	if node.Start != nil {
		startObj := evalWithContext(ctx, node.Start, env)
		if isError(startObj) {
			return startObj
		}
		if startObj.Type() != object.INTEGER_OBJ {
			return errors.NewTypeError("INTEGER", startObj.Type().String())
		}
		start = startObj.(*object.Integer).Value
		hasStart = true
	}

	if node.End != nil {
		endObj := evalWithContext(ctx, node.End, env)
		if isError(endObj) {
			return endObj
		}
		if endObj.Type() != object.INTEGER_OBJ {
			return errors.NewTypeError("INTEGER", endObj.Type().String())
		}
		end = endObj.(*object.Integer).Value
		hasEnd = true
	}

	if node.Step != nil {
		stepObj := evalWithContext(ctx, node.Step, env)
		if isError(stepObj) {
			return stepObj
		}
		if stepObj.Type() != object.INTEGER_OBJ {
			return errors.NewTypeError("INTEGER", stepObj.Type().String())
		}
		step = stepObj.(*object.Integer).Value
		hasStep = true
		if step == 0 {
			return errors.NewError("slice step cannot be zero")
		}
	}

	switch obj := left.(type) {
	case *object.List:
		return sliceList(obj.Elements, start, end, step, hasStart, hasEnd, hasStep)
	case *object.String:
		elements := sliceString(obj.Value, start, end, step, hasStart, hasEnd, hasStep)
		return &object.String{Value: elements}
	default:
		return errors.NewError("slice operator not supported: %s", left.Type())
	}
}

func sliceList(elements []object.Object, start, end, step int64, hasStart, hasEnd, hasStep bool) object.Object {
	length := int64(len(elements))

	// Handle negative step (reverse iteration)
	if step < 0 {
		if !hasStart {
			start = length - 1
		} else if start < 0 {
			start = length + start
		}
		if !hasEnd {
			end = -1
		} else if end < 0 {
			end = length + end
		}

		// Bounds checking
		if start >= length {
			start = length - 1
		}
		if start < 0 {
			start = -1
		}
		if end >= length {
			end = length - 1
		}

		result := []object.Object{}
		for i := start; i > end; i += step {
			if i >= 0 && i < length {
				result = append(result, elements[i])
			}
		}
		return &object.List{Elements: result}
	}

	// Positive step (forward iteration)
	if !hasStart {
		start = 0
	} else if start < 0 {
		start = length + start
		if start < 0 {
			start = 0
		}
	}
	if !hasEnd {
		end = length
	} else if end < 0 {
		end = length + end
		if end < 0 {
			end = 0
		}
	}

	// Bounds checking
	if start < 0 {
		start = 0
	}
	if end > length {
		end = length
	}
	if start > end {
		start = end
	}

	// If step is 1, use simple slicing
	if step == 1 {
		return &object.List{Elements: elements[start:end]}
	}

	// Step > 1
	result := []object.Object{}
	for i := start; i < end; i += step {
		result = append(result, elements[i])
	}
	return &object.List{Elements: result}
}

func sliceString(str string, start, end, step int64, hasStart, hasEnd, hasStep bool) string {
	length := int64(len(str))

	// Handle negative step (reverse iteration)
	if step < 0 {
		if !hasStart {
			start = length - 1
		} else if start < 0 {
			start = length + start
		}
		if !hasEnd {
			end = -1
		} else if end < 0 {
			end = length + end
		}

		// Bounds checking
		if start >= length {
			start = length - 1
		}
		if start < 0 {
			start = -1
		}
		if end >= length {
			end = length - 1
		}

		result := ""
		for i := start; i > end; i += step {
			if i >= 0 && i < length {
				result += string(str[i])
			}
		}
		return result
	}

	// Positive step (forward iteration)
	if !hasStart {
		start = 0
	} else if start < 0 {
		start = length + start
		if start < 0 {
			start = 0
		}
	}
	if !hasEnd {
		end = length
	} else if end < 0 {
		end = length + end
		if end < 0 {
			end = 0
		}
	}

	// Bounds checking
	if start < 0 {
		start = 0
	}
	if end > length {
		end = length
	}
	if start > end {
		start = end
	}

	// If step is 1, use simple slicing
	if step == 1 {
		return str[start:end]
	}

	// Step > 1
	result := ""
	for i := start; i < end; i += step {
		result += string(str[i])
	}
	return result
}
