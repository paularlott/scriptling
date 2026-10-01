//go:build !amd64 && !arm64

package object

import "runtime"

// goid returns the current goroutine's numeric id by parsing the runtime
// stack header. This forces a stack unwind on every call, so it is only used
// on architectures without a direct g-register read (see goid.go).
func goid() int64 {
	var buf [32]byte
	n := runtime.Stack(buf[:], false)
	b := buf[len("goroutine "):n]
	var id int64
	for _, c := range b {
		if c < '0' || c > '9' {
			break
		}
		id = id*10 + int64(c-'0')
	}
	return id
}
