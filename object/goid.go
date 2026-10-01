//go:build amd64 || arm64

package object

// goid returns an identifier for the calling goroutine that is unique among
// all goroutines alive at the same time and never zero. The interpreter lock
// uses it only to recognise re-entrant acquisition and to let the holder
// release the lock around blocking calls; it is never exposed to scripts or
// persisted.
//
// On amd64 and arm64 the identifier is the address of the runtime's goroutine
// descriptor, read directly from the g register (see goid_amd64.s and
// goid_arm64.s). That costs well under a nanosecond. Other architectures fall
// back to parsing the runtime stack header, which forces a full stack unwind
// and costs microseconds per call (see goid_fallback.go).
//
// The descriptor address can be reused by a later goroutine once the current
// one exits, unlike the runtime's numeric id. That is safe here because every
// holder clears the owner field before its goroutine can exit: EnterGIL is
// always paired with ExitGIL, and RunUnlocked restores ownership before
// returning. A goroutine that exited while still owning the lock would have
// left the mutex locked forever regardless of how it was identified.
func goid() int64 {
	return int64(getg())
}

// getg returns the address of the current goroutine's runtime descriptor.
// Implemented in goid_amd64.s and goid_arm64.s.
func getg() uintptr
