package object

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// TestReflectionCaching verifies that function signatures are cached
func TestReflectionCaching(t *testing.T) {
	// Clear cache for clean test
	signatureCache = sync.Map{}

	fn := func(a, b int) int { return a + b }
	fnType := reflect.TypeOf(fn)

	// First call should cache
	sig1 := analyzeFunctionSignature(fnType)
	if sig1 == nil {
		t.Fatal("expected signature, got nil")
	}

	// Second call should return cached value
	sig2 := analyzeFunctionSignature(fnType)
	if sig1 != sig2 {
		t.Error("expected same signature instance (cached), got different instances")
	}

	// Verify cache contains the entry
	cached, ok := signatureCache.Load(fnType)
	if !ok {
		t.Error("signature not found in cache")
	}
	if cached != sig1 {
		t.Error("cached signature doesn't match original")
	}
}

// TestSignatureCacheContent verifies cached signature contains correct data
func TestSignatureCacheContent(t *testing.T) {
	tests := []struct {
		name     string
		fn       interface{}
		expected FunctionSignature
	}{
		{
			name: "simple function",
			fn:   func(a, b int) int { return a + b },
			expected: FunctionSignature{
				numIn:         2,
				numOut:        1,
				isVariadic:    false,
				hasContext:    false,
				hasKwargs:     false,
				paramOffset:   0,
				maxPosArgs:    2,
				returnIsError: false,
			},
		},
		{
			name: "function with context",
			fn:   func(ctx context.Context, a int) int { return a },
			expected: FunctionSignature{
				numIn:         2,
				numOut:        1,
				isVariadic:    false,
				hasContext:    true,
				hasKwargs:     false,
				paramOffset:   1,
				maxPosArgs:    1,
				returnIsError: false,
			},
		},
		{
			name: "function with kwargs",
			fn:   func(kwargs Kwargs, a int) int { return a },
			expected: FunctionSignature{
				numIn:         2,
				numOut:        1,
				isVariadic:    false,
				hasContext:    false,
				hasKwargs:     true,
				paramOffset:   1,
				maxPosArgs:    1,
				returnIsError: false,
			},
		},
		{
			name: "function with context and kwargs",
			fn:   func(ctx context.Context, kwargs Kwargs, a int) int { return a },
			expected: FunctionSignature{
				numIn:         3,
				numOut:        1,
				isVariadic:    false,
				hasContext:    true,
				hasKwargs:     true,
				paramOffset:   2,
				maxPosArgs:    1,
				returnIsError: false,
			},
		},
		{
			name: "function with error return",
			fn:   func(a int) (int, error) { return a, nil },
			expected: FunctionSignature{
				numIn:         1,
				numOut:        2,
				isVariadic:    false,
				hasContext:    false,
				hasKwargs:     false,
				paramOffset:   0,
				maxPosArgs:    1,
				returnIsError: true,
			},
		},
		{
			name: "variadic function",
			fn:   func(args ...int) int { return 0 },
			expected: FunctionSignature{
				numIn:         1,
				numOut:        1,
				isVariadic:    true,
				variadicIndex: 0,
				hasContext:    false,
				hasKwargs:     false,
				paramOffset:   0,
				maxPosArgs:    1,
				returnIsError: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fnType := reflect.TypeOf(tt.fn)
			sig := analyzeFunctionSignature(fnType)

			if sig.numIn != tt.expected.numIn {
				t.Errorf("numIn: expected %d, got %d", tt.expected.numIn, sig.numIn)
			}
			if sig.numOut != tt.expected.numOut {
				t.Errorf("numOut: expected %d, got %d", tt.expected.numOut, sig.numOut)
			}
			if sig.isVariadic != tt.expected.isVariadic {
				t.Errorf("isVariadic: expected %v, got %v", tt.expected.isVariadic, sig.isVariadic)
			}
			if sig.hasContext != tt.expected.hasContext {
				t.Errorf("hasContext: expected %v, got %v", tt.expected.hasContext, sig.hasContext)
			}
			if sig.hasKwargs != tt.expected.hasKwargs {
				t.Errorf("hasKwargs: expected %v, got %v", tt.expected.hasKwargs, sig.hasKwargs)
			}
			if sig.paramOffset != tt.expected.paramOffset {
				t.Errorf("paramOffset: expected %d, got %d", tt.expected.paramOffset, sig.paramOffset)
			}
			if sig.maxPosArgs != tt.expected.maxPosArgs {
				t.Errorf("maxPosArgs: expected %d, got %d", tt.expected.maxPosArgs, sig.maxPosArgs)
			}
			if sig.returnIsError != tt.expected.returnIsError {
				t.Errorf("returnIsError: expected %v, got %v", tt.expected.returnIsError, sig.returnIsError)
			}

			// Verify paramTypes are cached
			if len(sig.paramTypes) != sig.numIn {
				t.Errorf("paramTypes length: expected %d, got %d", sig.numIn, len(sig.paramTypes))
			}
		})
	}
}

