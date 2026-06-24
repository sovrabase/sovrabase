//go:build !windows

package api

import "golang.org/x/sys/unix"

// getDiskUsage returns disk usage information for the given path using unix.Statfs.
func getDiskUsage(path string) *diskUsage {
	var stat unix.Statfs_t
	err := unix.Statfs(path, &stat)
	if err != nil {
		return &diskUsage{
			Path:        path,
			TotalBytes:  0,
			FreeBytes:   0,
			UsedPercent: 0,
		}
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	used := totalBytes - freeBytes
	var usedPercent float64
	if totalBytes > 0 {
		usedPercent = float64(used) / float64(totalBytes) * 100.0
	}

	return &diskUsage{
		Path:        path,
		TotalBytes:  totalBytes,
		FreeBytes:   freeBytes,
		UsedPercent: usedPercent,
	}
}
