package object

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Global type cache for maximum performance
var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	kwargsType  = reflect.TypeOf(Kwargs{})
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
	objectType  = reflect.TypeOf((*Object)(nil)).Elem()

	// Pre-allocated common reflect.Values
	nullValue = reflect.ValueOf(&Null{})

	// Function signature cache
	signatureCache = sync.Map{} // map[reflect.Type]*FunctionSignature

	// Pool for reflect.Value slices to reduce allocations
	// We use different pools for different slice sizes
	argValuePool2 = sync.Pool{New: func() any { s := make([]reflect.Value, 0, 2); return &s }}
	argValuePool4 = sync.Pool{New: func() any { s := make([]reflect.Value, 0, 4); return &s }}
	argValuePool8 = sync.Pool{New: func() any { s := make([]reflect.Value, 0, 8); return &s }}
)

// FunctionSignature holds pre-computed function analysis
type FunctionSignature struct {
	numIn, numOut int
	isVariadic    bool
	variadicIndex int
	hasContext    bool
	hasKwargs     bool
	paramOffset   int
	maxPosArgs    int
	paramTypes    []reflect.Type // Cache parameter types
	returnIsError bool           // Cache if second return is error
}

// LibraryBuilder provides a fluent API for creating scriptling libraries.
// It allows registering typed Go functions that are automatically wrapped
// to handle conversion between Go types and scriptling Objects.
//
// Example usage:
//
//	lib := NewLibraryBuilder("mylib", "My custom library")
//	lib.Function("connect", func(host string, port int) error {
//	    // Connect to host:port
//	    return nil
//	})
//	lib.Function("disconnect", func() error {
//	    // Disconnect
//	    return nil
//	})
//	lib.Constant("VERSION", "1.0.0")
//	library := lib.Build()
type LibraryBuilder struct {
	name        string
	description string
	functions   map[string]*Builtin
	constants   map[string]Object
}

// analyzeFunctionSignature performs one-time analysis of function signature
// For functions/libraries: [context], [kwargs], ...args (all optional)
func analyzeFunctionSignature(fnType reflect.Type) *FunctionSignature {
	// Check cache first
	if cached, ok := signatureCache.Load(fnType); ok {
		return cached.(*FunctionSignature)
	}

	numIn := fnType.NumIn()
	numOut := fnType.NumOut()
	isVariadic := fnType.IsVariadic()
	variadicIndex := -1
	if isVariadic {
		variadicIndex = numIn - 1
	}

	// Detect context and kwargs (both optional, context first if present)
	paramOffset := 0
	hasContext := numIn > paramOffset && fnType.In(paramOffset) == contextType
	if hasContext {
		paramOffset++
	}
	hasKwargs := numIn > paramOffset && fnType.In(paramOffset) == kwargsType
	if hasKwargs {
		paramOffset++
	}
	maxPosArgs := numIn - paramOffset

	// Pre-cache parameter types
	paramTypes := make([]reflect.Type, numIn)
	for i := 0; i < numIn; i++ {
		paramTypes[i] = fnType.In(i)
	}

	// Check if second return is error
	returnIsError := numOut == 2 && fnType.Out(1).Implements(errorType)

	sig := &FunctionSignature{
		numIn:         numIn,
		numOut:        numOut,
		isVariadic:    isVariadic,
		variadicIndex: variadicIndex,
		hasContext:    hasContext,
		hasKwargs:     hasKwargs,
		paramOffset:   paramOffset,
		maxPosArgs:    maxPosArgs,
		paramTypes:    paramTypes,
		returnIsError: returnIsError,
	}

	// Cache for future use
	signatureCache.Store(fnType, sig)
	return sig
}

// NewLibraryBuilder creates a new LibraryBuilder with the given name and description.
func NewLibraryBuilder(name, description string) *LibraryBuilder {
	return &LibraryBuilder{
		name:        name,
		description: description,
		functions:   make(map[string]*Builtin),
		constants:   make(map[string]Object),
	}
}

// Function registers a function with the given name.
// The function must be a Go function with typed parameters.
// Parameters can be: string, int, int64, float64, bool, []any, map[string]any
// Return values can be: any of the above types, or error
//
// Example:
//
//	builder.Function("add", func(a, b int) int { return a + b })
//	builder.Function("greet", func(name string) string { return "Hello, " + name })
//	builder.Function("connect", func(host string, port int) error { ... })
func (b *LibraryBuilder) Function(name string, fn interface{}) *LibraryBuilder {
	b.FunctionWithHelp(name, fn, "")
	return b
}

