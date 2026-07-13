//go:build !windows

package extlibs

import "golang.org/x/sys/unix"

// diskUsageStat returns total, used, and free bytes for the file system
// containing path. On Unix this uses statfs(2).
func diskUsageStat(path string) (total, used, free int64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, 0, 0, err
	}
	total = int64(stat.Blocks) * int64(stat.Bsize)
	free = int64(stat.Bavail) * int64(stat.Bsize)
	used = total - free
	return total, used, free, nil
}
