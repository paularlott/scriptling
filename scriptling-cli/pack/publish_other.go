//go:build !windows

package pack

import "os"

// publishPackage atomically publishes a completed sibling temporary file.
// A hard link supplies create-if-absent semantics without a check/rename race.
func publishPackage(tmp, dst string, force bool) error {
	if force {
		return os.Rename(tmp, dst)
	}
	return os.Link(tmp, dst)
}
