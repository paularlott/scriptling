package object

import (
	"math"
	"strings"
	"testing"
)

func TestReviewReadAllLimitedHandlesMaxInt64Limit(t *testing.T) {
	withDefaults(t)
	_, guard := boundEnv(t)
	guard.SetMaxValueBytes(math.MaxInt64)

	data, limitErr, err := ReadAllLimited(strings.NewReader("abc"), "body")
	if err != nil || limitErr != nil {
		t.Fatalf("unexpected error: limit=%v read=%v", limitErr, err)
	}
	if string(data) != "abc" {
		t.Fatalf("ReadAllLimited returned %q, want %q", data, "abc")
	}
}

func TestReviewCheckAllocHandlesUsedPlusSizeOverflow(t *testing.T) {
	withDefaults(t)
	_, guard := boundEnv(t)
	guard.SetMaxValueBytes(0)
	guard.SetMemoryLimit(math.MaxInt64)
	guard.used.Store(1)

	if errObj := CheckAlloc(math.MaxInt64); errObj == nil {
		t.Fatal("allocation accepted even though used + size exceeds the memory limit")
	}
}
