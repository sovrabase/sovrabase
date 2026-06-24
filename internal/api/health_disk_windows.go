//go:build windows

package api

import (
	"os"

	"golang.org/x/sys/windows"
)

// getDiskUsage returns disk usage information for the given path using Windows API.
func getDiskUsage(path string) *diskUsage {
	info, err := os.Stat(path)
	if err != nil {
		return &diskUsage{
			Path:        path,
			TotalBytes:  0,
			FreeBytes:   0,
			UsedPercent: 0,
		}
	}

	dir := path
	if !info.IsDir() {
		dir = info.Name()
	}

	absPath, err := os.Getwd()
	if err != nil {
		absPath = dir
	}

	abs, err := windows.UTF16PtrFromString(absPath)
	if err != nil {
		return &diskUsage{Path: path}
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	err = windows.GetDiskFreeSpaceEx(abs, &freeBytesAvailable, &totalBytes, &totalFreeBytes)
	if err != nil {
		return &diskUsage{Path: path}
	}
	_ = freeBytesAvailable

	used := totalBytes - totalFreeBytes
	var usedPercent float64
	if totalBytes > 0 {
		usedPercent = float64(used) / float64(totalBytes) * 100.0
	}

	return &diskUsage{
		Path:        path,
		TotalBytes:  totalBytes,
		FreeBytes:   totalFreeBytes,
		UsedPercent: usedPercent,
	}
}
