//go:build windows

package procstats

import (
	"syscall"
	"time"
)

// CPUTimes holds cumulative CPU time for the current process.
type CPUTimes struct {
	User time.Duration
	Sys  time.Duration
}

// ReadCPUTimes returns user and system CPU time for the current process.
func ReadCPUTimes() CPUTimes {
	var creation, exit, kernel, user syscall.Filetime
	h, _ := syscall.GetCurrentProcess()
	if err := syscall.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return CPUTimes{}
	}
	kernelNs := int64(kernel.HighDateTime)<<32 | int64(kernel.LowDateTime)
	userNs := int64(user.HighDateTime)<<32 | int64(user.LowDateTime)
	return CPUTimes{
		User: time.Duration(userNs) * 100,
		Sys:  time.Duration(kernelNs) * 100,
	}
}