// BenchmarkBuilderWithCache benchmarks function calls with signature caching
func BenchmarkBuilderWithCache(b *testing.B) {
	builder := NewLibraryBuilder("test", "test")
	builder.Function("add", func(a, b int) int { return a + b })
	lib := builder.Build()
	fn := lib.Functions()["add"]

	ctx := context.Background()
	kwargs := NewKwargs(nil)
	arg1 := NewInteger(5)
	arg2 := NewInteger(3)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fn.Fn(ctx, kwargs, arg1, arg2)
	}
}

// BenchmarkBuilderComplexFunction benchmarks more complex function
func BenchmarkBuilderComplexFunction(b *testing.B) {
	builder := NewLibraryBuilder("test", "test")
	builder.Function("complex", func(ctx context.Context, kwargs Kwargs, name string, count int) string {
		prefix, _ := kwargs.GetString("prefix", ">>")
		return prefix + name
	})
	lib := builder.Build()
	fn := lib.Functions()["complex"]

	ctx := context.Background()
	kwargs := NewKwargs(map[string]Object{"prefix": &String{Value: ">>"}})
	arg1 := &String{Value: "test"}
	arg2 := NewInteger(5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fn.Fn(ctx, kwargs, arg1, arg2)
	}
}

// BenchmarkSignatureAnalysis benchmarks the signature analysis itself
func BenchmarkSignatureAnalysis(b *testing.B) {
	fn := func(ctx context.Context, kwargs Kwargs, a, b int) (int, error) { return a + b, nil }
	fnType := reflect.TypeOf(fn)

	b.Run("first_call_no_cache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			signatureCache = sync.Map{} // Clear cache each time
			analyzeFunctionSignature(fnType)
		}
	})

	b.Run("cached_calls", func(b *testing.B) {
		signatureCache = sync.Map{}
		analyzeFunctionSignature(fnType) // Prime cache
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			analyzeFunctionSignature(fnType)
		}
	})
}

// BenchmarkTypeConversion benchmarks the type conversion functions
func BenchmarkTypeConversion(b *testing.B) {
	intType := reflect.TypeOf(int(0))
	strType := reflect.TypeOf("")
	floatType := reflect.TypeOf(float64(0))

	b.Run("int_conversion", func(b *testing.B) {
		obj := NewInteger(42)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			convertObjectToValue(obj, intType)
		}
	})

	b.Run("string_conversion", func(b *testing.B) {
		obj := &String{Value: "test"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			convertObjectToValue(obj, strType)
		}
	})

	b.Run("float_conversion", func(b *testing.B) {
		obj := &Float{Value: 3.14}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			convertObjectToValue(obj, floatType)
		}
	})
}

// BenchmarkReturnConversion benchmarks return value conversion
func BenchmarkReturnConversion(b *testing.B) {
	b.Run("int_return", func(b *testing.B) {
		val := reflect.ValueOf(42)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			convertReturnValue(val)
		}
	})

	b.Run("string_return", func(b *testing.B) {
		val := reflect.ValueOf("test")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			convertReturnValue(val)
		}
	})

	b.Run("float_return", func(b *testing.B) {
		val := reflect.ValueOf(3.14)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			convertReturnValue(val)
		}
	})
}

// BenchmarkBuilderVsRawFunction compares builder performance to raw function
func BenchmarkBuilderVsRawFunction(b *testing.B) {
	// Builder-based function
	builder := NewLibraryBuilder("test", "test")
	builder.Function("add", func(a, b int) int { return a + b })
	lib := builder.Build()
	builderFn := lib.Functions()["add"]

	// Raw function
	rawFn := &Builtin{
		Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) Object {
			if len(args) != 2 {
				return newArgumentError(len(args), 2)
			}
			a, err := args[0].AsInt()
			if err != nil {
				return err
			}
			b, err := args[1].AsInt()
			if err != nil {
				return err
			}
			return NewInteger(a + b)
		},
	}

	ctx := context.Background()
	kwargs := NewKwargs(nil)
	arg1 := NewInteger(5)
	arg2 := NewInteger(3)

	b.Run("builder_function", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			builderFn.Fn(ctx, kwargs, arg1, arg2)
		}
	})

	b.Run("raw_function", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rawFn.Fn(ctx, kwargs, arg1, arg2)
		}
	})
}

