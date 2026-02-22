package evaluator

import (
	"context"
	"strings"

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
		if object.IsError(key) {
			return key
		}

		value := evalWithContext(ctx, valueNode, env)
		if object.IsError(value) {
			return value
		}

		pairs[object.DictKey(key)] = object.DictPair{Key: key, Value: value}
	}

	return &object.Dict{Pairs: pairs}
}

func evalIndexExpression(ctx context.Context, left, index object.Object) object.Object {
	switch {
	case left.Type() == object.LIST_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalListIndexExpression(left, index)
	case left.Type() == object.LIST_OBJ && index.Type() == object.SLICE_OBJ:
		return evalListSliceExpression(left, index)
	case left.Type() == object.TUPLE_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalTupleIndexExpression(left, index)
	case left.Type() == object.TUPLE_OBJ && index.Type() == object.SLICE_OBJ:
		return evalTupleSliceExpression(left, index)
	case left.Type() == object.DICT_OBJ:
		return evalDictIndexExpression(left, index)
	case left.Type() == object.STRING_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalStringIndexExpression(left, index)
	case left.Type() == object.STRING_OBJ && index.Type() == object.SLICE_OBJ:
		return evalStringSliceExpression(left, index)
	case left.Type() == object.INSTANCE_OBJ:
		return evalInstanceIndexExpression(ctx, left, index)
	case left.Type() == object.CLASS_OBJ:
		return evalClassIndexExpression(left, index)
	case left.Type() == object.BUILTIN_OBJ:
		return evalBuiltinIndexExpression(left, index)
	case left.Type() == object.PROPERTY_OBJ:
		return evalPropertyIndexExpression(left, index)
	case left.Type() == object.SUPER_OBJ:
		return evalSuperIndexExpression(left, index)
	default:
		return errors.NewError("index operator not supported: %s", left.Type())
	}
}

func evalSuperIndexExpression(superObj, index object.Object) object.Object {
	if index.Type() != object.STRING_OBJ {
		return errors.NewError("super index must be string")
	}
	field, err := index.AsString()
	if err != nil {
		return err
	}
	super := superObj.(*object.Super)

	currentClass := super.Class.BaseClass
	for currentClass != nil {
		if fn, ok := currentClass.Methods[field]; ok {
			return fn
		}
		currentClass = currentClass.BaseClass
	}
	return NULL
}

func evalListIndexExpression(list, index object.Object) object.Object {
	listObject := list.(*object.List)
	idx, err := index.AsInt()
	if err != nil {
		return errors.NewError("list index must be integer")
	}
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
	idx, err := index.AsInt()
	if err != nil {
		return errors.NewError("tuple index must be integer")
	}
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
	key := object.DictKey(index)

	pair, ok := dictObject.Pairs[key]
	if !ok {
		return NULL
	}

	return pair.Value
}

// isASCII checks if a string contains only ASCII characters.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func evalStringIndexExpression(str, index object.Object) object.Object {
	strObject := str.(*object.String)
	idx, err := index.AsInt()
	if err != nil {
		return errors.NewError("string index must be integer")
	}

	// ASCII fast-path: avoid []rune conversion
	if isASCII(strObject.Value) {
		length := int64(len(strObject.Value))
		if idx < 0 {
			idx += length
		}
		if idx < 0 || idx >= length {
			return NULL
		}
		return &object.String{Value: strObject.Value[idx : idx+1]}
	}

	// Non-ASCII: fall back to rune conversion
	runes := []rune(strObject.Value)
	length := int64(len(runes))
	if idx < 0 {
		idx += length
	}
	if idx < 0 || idx >= length {
		return NULL
	}
	return &object.String{Value: string(runes[idx])}
}

