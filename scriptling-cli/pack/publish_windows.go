//go:build windows

package pack

import "golang.org/x/sys/windows"

// publishPackage atomically publishes a completed sibling temporary file.
// MoveFileEx without REPLACE_EXISTING supplies fail-if-present semantics;
// force mode adds replacement so Windows does not need a remove/rename gap.
func publishPackage(tmp, dst string, force bool) error {
	from, err := windows.UTF16PtrFromString(tmp)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	flags := uint32(windows.MOVEFILE_WRITE_THROUGH)
	if force {
		flags |= windows.MOVEFILE_REPLACE_EXISTING
	}
	return windows.MoveFileEx(from, to, flags)
}
