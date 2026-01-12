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
			return callTypedFunction(fnValue, sig, ctx, kwargs, args)
		},
		HelpText: helpText,
	}
}