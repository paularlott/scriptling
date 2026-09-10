package object

import "testing"

// sizeHint must never return a negative capacity hint: a wrapped-negative
// hint panics make (CWE-190, code-scanning alert 46). Overflow degrades to 0
// and the container grows naturally.
func TestSizeHintOverflowGuard(t *testing.T) {
	cases := []struct {
		name string
		a, b int
		want int
	}{
		{"small", 1, 2, 3},
		{"zeros", 0, 0, 0},
		{"max plus one", MaxIntSafe, 1, 0},
		{"max plus max", MaxIntSafe, MaxIntSafe, 0},
		{"one side zero", MaxIntSafe, 0, MaxIntSafe},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sizeHint(tc.a, tc.b)
			if got != tc.want {
				t.Fatalf("sizeHint(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
			if got < 0 {
				t.Fatalf("sizeHint(%d, %d) returned negative hint %d", tc.a, tc.b, got)
			}
		})
	}
}

const MaxIntSafe = int(^uint(0) >> 1)
