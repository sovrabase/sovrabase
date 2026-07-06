//go:build unix

package procstats

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ProcessStats holds sampled stats for a specific OS process.
type ProcessStats struct {
	RSSBytes int64
	CPU      CPUTimes
}

// ReadProcessStats returns RSS and cumulative CPU for pid.
func ReadProcessStats(pid int) (ProcessStats, error) {
	var out ProcessStats
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return out, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				out.RSSBytes = kb * 1024
			}
			break
		}
	}

	statData, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err == nil {
		fields := strings.Fields(string(statData))
		if len(fields) >= 17 {
			utime, _ := strconv.ParseInt(fields[13], 10, 64)
			stime, _ := strconv.ParseInt(fields[14], 10, 64)
			const clkTck = 100
			out.CPU.User = time.Duration(utime) * time.Second / clkTck
			out.CPU.Sys = time.Duration(stime) * time.Second / clkTck
		}
	}
	return out, nil
}

// ChildCPUTimes extracts child CPU from exec.ProcessState after Wait.
func ChildCPUTimes(state interface{}) CPUTimes {
	ps, ok := state.(*os.ProcessState)
	if !ok || ps == nil {
		return CPUTimes{}
	}
	if ru, ok := ps.SysUsage().(*syscall.Rusage); ok && ru != nil {
		return CPUTimes{
			User: time.Duration(ru.Utime.Sec)*time.Second + time.Duration(ru.Utime.Usec)*time.Microsecond,
			Sys:  time.Duration(ru.Stime.Sec)*time.Second + time.Duration(ru.Stime.Usec)*time.Microsecond,
		}
	}
	return CPUTimes{User: ps.UserTime(), Sys: ps.SystemTime()}
}