// FunctionWithHelp registers a function with the given name and help text.
// The function must be a Go function with typed parameters.
// Help text is displayed when users call help() on the function.
//
// Example:
//
//	builder.FunctionWithHelp("sqrt", func(x float64) float64 {
//	    return math.Sqrt(x)
//	}, "sqrt(x) - Return the square root of x")
func (b *LibraryBuilder) FunctionWithHelp(name string, fn interface{}, helpText string) *LibraryBuilder {
	wrapper := b.createWrapper(reflect.ValueOf(fn), helpText)
	b.functions[name] = wrapper
	return b
}

// Constant registers a constant value with the given name.
// The value is automatically converted to a scriptling Object.
// Supported types: string, int, int64, float64, bool, nil
//
// Example:
//
//	builder.Constant("VERSION", "1.0.0")
//	builder.Constant("MAX_CONNECTIONS", 100)
//	builder.Constant("DEBUG", true)
func (b *LibraryBuilder) Constant(name string, value interface{}) *LibraryBuilder {
	b.constants[name] = convertValueToObject(value)
	return b
}

// Build creates and returns the Library from this builder.
// After calling Build(), the builder should not be used further.
func (b *LibraryBuilder) Build() *Library {
	return NewLibrary(b.name, b.functions, b.constants, b.description)
}

// createWrapper creates a Builtin wrapper for a typed Go function.
// It uses reflection to convert between scriptling Objects and Go types.
func (b *LibraryBuilder) createWrapper(fnValue reflect.Value, helpText string) *Builtin {
	fnType := fnValue.Type()

	// Validate that it's a function
	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf("LibraryBuilder: must be a function, got %T", fnValue.Interface()))
	}

	// Analyze function signature once (cached)
	sig := analyzeFunctionSignature(fnType)

	return &Builtin{
		Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) Object {
			return callTypedFunction(fnValue, sig, ctx, kwargs, args)
		},
		HelpText: helpText,
	}
}

// getArgValueSlice gets a pooled slice with the appropriate capacity
func getArgValueSlice(capacity int) *[]reflect.Value {
	switch {
	case capacity <= 2:
		return argValuePool2.Get().(*[]reflect.Value)
	case capacity <= 4:
		return argValuePool4.Get().(*[]reflect.Value)
	default:
		return argValuePool8.Get().(*[]reflect.Value)
	}
}

// putArgValueSlice returns a slice to the pool
func putArgValueSlice(s *[]reflect.Value, capacity int) {
	*s = (*s)[:0] // Reset length to 0
	switch {
	case capacity <= 2:
		argValuePool2.Put(s)
	case capacity <= 4:
		argValuePool4.Put(s)
	default:
		argValuePool8.Put(s)
	}
}

// callTypedFunction calls a typed Go function with cached signature info.
// Shared by LibraryBuilder and FunctionBuilder.
func callTypedFunction(fnValue reflect.Value, sig *FunctionSignature, ctx context.Context, kwargs Kwargs, args []Object) Object {
	// Get pooled slice for arguments
	argValuesPtr := getArgValueSlice(sig.numIn)
	argValues := *argValuesPtr

	// Add context parameter if present
	if sig.hasContext {
		argValues = append(argValues, reflect.ValueOf(ctx))
	}

	// Add kwargs parameter if present
	if sig.hasKwargs {
		argValues = append(argValues, reflect.ValueOf(kwargs))
	}

	// Positional arguments with cached types
	argIndex := 0
	for i := 0; i < sig.maxPosArgs; i++ {
		fnParamIndex := i + sig.paramOffset

		if sig.isVariadic && fnParamIndex == sig.variadicIndex {
			// Variadic parameters - collect remaining args
			elemType := sig.paramTypes[fnParamIndex].Elem()
			for j := argIndex; j < len(args); j++ {
				val, convErr := convertObjectToValue(args[j], elemType)
				if convErr != nil {
					putArgValueSlice(argValuesPtr, sig.numIn)
					return convErr
				}
				argValues = append(argValues, val)
			}
			break
		}

		if argIndex >= len(args) {
			putArgValueSlice(argValuesPtr, sig.numIn)
			return newArgumentError(len(args), sig.maxPosArgs)
		}

		// Use cached parameter type
		val, convErr := convertObjectToValue(args[argIndex], sig.paramTypes[fnParamIndex])
		if convErr != nil {
			putArgValueSlice(argValuesPtr, sig.numIn)
			return convErr
		}
		argValues = append(argValues, val)
		argIndex++
	}

	// Check if we have extra positional arguments
	if argIndex < len(args) && !sig.isVariadic {
		putArgValueSlice(argValuesPtr, sig.numIn)
		return newArgumentError(len(args), sig.maxPosArgs)
	}

	// Call the function
	results := fnValue.Call(argValues)

	// Return slice to pool
	putArgValueSlice(argValuesPtr, sig.numIn)

	// Handle return values with cached info
	switch sig.numOut {
	case 0:
		return &Null{}
	case 1:
		return convertReturnValue(results[0])
	case 2:
		// Use cached error check
		if sig.returnIsError && !results[1].IsNil() {
			err, _ := results[1].Interface().(error)
			return newError("%s", err.Error())
		}
		return convertReturnValue(results[0])
	default:
		return newError("function can return at most 2 values")
	}
}

