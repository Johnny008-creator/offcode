package sysmon

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

var kernel32 = windows.NewLazySystemDLL("kernel32.dll")
var getSystemTimes = kernel32.NewProc("GetSystemTimes")
var globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")

func readCPUPlatform(prev CPUSample) (float64, CPUSample, bool) {
	var idle, kernel, user windows.Filetime
	ok, _, _ := getSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ok == 0 {
		return 0, prev, false
	}
	ticks := func(f windows.Filetime) uint64 { return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime) }
	next := CPUSample{idle: ticks(idle), total: ticks(kernel) + ticks(user), valid: true}
	if !prev.valid || next.total <= prev.total || next.idle < prev.idle {
		return 0, next, true
	}
	total, idleDelta := next.total-prev.total, next.idle-prev.idle
	if idleDelta > total {
		return 0, next, false
	}
	return 100 * float64(total-idleDelta) / float64(total), next, true
}

func readMemoryPlatform() (Memory, bool) {
	var status struct {
		Length, Load                                                                                         uint32
		TotalPhys, AvailPhys, TotalPageFile, AvailPageFile, TotalVirtual, AvailVirtual, AvailExtendedVirtual uint64
	}
	status.Length = uint32(unsafe.Sizeof(status))
	ok, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if ok == 0 || status.TotalPhys == 0 || status.AvailPhys > status.TotalPhys {
		return Memory{}, false
	}
	return Memory{UsedBytes: status.TotalPhys - status.AvailPhys, TotalBytes: status.TotalPhys}, true
}

// Vendor-specific GPU telemetry is not available through these Win32 APIs.
func ReadGPU() (GPU, bool) { return GPU{}, false }