func evalInstanceIndexExpression(ctx context.Context, instance, index object.Object) object.Object {
	inst := instance.(*object.Instance)

	// First check for __getitem__ method
	if getitem, ok := inst.Class.Methods["__getitem__"]; ok {
		// Call __getitem__ with the index
		args := []object.Object{instance, index}
		return applyFunctionWithContext(ctx, getitem, args, nil, nil)
	}

	// Fallback to string-based field access
	if index.Type() != object.STRING_OBJ {
		return errors.NewError("instance index must be string")
	}
	field, err := index.AsString()
	if err != nil {
		return err
	}
	// Check instance fields first
	if val, ok := inst.Fields[field]; ok {
		// If it's a property descriptor, call the getter
		if prop, ok := val.(*object.Property); ok {
			return applyFunctionWithContext(ctx, prop.Getter, []object.Object{instance}, nil, nil)
		}
		return val
	}
	// Check class methods - property descriptors on the class are also supported
	if fn, ok := inst.Class.Methods[field]; ok {
		if prop, ok := fn.(*object.Property); ok {
			return applyFunctionWithContext(ctx, prop.Getter, []object.Object{instance}, nil, nil)
		}
		if sm, ok := fn.(*object.StaticMethod); ok {
			return sm.Fn // return the raw function for later calling
		}
		switch fn.(type) {
		case *object.Function, *object.LambdaFunction, *object.Builtin:
			return &object.BoundMethod{Instance: instance, Method: fn}
		default:
			return fn // non-callable class attribute (e.g. string set by class decorator)
		}
	}
	// Walk base classes
	currentClass := inst.Class.BaseClass
	for currentClass != nil {
		if fn, ok := currentClass.Methods[field]; ok {
			if prop, ok := fn.(*object.Property); ok {
				return applyFunctionWithContext(ctx, prop.Getter, []object.Object{instance}, nil, nil)
			}
			if sm, ok := fn.(*object.StaticMethod); ok {
				return sm.Fn
			}
			switch fn.(type) {
			case *object.Function, *object.LambdaFunction, *object.Builtin:
				return &object.BoundMethod{Instance: instance, Method: fn}
			default:
				return fn
			}
		}
		currentClass = currentClass.BaseClass
	}
	return NULL
}

func evalClassIndexExpression(class, index object.Object) object.Object {
	if index.Type() != object.STRING_OBJ {
		return errors.NewError("class index must be string")
	}
	field, err := index.AsString()
	if err != nil {
		return err
	}
	cl := class.(*object.Class)
	if fn, ok := cl.Methods[field]; ok {
		if sm, ok := fn.(*object.StaticMethod); ok {
			return sm.Fn
		}
		return fn
	}
	return NULL
}

func evalPropertyIndexExpression(prop, index object.Object) object.Object {
	field, err := index.AsString()
	if err != nil {
		return errors.NewError("property attribute must be string")
	}
	if field != "setter" {
		return errors.NewError("property has no attribute '%s'", field)
	}
	p := prop.(*object.Property)
	// Return a callable: setter(fn) -> new Property{Getter: p.Getter, Setter: fn}
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewError("setter() takes exactly 1 argument")
			}
			return &object.Property{Getter: p.Getter, Setter: args[0]}
		},
	}
}