// newError creates a new error object (avoids circular import with errors package)
func newError(format string, args ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(format, args...)}
}

// newTypeError creates a type error object (matches errors.NewTypeError format)
func newTypeError(expected, got string) *Error {
	return &Error{Message: fmt.Sprintf("type error: expected %s, got %s", expected, got)}
}

// newArgumentError creates an argument error object (matches errors.NewArgumentError format)
func newArgumentError(got, want int) *Error {
	return &Error{Message: fmt.Sprintf("argument error: got %d arguments, want %d", got, want)}
}

// convertObjectToValue converts a scriptling Object to a reflect.Value for the given type.
func convertObjectToValue(obj Object, targetType reflect.Type) (reflect.Value, Object) {
	if obj == nil {
		return reflect.Zero(targetType), nil
	}

	switch targetType.Kind() {
	case reflect.String:
		s, err := obj.AsString()
		if err == nil {
			return reflect.ValueOf(s), nil
		}
		return reflect.Value{}, err

	case reflect.Int:
		// Fast path for int (common type)
		i, err := obj.AsInt()
		if err == nil {
			return reflect.ValueOf(int(i)), nil
		}
		return reflect.Value{}, err

	case reflect.Int64:
		// Fast path for int64 (most common int type in scriptling)
		i, err := obj.AsInt()
		if err == nil {
			return reflect.ValueOf(i), nil
		}
		return reflect.Value{}, err

	case reflect.Int32:
		i, err := obj.AsInt()
		if err == nil {
			return reflect.ValueOf(int32(i)), nil
		}
		return reflect.Value{}, err

	case reflect.Float64:
		// Fast path for float64 (most common float type)
		f, err := obj.AsFloat()
		if err == nil {
			return reflect.ValueOf(f), nil
		}
		return reflect.Value{}, err

	case reflect.Float32:
		f, err := obj.AsFloat()
		if err == nil {
			return reflect.ValueOf(float32(f)), nil
		}
		return reflect.Value{}, err

	case reflect.Bool:
		b, err := obj.AsBool()
		if err == nil {
			return reflect.ValueOf(b), nil
		}
		return reflect.Value{}, err

	case reflect.Interface:
		// If the target type is object.Object, return the object as-is
		if targetType.Implements(objectType) {
			return reflect.ValueOf(obj), nil
		}
		// For interface{}, return the underlying value
		switch v := obj.(type) {
		case *String:
			return reflect.ValueOf(v.Value), nil
		case *Integer:
			return reflect.ValueOf(v.Value), nil
		case *Float:
			return reflect.ValueOf(v.Value), nil
		case *Boolean:
			return reflect.ValueOf(v.Value), nil
		case *Null:
			return reflect.ValueOf(nil), nil
		case *List:
			// Convert to []any
			items := make([]interface{}, len(v.Elements))
			for i, el := range v.Elements {
				items[i] = objectToAny(el)
			}
			return reflect.ValueOf(items), nil
		case *Dict:
			// Convert to map[string]any
			m := make(map[string]interface{})
			for _, pair := range v.Pairs {
				m[pair.StringKey()] = objectToAny(pair.Value)
			}
			return reflect.ValueOf(m), nil
		default:
			return reflect.Value{}, newTypeError("value", obj.Type().String())
		}

	case reflect.Slice:
		// Convert to slice
		if list, err := obj.AsList(); err == nil {
			elemType := targetType.Elem()
			slice := reflect.MakeSlice(targetType, len(list), len(list))
			for i, el := range list {
				val, err := convertObjectToValue(el, elemType)
				if err != nil {
					return reflect.Value{}, err
				}
				slice.Index(i).Set(val)
			}
			return slice, nil
		}
		return reflect.Value{}, newTypeError("LIST", obj.Type().String())

	case reflect.Map:
		// Convert to map
		if d, err := obj.AsDict(); err == nil {
			keyType := targetType.Key()
			if keyType.Kind() != reflect.String {
				return reflect.Value{}, newError("map keys must be strings")
			}
			valueType := targetType.Elem()
			resultMap := reflect.MakeMap(targetType)
			for key, val := range d {
				convertedVal, err := convertObjectToValue(val, valueType)
				if err != nil {
					return reflect.Value{}, err
				}
				resultMap.SetMapIndex(reflect.ValueOf(key), convertedVal)
			}
			return resultMap, nil
		}
		return reflect.Value{}, newTypeError("DICT", obj.Type().String())

	default:
		return reflect.Value{}, newTypeError("supported type", targetType.String())
	}
}

