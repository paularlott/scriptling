package object

import (
	"context"
	"fmt"
	"reflect"
)

var (
	instanceType = reflect.TypeOf((*Instance)(nil))
)

// ClassBuilder provides a fluent API for creating scriptling classes.
// It allows registering typed Go methods that are automatically wrapped
// to handle conversion between Go types and scriptling Objects.
//
// Example usage:
//
//	cb := NewClassBuilder("Person")
//	cb.Method("greet", func(self *Instance, name string) string {
//	    return "Hello, " + name
//	})
//	class := cb.Build()
type ClassBuilder struct {
	name      string
	baseClass *Class
	methods   map[string]*Builtin
	env       *Environment
}

// NewClassBuilder creates a new ClassBuilder with the given class name.
func NewClassBuilder(name string) *ClassBuilder {
	return &ClassBuilder{
		name:    name,
		methods: make(map[string]*Builtin),
	}
}

// BaseClass sets the base class for inheritance.
func (cb *ClassBuilder) BaseClass(base *Class) *ClassBuilder {
	cb.baseClass = base
	return cb
}

// Method registers a typed Go method with the class.
// The method must be a Go function with typed parameters.
// The first parameter should be *Instance (the 'self' parameter).
// Supported signatures are the same as LibraryBuilder.Function().
//
// Example:
//
//	cb.Method("greet", func(self *Instance, name string) string {
//	    return "Hello, " + name
//	})
func (cb *ClassBuilder) Method(name string, fn interface{}) *ClassBuilder {
	cb.MethodWithHelp(name, fn, "")
	return cb
}

// MethodWithHelp registers a method with help text.
// Help text is displayed when users call help() on the method.
//
// Example:
//
//	cb.MethodWithHelp("sqrt", func(self *Instance, x float64) float64 {
//	    return math.Sqrt(x)
//	}, "sqrt(x) - Return the square root of x")
func (cb *ClassBuilder) MethodWithHelp(name string, fn interface{}, helpText string) *ClassBuilder {
	wrapper := cb.createWrapper(fn, helpText)
	cb.methods[name] = wrapper
	return cb
}

// Environment sets the environment for the class.
// This is optional and usually not needed.
func (cb *ClassBuilder) Environment(env *Environment) *ClassBuilder {
	cb.env = env
	return cb
}

// Build creates and returns the Class from this builder.
func (cb *ClassBuilder) Build() *Class {
	return &Class{
		Name:      cb.name,
		BaseClass: cb.baseClass,
		Methods:   cb.convertMethodsToObjects(),
		Env:       cb.env,
	}
}

// convertMethodsToObjects converts the methods map to map[string]Object
func (cb *ClassBuilder) convertMethodsToObjects() map[string]Object {
	result := make(map[string]Object, len(cb.methods))
	for name, builtin := range cb.methods {
		result[name] = builtin
	}
	return result
}

// createWrapper creates a Builtin wrapper for a typed Go method.
// This is adapted from LibraryBuilder.createWrapper for methods.
func (cb *ClassBuilder) createWrapper(fn interface{}, helpText string) *Builtin {
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	// Validate that it's a function
	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf("ClassBuilder: must be a function, got %T", fnValue.Interface()))
	}

	// Analyze function signature once (cached)
	sig := analyzeClassMethodSignature(fnType)

	return &Builtin{
		Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) Object {
			return cb.callTypedMethod(fnValue, sig, ctx, kwargs, args)
		},
		HelpText: helpText,
	}
}
// callTypedMethod calls a typed Go method with cached signature info.
// For class methods, self is ALWAYS first, then: [context], [kwargs], ...args
func (cb *ClassBuilder) callTypedMethod(fnValue reflect.Value, sig *FunctionSignature, ctx context.Context, kwargs Kwargs, args []Object) Object {
	if len(args) == 0 {
		return newError("method call requires at least one argument (instance)")
	}

	// The first argument is always the instance
	instance, ok := args[0].(*Instance)
	if !ok {
		return newError("first argument must be an instance, got %T", args[0])
	}
	methodArgs := args[1:]

	// Get pooled slice for arguments
	argValuesPtr := getArgValueSlice(sig.numIn)
	argValues := *argValuesPtr

	// Build arguments in the order the Go function expects:
	// self, [ctx], [kwargs], ...args

	// Add the instance parameter (always first)
	argValues = append(argValues, reflect.ValueOf(instance))

	// Add context parameter if present (second)
	if sig.hasContext {
		argValues = append(argValues, reflect.ValueOf(ctx))
	}

	// Add kwargs parameter if present (third)
	if sig.hasKwargs {
		argValues = append(argValues, reflect.ValueOf(kwargs))
	}

	// Now add the method arguments
	// sig.maxPosArgs already accounts for self, context, and kwargs offset
	argIndex := 0
	expectedArgs := sig.maxPosArgs

	for i := 0; i < expectedArgs; i++ {
		fnParamIndex := i + sig.paramOffset

		if sig.isVariadic && fnParamIndex == sig.variadicIndex {
			// Variadic parameters - collect remaining args
			elemType := sig.paramTypes[fnParamIndex].Elem()
			for j := argIndex; j < len(methodArgs); j++ {
				val, convErr := convertObjectToValue(methodArgs[j], elemType)
				if convErr != nil {
					putArgValueSlice(argValuesPtr, sig.numIn)
					return convErr
				}
				argValues = append(argValues, val)
			}
			break
		}

		if argIndex >= len(methodArgs) {
			putArgValueSlice(argValuesPtr, sig.numIn)
			return newArgumentError(len(methodArgs), expectedArgs)
		}

		// Use cached parameter type
		val, convErr := convertObjectToValue(methodArgs[argIndex], sig.paramTypes[fnParamIndex])
		if convErr != nil {
			putArgValueSlice(argValuesPtr, sig.numIn)
			return convErr
		}
		argValues = append(argValues, val)
		argIndex++
	}

	// Check if we have extra positional arguments
	if argIndex < len(methodArgs) && !sig.isVariadic {
		putArgValueSlice(argValuesPtr, sig.numIn)
		return newArgumentError(len(methodArgs), expectedArgs)
	}

	// Call the method
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
		return newError("method can return at most 2 values")
	}
}

// analyzeClassMethodSignature analyzes a function signature for class methods.
// For class methods: self (required), [context], [kwargs], ...args
// Valid signatures:
//   - func(self *Instance, ...args)
//   - func(self *Instance, ctx context.Context, ...args)
//   - func(self *Instance, kwargs Kwargs, ...args)
//   - func(self *Instance, ctx context.Context, kwargs Kwargs, ...args)
func analyzeClassMethodSignature(fnType reflect.Type) *FunctionSignature {
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

	// For class methods: self is REQUIRED as first parameter
	if numIn == 0 || fnType.In(0) != instanceType {
		panic(fmt.Sprintf("class method must have *Instance as first parameter, got %v", fnType))
	}
	paramOffset := 1 // Skip self

	// After self: [context], [kwargs], ...args (all optional)
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
