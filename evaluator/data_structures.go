package evaluator

import (
	"context"
	"fmt"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// evalHashKeyChecked is like evalHashKey but surfaces a raise/error from a
// user-defined __hash__ instead of silently falling back to the identity hash.
// The second return is non-nil exactly when __hash__ raised or errored, or the
// value is unhashable (Python's TypeError: unhashable type).
func evalHashKeyChecked(ctx context.Context, obj object.Object) (string, object.Object) {
	if inst, ok := obj.(*object.Instance); ok {
		if _, hasHash := inst.Class.Methods["__hash__"]; hasHash && hashInstanceFn != nil {
			result := hashInstanceFn(ctx, inst)
			if object.IsError(result) || result.Type() == object.EXCEPTION_OBJ {
				return "", result
			}
			if n, ok := result.(*object.Integer); ok {
				return fmt.Sprintf("h:%d", n.IntValue()), nil
			}
			// __hash__ returned a non-integer — Python raises TypeError.
			return "", &object.Exception{Message: "__hash__ method should return an integer", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
		}
	}
	if !hashableAsKey(obj) {
		return "", &object.Exception{
			Message:       fmt.Sprintf("unhashable type: '%s'", getTypeName(obj)),
			ExceptionType: object.ExceptionTypeTypeError,
			Raised:        true,
		}
	}
	return object.DictKey(obj), nil
}

// hashableAsKey reports whether obj may serve as a dict key / set member.
// object.IsHashable is the strict content-hashability allowlist (used by
// sentinel machinery); dict and set keys additionally accept anything Python
// identity-hashes by default: plain instances, classes, functions. As in
// Python, an instance that defines __eq__ without __hash__ is unhashable.
func hashableAsKey(obj object.Object) bool {
	switch o := obj.(type) {
	case *object.Instance:
		if _, hasEq := o.Class.Methods["__eq__"]; hasEq {
			_, hasHash := o.Class.Methods["__hash__"]
			return hasHash
		}
		return true
	case *object.Class, *object.Function, *object.LambdaFunction, *object.Builtin:
		return true
	}
	return object.IsHashable(obj)
}

// evalSetAdd adds obj to set s, using __hash__ for instances.
// Returns a TypeError exception if obj is not hashable.
// iterableToSet materializes any iterable into a set, applying the same
// hashability checks as set literals. Used by the in-place set methods,
// which accept arbitrary iterables like Python's.
func iterableToSet(ctx context.Context, obj object.Object, env *object.Environment) (*object.Set, object.Object) {
	if asSet, ok := obj.(*object.Set); ok {
		return asSet, nil
	}
	elements, ok, rerr := iterableToSliceChecked(ctx, obj, env)
	if rerr != nil {
		return nil, rerr
	}
	if !ok {
		return nil, errors.NewTypeError("set or iterable", obj.Type().String())
	}
	result := object.NewSet()
	for _, elem := range elements {
		if err := evalSetAdd(ctx, result, elem); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func evalSetAdd(ctx context.Context, s *object.Set, obj object.Object) object.Object {
	if !hashableAsKey(obj) {
		return &object.Exception{Message: "unhashable type: '" + obj.Type().String() + "'", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
	}
	hk, raised := evalHashKeyChecked(ctx, obj)
	if raised != nil {
		return raised
	}
	s.AddKeyed(hk, obj)
	return nil
}

func evalIndexExpression(ctx context.Context, left, index object.Object, isDotAccess bool) object.Object {
	// Resolve each operand's type once. Type() is a dynamic interface call,
	// and attribute access on instances is hot enough that a chain of
	// per-case comparisons, each calling Type() again, is a measurable share
	// of method-call cost.
	leftType := left.Type()
	switch leftType {
	case object.LIST_OBJ:
		switch index.Type() {
		case object.INTEGER_OBJ:
			return evalListIndexExpression(left, index)
		case object.SLICE_OBJ:
			return evalListSliceExpression(left, index)
		}
	case object.FLOAT_ARRAY_OBJ:
		switch index.Type() {
		case object.INTEGER_OBJ:
			return evalFloatArrayIndexExpression(left, index)
		case object.SLICE_OBJ:
			return evalFloatArraySliceExpression(left, index)
		}
	case object.TUPLE_OBJ:
		switch index.Type() {
		case object.INTEGER_OBJ:
			return evalTupleIndexExpression(left, index)
		case object.SLICE_OBJ:
			return evalTupleSliceExpression(left, index)
		}
	case object.DICT_OBJ:
		// Dot access is attribute access (library modules are dicts): a
		// missing name is AttributeError, as for math.nope in Python.
		// Bracket access keeps KeyError.
		if isDotAccess {
			if name, ok := index.(*object.String); ok {
				if pair, exists := left.(*object.Dict).GetByString(name.StringValue()); exists {
					return pair.Value
				}
				// Modules expose only their members, not dict methods.
				if left.(*object.Dict).Module == "" {
					if method := builtinMethodRef(left, name.StringValue()); method != nil {
						return method
					}
				}
				return attributeError(left, name.StringValue())
			}
		}
		return evalDictIndexExpression(ctx, left, index)
	case object.STRING_OBJ:
		switch index.Type() {
		case object.INTEGER_OBJ:
			return evalStringIndexExpression(left, index)
		case object.SLICE_OBJ:
			return evalStringSliceExpression(left, index)
		}
	case object.BYTES_OBJ:
		switch index.Type() {
		case object.INTEGER_OBJ:
			return evalBytesIndexExpression(left, index)
		case object.SLICE_OBJ:
			return evalBytesSliceExpression(left, index)
		}
	case object.INSTANCE_OBJ:
		return evalInstanceIndexExpression(ctx, left, index, isDotAccess)
	case object.CLASS_OBJ:
		return evalClassIndexExpression(left, index)
	case object.BUILTIN_OBJ:
		return evalBuiltinIndexExpression(left, index)
	case object.PROPERTY_OBJ:
		return evalPropertyIndexExpression(left, index)
	case object.SUPER_OBJ:
		return evalSuperIndexExpression(left, index)
	case object.FUNCTION_OBJ:
		if !isDotAccess {
			return errors.NewError("index operator not supported: %s", leftType)
		}
		attr, _ := index.AsString()
		fn := left.(*object.Function)
		switch attr {
		case "__name__", "name":
			return object.NewString(fn.Name)
		}
		return attributeError(left, attr)
	case object.LAMBDA_OBJ:
		if !isDotAccess {
			return errors.NewError("index operator not supported: %s", leftType)
		}
		attr, _ := index.AsString()
		switch attr {
		case "__name__", "name":
			return object.NewString("<lambda>")
		}
		return attributeError(left, attr)
	case object.SENTINEL_OBJ:
		if !isDotAccess {
			// Python: "'sentinel' object is not subscriptable"
			return &object.Exception{
				Message:       fmt.Sprintf("'%s' object is not subscriptable", getTypeName(left)),
				ExceptionType: object.ExceptionTypeTypeError,
				Raised:        true,
			}
		}
		attr, _ := index.AsString()
		if attr == "__name__" {
			return object.NewString(left.(*object.Sentinel).Name)
		}
		return attributeError(left, attr)
	case object.SLICE_OBJ:
		if !isDotAccess {
			return &object.Exception{
				Message:       "'slice' object is not subscriptable",
				ExceptionType: object.ExceptionTypeTypeError,
				Raised:        true,
			}
		}
		attr, _ := index.AsString()
		if sl, ok := left.(*object.Slice); ok {
			switch attr {
			case "start":
				if sl.Start != nil {
					return sl.Start
				}
				return NULL
			case "stop":
				if sl.End != nil {
					return sl.End
				}
				return NULL
			case "step":
				if sl.Step != nil {
					return sl.Step
				}
				return NULL
			}
		}
		return attributeError(left, attr)
	}
	if isDotAccess {
		attr, _ := index.AsString()
		// Strings are scriptling's type representation (type(e) returns the
		// type name), so `.__name__` on one resolves the type(e).__name__
		// idiom: the name itself.
		if s, ok := left.(*object.String); ok && attr == "__name__" {
			return s
		}
		if exc, ok := left.(*object.Exception); ok {
			switch attr {
			case "args":
				args := exc.Args
				if args == nil {
					args = []object.Object{}
					if exc.Message != "" {
						args = []object.Object{object.NewString(exc.Message)}
					}
				}
				return &object.Tuple{Elements: args}
			case "__name__":
				name := exc.ExceptionType
				if name == "" {
					name = "Exception"
				}
				return object.NewString(name)
			}
			// Custom attributes of the user exception instance the raise
			// converted from (e.code for a class storing it in __init__).
			if exc.OriginInstance != nil {
				if v, exists := exc.OriginInstance.GetField(attr); exists {
					return v
				}
			}
		}
		if method := builtinMethodRef(left, attr); method != nil {
			return method
		}
		return attributeError(left, attr)
	}
	// Python's wording: sequences reject the index type, other values
	// cannot be indexed at all.
	message := fmt.Sprintf("'%s' object is not subscriptable", getTypeName(left))
	switch leftType {
	case object.LIST_OBJ, object.TUPLE_OBJ:
		message = fmt.Sprintf("%s indices must be integers or slices, not %s", getTypeName(left), getTypeName(index))
	case object.STRING_OBJ:
		message = fmt.Sprintf("string indices must be integers, not '%s'", getTypeName(index))
	case object.BYTES_OBJ:
		message = fmt.Sprintf("byte indices must be integers or slices, not %s", getTypeName(index))
	}
	return &object.Exception{
		Message:       message,
		ExceptionType: object.ExceptionTypeTypeError,
		Raised:        true,
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
	return attributeError(super, field)
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
		return &object.Exception{Message: "list index out of range", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
	}

	return listObject.Elements[idx]
}

func evalFloatArrayIndexExpression(faObj, index object.Object) object.Object {
	fa := faObj.(*object.FloatArray)
	idx, err := index.AsInt()
	if err != nil {
		return errors.NewError("float_array index must be integer")
	}

	if fa.Is2D() {
		rows := int64(fa.Rows())
		if idx < 0 {
			idx += rows
		}
		if idx < 0 || idx >= rows {
			return &object.Exception{Message: "float_array index out of range", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
		}
		cols := fa.Cols()
		start := int(idx) * cols
		rowData := make([]float64, cols)
		copy(rowData, fa.Data[start:start+cols])
		return object.NewFloatArray1D(rowData)
	}

	length := int64(len(fa.Data))
	if idx < 0 {
		idx += length
	}
	if idx < 0 || idx >= length {
		return &object.Exception{Message: "float_array index out of range", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
	}
	return object.NewFloat(fa.Data[idx])
}

func evalFloatArraySliceExpression(faObj, sliceObj object.Object) object.Object {
	fa := faObj.(*object.FloatArray)
	slice := sliceObj.(*object.Slice)

	if fa.Is2D() {
		rows := int64(fa.Rows())
		cols := fa.Cols()
		start := int64(0)
		end := rows
		step := int64(1)

		if slice.Start != nil {
			start = slice.Start.IntValue()
			if start < 0 {
				start += rows
			}
			if start < 0 {
				start = 0
			}
			if start > rows {
				start = rows
			}
		}
		if slice.End != nil {
			end = slice.End.IntValue()
			if end < 0 {
				end += rows
			}
			if end < 0 {
				end = 0
			}
			if end > rows {
				end = rows
			}
		}
		if slice.Step != nil {
			step = slice.Step.IntValue()
		}
		if step <= 0 {
			return errors.NewError("slice step cannot be zero or negative")
		}

		var data []float64
		for i := start; i < end; i += step {
			rowStart := int(i) * cols
			data = append(data, fa.Data[rowStart:rowStart+cols]...)
		}
		resultRows := 0
		if start < end && len(data) > 0 {
			resultRows = len(data) / cols
		}
		return object.NewFloatArray2D(data, resultRows, cols)
	}

	length := int64(len(fa.Data))
	start := int64(0)
	end := length
	step := int64(1)

	if slice.Start != nil {
		start = slice.Start.IntValue()
		if start < 0 {
			start += length
		}
		if start < 0 {
			start = 0
		}
		if start > length {
			start = length
		}
	}
	if slice.End != nil {
		end = slice.End.IntValue()
		if end < 0 {
			end += length
		}
		if end < 0 {
			end = 0
		}
		if end > length {
			end = length
		}
	}
	if slice.Step != nil {
		step = slice.Step.IntValue()
	}
	if step <= 0 {
		return errors.NewError("slice step cannot be zero or negative")
	}

	var data []float64
	for i := start; i < end; i += step {
		data = append(data, fa.Data[i])
	}
	return object.NewFloatArray1D(data)
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
		return &object.Exception{Message: "tuple index out of range", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
	}

	return tupleObject.Elements[idx]
}

func evalDictIndexExpression(ctx context.Context, dict, index object.Object) object.Object {
	dictObject := dict.(*object.Dict)
	key, rerr := evalHashKeyChecked(ctx, index)
	if rerr != nil {
		return rerr
	}

	pair, ok := dictObject.Pairs[key]
	if !ok {
		keyMsg := index.Inspect()
		if ks, ok := index.(*object.String); ok {
			keyMsg = object.ReprString(ks.StringValue())
		}
		return &object.Exception{Message: keyMsg, ExceptionType: object.ExceptionTypeKeyError, Raised: true}
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
	if isASCII(strObject.StringValue()) {
		length := int64(len(strObject.StringValue()))
		if idx < 0 {
			idx += length
		}
		if idx < 0 || idx >= length {
			return &object.Exception{Message: "string index out of range", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
		}
		return object.NewString(strObject.StringValue()[idx : idx+1])
	}

	runes := []rune(strObject.StringValue())
	length := int64(len(runes))
	if idx < 0 {
		idx += length
	}
	if idx < 0 || idx >= length {
		return &object.Exception{Message: "string index out of range", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
	}
	return object.NewString(string(runes[idx]))
}

func evalInstanceIndexExpression(ctx context.Context, instance, index object.Object, isDotAccess bool) object.Object {
	inst := instance.(*object.Instance)

	// Bracket access (obj[key]) is __getitem__, inherited or not; without it
	// the object is not subscriptable, as in Python.
	if !isDotAccess {
		if getitem, ok := inst.Class.LookupMember("__getitem__"); ok {
			args := []object.Object{instance, index}
			return applyFunctionWithContext(ctx, getitem, args, nil, nil)
		}
		return &object.Exception{
			Message:       fmt.Sprintf("'%s' object is not subscriptable", inst.Class.Name),
			ExceptionType: object.ExceptionTypeTypeError,
			Raised:        true,
		}
	}

	if index.Type() != object.STRING_OBJ {
		return &object.Exception{
			Message:       fmt.Sprintf("'%s' object is not subscriptable", inst.Class.Name),
			ExceptionType: object.ExceptionTypeTypeError,
			Raised:        true,
		}
	}
	field, err := index.AsString()
	if err != nil {
		return err
	}
	// Check instance fields first
	if val, ok := inst.GetField(field); ok {
		// If it's a property descriptor, call the getter
		if prop, ok := val.(*object.Property); ok {
			return applyFunctionWithContext(ctx, prop.Getter, []object.Object{instance}, nil, nil)
		}
		return val
	}
	// Check class methods - property descriptors on the class are also supported
	if fn, ok := inst.Class.LookupMember(field); ok {
		if prop, ok := fn.(*object.Property); ok {
			return applyFunctionWithContext(ctx, prop.Getter, []object.Object{instance}, nil, nil)
		}
		if sm, ok := fn.(*object.StaticMethod); ok {
			return sm.Fn // return the raw function for later calling
		}
		switch fn.(type) {
		case *object.Function, *object.LambdaFunction, *object.Builtin:
			return inst.GetBoundMethod(field, fn)
		default:
			return fn // non-callable class attribute (e.g. string set by class decorator)
		}
	}
	if field == "__class__" {
		return inst.Class
	}
	// As in Python, __getattr__ is consulted only when normal lookup fails.
	if getattr, ok := inst.Class.LookupMember("__getattr__"); ok {
		return applyFunctionWithContext(ctx, getattr, []object.Object{instance, index}, nil, nil)
	}
	return attributeError(inst, field)
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
	if field == "__name__" {
		return object.NewString(cl.Name)
	}
	if fn, ok := cl.LookupMember(field); ok {
		if sm, ok := fn.(*object.StaticMethod); ok {
			return sm.Fn
		}
		return fn
	}
	return attributeError(cl, field)
}

func evalPropertyIndexExpression(prop, index object.Object) object.Object {
	field, err := index.AsString()
	if err != nil {
		return errors.NewError("property attribute must be string")
	}
	if field != "setter" {
		return attributeError(prop, field)
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
	return attributeError(b, field)
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

// sliceBytes applies a [start:end:step] slice to a Bytes value, returning a new
// Bytes. Negative steps are honoured (matching String/List semantics).
func sliceBytes(b *object.Bytes, start, end, step int64, hasStart, hasEnd, hasStep bool) object.Object {
	src := b.BytesValue()
	length := int64(len(src))

	// Negative step (reverse iteration) — mirror sliceList's logic.
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
		if start >= length {
			start = length - 1
		}
		if start < 0 {
			start = -1
		}
		if end >= length {
			end = length - 1
		}

		var out []byte
		for i := start; i > end; i += step {
			if i >= 0 && i < length {
				out = append(out, src[i])
			}
		}
		return object.NewBytes(out)
	}

	// Positive step.
	if !hasStart {
		start = 0
	} else if start < 0 {
		start += length
		if start < 0 {
			start = 0
		}
	}
	if !hasEnd {
		end = length
	} else if end < 0 {
		end += length
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

	if step == 1 {
		return object.NewBytes(src[start:end])
	}
	var out []byte
	for i := start; i < end; i += step {
		out = append(out, src[i])
	}
	return object.NewBytes(out)
}

func sliceFloatArray(fa *object.FloatArray, start, end, step int64, hasStart, hasEnd, hasStep bool) object.Object {
	if step <= 0 {
		return errors.NewError("slice step cannot be zero or negative for FloatArray")
	}

	if fa.Is2D() {
		length := int64(fa.Rows())
		if !hasStart {
			start = 0
		} else if start < 0 {
			start += length
			if start < 0 {
				start = 0
			}
		}
		if !hasEnd {
			end = length
		} else if end < 0 {
			end += length
			if end < 0 {
				end = 0
			}
		}
		if start > length {
			start = length
		}
		if end > length {
			end = length
		}

		cols := fa.Cols()
		var data []float64
		for i := start; i < end; i += step {
			off := int(i) * cols
			data = append(data, fa.Data[off:off+cols]...)
		}
		resultRows := 0
		if len(data) > 0 {
			resultRows = len(data) / cols
		}
		return object.NewFloatArray2D(data, resultRows, cols)
	}

	length := int64(len(fa.Data))
	if !hasStart {
		start = 0
	} else if start < 0 {
		start += length
		if start < 0 {
			start = 0
		}
	}
	if !hasEnd {
		end = length
	} else if end < 0 {
		end += length
		if end < 0 {
			end = 0
		}
	}
	if start > length {
		start = length
	}
	if end > length {
		end = length
	}

	var data []float64
	for i := start; i < end; i += step {
		data = append(data, fa.Data[i])
	}
	return object.NewFloatArray1D(data)
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
		start = sliceObj.Start.IntValue()
	}
	if hasEnd {
		end = sliceObj.End.IntValue()
	}
	if hasStep {
		step = sliceObj.Step.IntValue()
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
		start = sliceObj.Start.IntValue()
	}
	if hasEnd {
		end = sliceObj.End.IntValue()
	}
	if hasStep {
		step = sliceObj.Step.IntValue()
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
		start = sliceObj.Start.IntValue()
	}
	if hasEnd {
		end = sliceObj.End.IntValue()
	}
	if hasStep {
		step = sliceObj.Step.IntValue()
		if step == 0 {
			return errors.NewError("slice step cannot be zero")
		}
	}

	slicedStr := sliceString(strObj.StringValue(), start, end, step, hasStart, hasEnd, hasStep)
	return object.NewString(slicedStr)
}

// evalBytesIndexExpression indexes a Bytes value by an integer, returning the
// byte value (0-255) as an Integer. Negative indices count from the end. An
// out-of-range index raises IndexError, matching list/tuple/string indexing.
func evalBytesIndexExpression(b, index object.Object) object.Object {
	bObj := b.(*object.Bytes)
	idx, err := index.AsInt()
	if err != nil {
		return errors.NewError("bytes index must be integer")
	}
	length := int64(bObj.Len())
	if idx < 0 {
		idx += length
	}
	if idx < 0 || idx >= length {
		return &object.Exception{Message: "bytes index out of range", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
	}
	return object.NewInteger(int64(bObj.BytesValue()[idx]))
}

// evalBytesSliceExpression handles slice objects applied to Bytes. Step is
// honoured. Returns a fresh Bytes value (immutability is preserved).
func evalBytesSliceExpression(b, index object.Object) object.Object {
	bObj := b.(*object.Bytes)
	sliceObj := index.(*object.Slice)

	length := int64(bObj.Len())
	start, end, step := int64(0), length, int64(1)
	hasStart, hasEnd := sliceObj.Start != nil, sliceObj.End != nil
	hasStep := sliceObj.Step != nil
	if hasStart {
		start = sliceObj.Start.IntValue()
	}
	if hasEnd {
		end = sliceObj.End.IntValue()
	}
	if hasStep {
		step = sliceObj.Step.IntValue()
		if step == 0 {
			return errors.NewError("slice step cannot be zero")
		}
	}

	// Normalise bounds against [0, length].
	if start < 0 {
		start += length
		if start < 0 {
			start = 0
		}
	} else if start > length {
		start = length
	}
	if end < 0 {
		end += length
		if end < 0 {
			end = 0
		}
	} else if end > length {
		end = length
	}

	src := bObj.BytesValue()
	var out []byte
	if step > 0 {
		if start < end {
			out = make([]byte, 0, (end-start+step-1)/step)
			for i := start; i < end; i += step {
				out = append(out, src[i])
			}
		}
	} else {
		// Negative step: iterate from start down to end (exclusive).
		if start > end {
			out = make([]byte, 0, (start-end-step-1)/-step)
			for i := start; i > end; i += step {
				out = append(out, src[i])
			}
		}
	}
	return object.NewBytes(out)
}