// convertReturnValue converts a Go return value to a scriptling Object.
func convertReturnValue(v reflect.Value) Object {
	if !v.IsValid() {
		return &Null{}
	}

	// Only check IsNil for types that can be nil
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if v.IsNil() {
			return &Null{}
		}
	}

	switch v.Kind() {
	case reflect.String:
		return &String{Value: v.String()}
	case reflect.Int, reflect.Int32, reflect.Int64:
		return NewInteger(v.Int())
	case reflect.Float32, reflect.Float64:
		return &Float{Value: v.Float()}
	case reflect.Bool:
		return &Boolean{Value: v.Bool()}
	case reflect.Interface:
		// For interface{}, convert the underlying value
		return convertValueToObject(v.Interface())
	case reflect.Map, reflect.Slice:
		// For maps and slices, convert via interface{}
		return convertValueToObject(v.Interface())
	default:
		return newError("unsupported return type: %s", v.Kind())
	}
}

// convertValueToObject converts a Go value to a scriptling Object.
func convertValueToObject(v interface{}) Object {
	if v == nil {
		return &Null{}
	}

	switch val := v.(type) {
	case string:
		return &String{Value: val}
	case int, int32, int64:
		return NewInteger(reflect.ValueOf(v).Int())
	case float32, float64:
		return &Float{Value: reflect.ValueOf(v).Float()}
	case bool:
		return &Boolean{Value: val}
	case Object:
		return val
	case []interface{}:
		elements := make([]Object, len(val))
		for i, item := range val {
			elements[i] = convertValueToObject(item)
		}
		return &List{Elements: elements}
	case map[string]interface{}:
		pairs := make(map[string]DictPair)
		for key, item := range val {
			pairs[DictKey(&String{Value: key})] = DictPair{
				Key:   &String{Value: key},
				Value: convertValueToObject(item),
			}
		}
		return &Dict{Pairs: pairs}
	default:
		return newError("unsupported constant type: %T", v)
	}
}

// objectToAny converts a scriptling Object to a Go any value.
func objectToAny(obj Object) interface{} {
	switch v := obj.(type) {
	case *String:
		return v.Value
	case *Integer:
		return v.Value
	case *Float:
		return v.Value
	case *Boolean:
		return v.Value
	case *Null:
		return nil
	case *List:
		items := make([]interface{}, len(v.Elements))
		for i, el := range v.Elements {
			items[i] = objectToAny(el)
		}
		return items
	case *Dict:
		m := make(map[string]interface{})
		for _, pair := range v.Pairs {
			m[pair.StringKey()] = objectToAny(pair.Value)
		}
		return m
	default:
		return nil
	}
}

// SubLibrary creates a new sub-library with the given name.
// Sub-libraries are accessed as `parent.sub` in scriptling code.
//
// Example:
//
//	subLib := NewLibraryBuilder("parse", "URL parsing utilities")
//	subLib.Function("quote", func(s string) string { ... })
//	builder.SubLibrary("parse", subLib.Build())
func (b *LibraryBuilder) SubLibrary(name string, lib *Library) *LibraryBuilder {
	// Add sub-library as a constant (will be accessible as parent.name)
	b.constants[name] = lib
	return b
}

