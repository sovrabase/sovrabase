//go:build windows

package procstats

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

// ProcessStats holds sampled stats for a specific OS process.
type ProcessStats struct {
	RSSBytes int64
	CPU      CPUTimes
}

var (
	modKernel32              = syscall.NewLazyDLL("kernel32.dll")
	modPsapi                 = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemoryInfo = modPsapi.NewProc("GetProcessMemoryInfo")
	procOpenProcess          = modKernel32.NewProc("OpenProcess")
)

const (
	processQueryInformation = 0x0400
	processVMRead           = 0x0010
)

type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

// ReadProcessStats returns RSS and cumulative CPU for pid.
func ReadProcessStats(pid int) (ProcessStats, error) {
	var out ProcessStats
	handle, _, err := procOpenProcess.Call(
		uintptr(processQueryInformation|processVMRead),
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return out, err
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	var mem processMemoryCounters
	mem.CB = uint32(unsafe.Sizeof(mem))
	r, _, err := procGetProcessMemoryInfo.Call(
		handle,
		uintptr(unsafe.Pointer(&mem)),
		uintptr(mem.CB),
	)
	if r == 0 {
		return out, err
	}
	out.RSSBytes = int64(mem.WorkingSetSize)

	var creation, exit, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(syscall.Handle(handle), &creation, &exit, &kernel, &user); err == nil {
		out.CPU.User = filetimeToDuration(user)
		out.CPU.Sys = filetimeToDuration(kernel)
	}
	return out, nil
}

func filetimeToDuration(ft syscall.Filetime) time.Duration {
	n := int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)
	return time.Duration(n) * 100
}

// ChildCPUTimes extracts child CPU from exec.ProcessState after Wait.
func ChildCPUTimes(state interface{}) CPUTimes {
	ps, ok := state.(*os.ProcessState)
	if !ok || ps == nil {
		return CPUTimes{}
	}
	return CPUTimes{User: ps.UserTime(), Sys: ps.SystemTime()}
}
