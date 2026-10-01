package ast

import "testing"

func TestAllocSizeMatchesSizeClasses(t *testing.T) {
	cases := map[int]int{0: 0, 1: 8, 8: 8, 9: 16, 24: 24, 25: 32, 33: 48, 56: 64, 72: 80, 100: 112, 129: 144, 256: 256, 257: 288, 1024: 1024, 1025: 1152, 32768: 32768, 32769: 40960}
	for n, want := range cases {
		if got := allocSize(n); got != want {
			t.Errorf("allocSize(%d) = %d, want %d", n, got, want)
		}
	}
	// The small-size table must agree with the scan for every size it covers.
	for n := 1; n <= smallAllocMax; n++ {
		want := 0
		for _, c := range sizeClasses {
			if n <= c {
				want = c
				break
			}
		}
		if got := allocSize(n); got != want {
			t.Fatalf("allocSize(%d) = %d, want %d", n, got, want)
		}
	}
}