// FunctionFromVariadic registers a variadic function that accepts a variable number of arguments.
// This is useful for functions like print() that can take any number of arguments.
//
// Example:
//
//	builder.FunctionFromVariadic("print_all", func(args ...any) {
//	    for _, arg := range args {
//	        fmt.Println(arg)
//	    }
//	})
func (b *LibraryBuilder) FunctionFromVariadic(name string, fn interface{}) *LibraryBuilder {
	return b.FunctionFromVariadicWithHelp(name, fn, "")
}

// FunctionFromVariadicWithHelp registers a variadic function with help text.
func (b *LibraryBuilder) FunctionFromVariadicWithHelp(name string, fn interface{}, helpText string) *LibraryBuilder {
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func || !fnType.IsVariadic() {
		panic("FunctionFromVariadic requires a variadic function")
	}

	// Create a wrapper that collects all args into a slice
	wrapper := &Builtin{
		Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) Object {
			// Convert all args to interface{}
			variadicType := fnType.In(0)
			if variadicType.Kind() != reflect.Slice {
				return newError("variadic function must take a slice parameter")
			}

			elemType := variadicType.Elem()
			slice := reflect.MakeSlice(variadicType, len(args), len(args))

			for i, arg := range args {
				val, err := convertObjectToValue(arg, elemType)
				if err != nil {
					return err
				}
				slice.Index(i).Set(val)
			}

			results := fnValue.Call([]reflect.Value{slice})

			if len(results) == 0 {
				return &Null{}
			}
			return convertReturnValue(results[0])
		},
		HelpText: helpText,
	}

	b.functions[name] = wrapper
	return b
}

// Alias creates an alias for an existing function.
//
// Example:
//
//	builder.Function("add", func(a, b int) int { return a + b })
//	builder.Alias("sum", "add")  // "sum" is now an alias for "add"
func (b *LibraryBuilder) Alias(alias, originalName string) *LibraryBuilder {
	if fn, ok := b.functions[originalName]; ok {
		b.functions[alias] = fn
	}
	return b
}

// Description sets or updates the library description.
func (b *LibraryBuilder) Description(desc string) *LibraryBuilder {
	b.description = desc
	return b
}

// GetDescription returns the current library description.
func (b *LibraryBuilder) GetDescription() string {
	return b.description
}

// HasFunction checks if a function with the given name has been registered.
func (b *LibraryBuilder) HasFunction(name string) bool {
	_, ok := b.functions[name]
	return ok
}

// HasConstant checks if a constant with the given name has been registered.
func (b *LibraryBuilder) HasConstant(name string) bool {
	_, ok := b.constants[name]
	return ok
}

// RemoveFunction removes a function by name.
func (b *LibraryBuilder) RemoveFunction(name string) *LibraryBuilder {
	delete(b.functions, name)
	return b
}

// RemoveConstant removes a constant by name.
func (b *LibraryBuilder) RemoveConstant(name string) *LibraryBuilder {
	delete(b.constants, name)
	return b
}

// FunctionCount returns the number of registered functions.
func (b *LibraryBuilder) FunctionCount() int {
	return len(b.functions)
}

// ConstantCount returns the number of registered constants.
func (b *LibraryBuilder) ConstantCount() int {
	return len(b.constants)
}

// Clear removes all registered functions and constants.
func (b *LibraryBuilder) Clear() *LibraryBuilder {
	b.functions = make(map[string]*Builtin)
	b.constants = make(map[string]Object)
	return b
}

// Merge merges another builder's functions and constants into this one.
// If there are conflicts, the other builder's values take precedence.
func (b *LibraryBuilder) Merge(other *LibraryBuilder) *LibraryBuilder {
	for name, fn := range other.functions {
		b.functions[name] = fn
	}
	for name, c := range other.constants {
		b.constants[name] = c
	}
	return b
}

// GetFunctionNames returns a sorted list of registered function names.
func (b *LibraryBuilder) GetFunctionNames() []string {
	names := make([]string, 0, len(b.functions))
	for name := range b.functions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetConstantNames returns a sorted list of registered constant names.
func (b *LibraryBuilder) GetConstantNames() []string {
	names := make([]string, 0, len(b.constants))
	for name := range b.constants {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// String returns a string representation of the builder's state.
func (b *LibraryBuilder) String() string {
	var sb strings.Builder
	sb.WriteString("LibraryBuilder(")
	sb.WriteString(b.name)
	sb.WriteString("):\n")
	sb.WriteString("  Functions: ")
	sb.WriteString(strconv.Itoa(len(b.functions)))
	sb.WriteString("\n  Constants: ")
	sb.WriteString(strconv.Itoa(len(b.constants)))
	return sb.String()
}
