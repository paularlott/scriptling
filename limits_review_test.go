package scriptling

import (
	"runtime"
	"testing"
)

// Twenty one-byte slices of 1 MiB strings must not keep the 1 MiB strings
// alive while only the slices are charged. Slices that would hold on to far
// more than they use are copied out of their source, so the script holds
// only the slices: it stays well within its 4 MiB limit, and the process heap
// does not keep the sources.
func TestReviewMemoryLimitAccountsForRetainedStringSlices(t *testing.T) {
	p := New(WithMemoryLimit(4<<20), WithMaxValueBytes(0))
	before := heapInUse()
	_, err := p.Eval(`
held = []
n = 1 << 20
for i in range(20):
    whole = "x" * n
    held.append(whole[0:1])
whole = None
`)
	if err != nil {
		t.Fatalf("holding twenty one-byte slices: %v; usage=%+v", err, p.ResourceUsage())
	}
	if grown := int64(heapInUse()) - int64(before); grown > 4<<20 {
		t.Fatalf("twenty one-byte slices kept %d MiB of their sources alive", grown>>20)
	}
	runtime.KeepAlive(p)
}
