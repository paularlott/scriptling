package stdlib

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func BenchmarkRegexFind(b *testing.B) {
	// Test regex performance with caching
	pattern := &object.String{Value: "[0-9]+"}
	text := &object.String{Value: "abc123def456ghi789"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := reLibrary.Functions()["find"].Fn(context.Background(), pattern, text)
		if result.Type() != object.STRING_OBJ && result.Type() != object.NULL_OBJ {
			b.Errorf("Unexpected result type: %v", result.Type())
		}
	}
}

func BenchmarkRegexFindall(b *testing.B) {
	// Test regex performance with caching
	pattern := &object.String{Value: "[0-9]+"}
	text := &object.String{Value: "abc123def456ghi789"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := reLibrary.Functions()["findall"].Fn(context.Background(), pattern, text)
		if result.Type() != object.LIST_OBJ {
			b.Errorf("Unexpected result type: %v", result.Type())
		}
	}
}
