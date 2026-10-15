package evaluator

import (
	"strings"
	"testing"
)

func TestReviewValueLimitCoversAllCreatedValues(t *testing.T) {
	t.Run("list literal", func(t *testing.T) {
		_, err := evalLimited(t, `[1, 2, 3, 4, 5]`, valueLimit(64))
		if err == nil || !strings.Contains(err.Error(), "too large") {
			t.Fatalf("five-element list under a 64-byte value limit: %v", err)
		}
	})

	t.Run("bytes base64 expansion", func(t *testing.T) {
		_, err := evalLimited(t, "x = b'x' * 60\nx.base64()", valueLimit(64))
		if err == nil || !strings.Contains(err.Error(), "too large") {
			t.Fatalf("80-byte base64 result under a 64-byte value limit: %v", err)
		}
	})
}
