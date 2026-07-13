//go:build windows

package extlibs

import "errors"

// diskUsageStat is a stub on Windows — Scriptling targets Unix platforms.
func diskUsageStat(path string) (total, used, free int64, err error) {
	return 0, 0, 0, errors.New("disk_usage is not supported on Windows")
}
