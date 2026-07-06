//go:build !windows

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
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return CPUTimes{}
	}
	return CPUTimes{
		User: time.Duration(usage.Utime.Sec)*time.Second + time.Duration(usage.Utime.Usec)*time.Microsecond,
		Sys:  time.Duration(usage.Stime.Sec)*time.Second + time.Duration(usage.Stime.Usec)*time.Microsecond,
	}
}
