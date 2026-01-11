package object

import (
	"context"
	"fmt"
	"reflect"
)

// FunctionBuilder provides a fluent API for creating individual scriptling functions.
// It allows registering a single typed Go function that is automatically wrapped
// to handle conversion between Go types and scriptling Objects.
//
// Example usage:
//
//	fb := NewFunctionBuilder()
//	fb.Function(func(a, b int) int { return a + b })
//	fn := fb.Build()
//	p.RegisterFunc("add", fn)
type FunctionBuilder struct {
	fn       *Builtin
	hasFunc  bool
}

// NewFunctionBuilder creates a new FunctionBuilder for building individual functions.
func NewFunctionBuilder() *FunctionBuilder {
	return &FunctionBuilder{}
}

// Function registers a typed Go function with the builder.
// The function must be a Go function with typed parameters.
// Supported signatures are the same as LibraryBuilder.Function().
//
// Example:
//
//	fb.Function(func(a, b int) int { return a + b })
func (fb *FunctionBuilder) Function(fn interface{}) *FunctionBuilder {
	fb.FunctionWithHelp(fn, "")
	return fb
}

// FunctionWithHelp registers a function with help text.
// Help text is displayed when users call help() on the function.
//
// Example:
//
//	fb.FunctionWithHelp(func(x float64) float64 {
//	    return math.Sqrt(x)
//	}, "sqrt(x) - Return the square root of x")
func (fb *FunctionBuilder) FunctionWithHelp(fn interface{}, helpText string) *FunctionBuilder {
	if fb.hasFunc {
		panic("FunctionBuilder: only one function can be registered")
	}
	wrapper := fb.createWrapper(reflect.ValueOf(fn), helpText)
	fb.fn = wrapper
	fb.hasFunc = true
	return fb
}

// Build creates and returns the BuiltinFunction from this builder.
// The returned function can be passed directly to RegisterFunc().
func (fb *FunctionBuilder) Build() func(ctx context.Context, kwargs Kwargs, args ...Object) Object {
	if !fb.hasFunc {
		panic("FunctionBuilder: no function registered, call Function() first")
	}
	return fb.fn.Fn
}

// createWrapper creates a Builtin wrapper for a typed Go function.
// This is a copy of LibraryBuilder.createWrapper for FunctionBuilder.
func (fb *FunctionBuilder) createWrapper(fnValue reflect.Value, helpText string) *Builtin {
	fnType := fnValue.Type()

	// Validate that it's a function
	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf("FunctionBuilder: must be a function, got %T", fnValue.Interface()))
	}

	// Analyze function signature once (cached)
	sig := analyzeFunctionSignature(fnType)

	return &Builtin{
		Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) Object {
			return fb.callTypedFunctionUltraFast(fnValue, sig, ctx, kwargs, args)
		},
		HelpText: helpText,
	}
}

// callTypedFunctionUltraFast calls a typed Go function with ultra-fast argument conversion.
// This is a copy of LibraryBuilder.callTypedFunctionUltraFast for FunctionBuilder.
func (fb *FunctionBuilder) callTypedFunctionUltraFast(fnValue reflect.Value, sig *FunctionSignature, ctx context.Context, kwargs Kwargs, args []Object) Object {
	// Pre-allocate argValues with exact capacity
	argValues := make([]reflect.Value, 0, sig.numIn)

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
			varArgs := make([]reflect.Value, 0, len(args)-argIndex)
			elemType := sig.paramTypes[fnParamIndex].Elem()
			for j := argIndex; j < len(args); j++ {
				val, convErr := convertObjectToValue(args[j], elemType)
				if convErr != nil {
					return convErr
				}
				varArgs = append(varArgs, val)
			}
			argValues = append(argValues, varArgs...)
			break
		}

		if argIndex >= len(args) {
			return newArgumentError(len(args), sig.maxPosArgs)
		}

		// Use cached parameter type
		val, convErr := convertObjectToValue(args[argIndex], sig.paramTypes[fnParamIndex])
		if convErr != nil {
			return convErr
		}
		argValues = append(argValues, val)
		argIndex++
	}

	// Check if we have extra positional arguments
	if argIndex < len(args) && !sig.isVariadic {
		return newArgumentError(len(args), sig.maxPosArgs)
	}

	// Call the function
	results := fnValue.Call(argValues)

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