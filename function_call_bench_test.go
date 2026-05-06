package scriptling

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
)

func BenchmarkFunctionCallSimple(b *testing.B) {
	p := New()
	_, err := p.Eval(`
def simple_func(x):
    return x * 2 + 1
`)
	if err != nil {
		b.Fatal(err)
	}

	fn, errObj := p.GetVarAsObject("simple_func")
	if errObj != nil {
		b.Fatal(errObj)
	}

	arg := object.NewInteger(42)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result := evaluator.ApplyFunction(context.Background(), fn, []object.Object{arg}, nil, p.env)
		if object.IsError(result) {
			b.Fatal(result.Inspect())
		}
	}
}

func BenchmarkFunctionCallFib20(b *testing.B) {
	p := New()
	_, err := p.Eval(`
def fib(n):
    if n < 2:
        return n
    return fib(n - 1) + fib(n - 2)
`)
	if err != nil {
		b.Fatal(err)
	}

	fn, errObj := p.GetVarAsObject("fib")
	if errObj != nil {
		b.Fatal(errObj)
	}

	arg := object.NewInteger(20)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result := evaluator.ApplyFunction(context.Background(), fn, []object.Object{arg}, nil, p.env)
		if object.IsError(result) {
			b.Fatal(result.Inspect())
		}
	}
}
