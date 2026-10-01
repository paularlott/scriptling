package object

import (
	"sync"
	"testing"
)

// goid must be stable within a goroutine, non-zero, and distinct across
// goroutines that are alive at the same time: that is exactly what the
// interpreter lock relies on to detect re-entrant acquisition.
func TestGoidStableAndDistinct(t *testing.T) {
	self := goid()
	if self == 0 {
		t.Fatal("goid returned 0")
	}
	if again := goid(); again != self {
		t.Fatalf("goid changed within one goroutine: %d then %d", self, again)
	}

	const n = 64
	ids := make([]int64, n)
	var start, done sync.WaitGroup
	start.Add(1)
	done.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer done.Done()
			start.Wait() // keep every goroutine alive until all ids are taken
			ids[i] = goid()
		}(i)
	}
	// Release all goroutines together so their lifetimes overlap.
	start.Done()
	done.Wait()

	seen := map[int64]bool{self: true}
	for i, id := range ids {
		if id == 0 {
			t.Fatalf("goroutine %d: goid returned 0", i)
		}
		if seen[id] {
			t.Fatalf("goroutine %d: goid %d collides with another live goroutine", i, id)
		}
		seen[id] = true
	}
}

func BenchmarkGoid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = goid()
	}
}