func evalBuiltinIndexExpression(builtin, index object.Object) object.Object {
	if index.Type() != object.STRING_OBJ {
		return errors.NewError("builtin index must be string")
	}
	field, err := index.AsString()
	if err != nil {
		return err
	}
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
	if object.IsError(left) {
		return left
	}

	var start, end, step int64
	var hasStart, hasEnd, hasStep bool
	step = 1 // default step

	if node.Start != nil {
		startObj := evalWithContext(ctx, node.Start, env)
		if object.IsError(startObj) {
			return startObj
		}
		s, err := startObj.AsInt()
		if err != nil {
			return err
		}
		start = s
		hasStart = true
	}

	if node.End != nil {
		endObj := evalWithContext(ctx, node.End, env)
		if object.IsError(endObj) {
			return endObj
		}
		e, err := endObj.AsInt()
		if err != nil {
			return err
		}
		end = e
		hasEnd = true
	}

	if node.Step != nil {
		stepObj := evalWithContext(ctx, node.Step, env)
		if object.IsError(stepObj) {
			return stepObj
		}
		s, err := stepObj.AsInt()
		if err != nil {
			return err
		}
		step = s
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
	// ASCII fast-path for step=1 (most common): use byte indexing directly
	if step == 1 && isASCII(str) {
		length := int64(len(str))
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
		if start < 0 {
			start = 0
		}
		if end > length {
			end = length
		}
		if start > end {
			start = end
		}
		return str[start:end]
	}

	runes := []rune(str)
	length := int64(len(runes))

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

		var builder strings.Builder
		for i := start; i > end; i += step {
			if i >= 0 && i < length {
				builder.WriteRune(runes[i])
			}
		}
		return builder.String()
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
		return string(runes[start:end])
	}

	// Step > 1
	var builder strings.Builder
	for i := start; i < end; i += step {
		builder.WriteRune(runes[i])
	}
	return builder.String()
}

// evalListSliceExpression handles slice objects applied to lists
func evalListSliceExpression(list, index object.Object) object.Object {
	listObj := list.(*object.List)
	sliceObj := index.(*object.Slice)

	// Extract slice parameters
	var start, end, step int64
	var hasStart, hasEnd, hasStep bool

	// Default values
	step = 1
	hasStart = sliceObj.Start != nil
	hasEnd = sliceObj.End != nil
	hasStep = sliceObj.Step != nil

	if hasStart {
		start = sliceObj.Start.Value
	}
	if hasEnd {
		end = sliceObj.End.Value
	}
	if hasStep {
		step = sliceObj.Step.Value
		if step == 0 {
			return errors.NewError("slice step cannot be zero")
		}
	}

	return sliceList(listObj.Elements, start, end, step, hasStart, hasEnd, hasStep)
}

// evalTupleSliceExpression handles slice objects applied to tuples
func evalTupleSliceExpression(tuple, index object.Object) object.Object {
	tupleObj := tuple.(*object.Tuple)
	sliceObj := index.(*object.Slice)

	// Extract slice parameters
	var start, end, step int64
	var hasStart, hasEnd, hasStep bool

	// Default values
	step = 1
	hasStart = sliceObj.Start != nil
	hasEnd = sliceObj.End != nil
	hasStep = sliceObj.Step != nil

	if hasStart {
		start = sliceObj.Start.Value
	}
	if hasEnd {
		end = sliceObj.End.Value
	}
	if hasStep {
		step = sliceObj.Step.Value
		if step == 0 {
			return errors.NewError("slice step cannot be zero")
		}
	}

	// Use sliceList and convert result to tuple
	sliced := sliceList(tupleObj.Elements, start, end, step, hasStart, hasEnd, hasStep)
	if slicedList, ok := sliced.(*object.List); ok {
		return &object.Tuple{Elements: slicedList.Elements}
	}
	return sliced
}

// evalStringSliceExpression handles slice objects applied to strings
func evalStringSliceExpression(str, index object.Object) object.Object {
	strObj := str.(*object.String)
	sliceObj := index.(*object.Slice)

	// Extract slice parameters
	var start, end, step int64
	var hasStart, hasEnd, hasStep bool

	// Default values
	step = 1
	hasStart = sliceObj.Start != nil
	hasEnd = sliceObj.End != nil
	hasStep = sliceObj.Step != nil

	if hasStart {
		start = sliceObj.Start.Value
	}
	if hasEnd {
		end = sliceObj.End.Value
	}
	if hasStep {
		step = sliceObj.Step.Value
		if step == 0 {
			return errors.NewError("slice step cannot be zero")
		}
	}

	slicedStr := sliceString(strObj.Value, start, end, step, hasStart, hasEnd, hasStep)
	return &object.String{Value: slicedStr}
}