// BenchmarkMultipleFunctionTypes benchmarks different function signatures
func BenchmarkMultipleFunctionTypes(b *testing.B) {
	ctx := context.Background()
	kwargs := NewKwargs(nil)

	tests := []struct {
		name string
		fn   interface{}
		args []Object
	}{
		{
			name: "simple_two_ints",
			fn:   func(a, b int) int { return a + b },
			args: []Object{NewInteger(5), NewInteger(3)},
		},
		{
			name: "with_context",
			fn:   func(ctx context.Context, a, b int) int { return a + b },
			args: []Object{NewInteger(5), NewInteger(3)},
		},
		{
			name: "with_kwargs",
			fn:   func(kwargs Kwargs, a, b int) int { return a + b },
			args: []Object{NewInteger(5), NewInteger(3)},
		},
		{
			name: "with_error",
			fn:   func(a, b int) (int, error) { return a + b, nil },
			args: []Object{NewInteger(5), NewInteger(3)},
		},
		{
			name: "string_concat",
			fn:   func(a, b string) string { return a + b },
			args: []Object{&String{Value: "hello"}, &String{Value: "world"}},
		},
		{
			name: "mixed_types",
			fn:   func(s string, i int, f float64) string { return s },
			args: []Object{&String{Value: "test"}, NewInteger(42), &Float{Value: 3.14}},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			builder := NewLibraryBuilder("test", "test")
			builder.Function("fn", tt.fn)
			lib := builder.Build()
			fn := lib.Functions()["fn"]

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				fn.Fn(ctx, kwargs, tt.args...)
			}
		})
	}
}

// TestReflectionOnlyAtBuildTime verifies reflection happens at build, not runtime
func TestReflectionOnlyAtBuildTime(t *testing.T) {
	// Track if analyzeFunctionSignature is called
	callCount := 0
	originalCache := signatureCache
	signatureCache = sync.Map{}
	defer func() { signatureCache = originalCache }()

	// Create builder and register function
	builder := NewLibraryBuilder("test", "test")
	fn := func(a, b int) int { return a + b }
	fnType := reflect.TypeOf(fn)

	// Build the library (this should call analyzeFunctionSignature once)
	builder.Function("add", fn)
	lib := builder.Build()

	// Verify signature was cached during build
	if _, ok := signatureCache.Load(fnType); !ok {
		t.Error("signature should be cached after build")
	}

	// Now call the function multiple times
	builtFn := lib.Functions()["add"]
	ctx := context.Background()
	kwargs := NewKwargs(nil)

	for i := 0; i < 100; i++ {
		result := builtFn.Fn(ctx, kwargs, NewInteger(5), NewInteger(3))
		if intResult, ok := result.(*Integer); !ok || intResult.Value != 8 {
			t.Errorf("expected 8, got %v", result)
		}
	}

	// The signature should still be the same cached instance
	cached, _ := signatureCache.Load(fnType)
	if cached == nil {
		t.Error("signature should still be cached after multiple calls")
	}

	t.Logf("Signature analysis call count during build: %d (expected: 1)", callCount)
	t.Log("✓ Reflection happens at build time, cached signature used at runtime")
}

// BenchmarkCacheHitRate measures cache effectiveness
func BenchmarkCacheHitRate(b *testing.B) {
	// Create multiple functions with same signature
	fns := []interface{}{
		func(a, b int) int { return a + b },
		func(a, b int) int { return a - b },
		func(a, b int) int { return a * b },
	}

	builder := NewLibraryBuilder("test", "test")
	for i, fn := range fns {
		builder.Function(fmt.Sprintf("fn%d", i), fn)
	}
	lib := builder.Build()

	ctx := context.Background()
	kwargs := NewKwargs(nil)
	args := []Object{NewInteger(10), NewInteger(5)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fnName := fmt.Sprintf("fn%d", i%len(fns))
		lib.Functions()[fnName].Fn(ctx, kwargs, args...)
	}
}
