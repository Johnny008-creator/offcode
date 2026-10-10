package sysmon

import "fmt"

type CPUSample struct {
	idle, total uint64
	valid       bool
}

type Memory struct {
	UsedBytes  uint64
	TotalBytes uint64
}

type GPU struct {
	Name        string
	BusyPercent float64
	VRAMUsed    uint64
	VRAMTotal   uint64
	TempC       float64
	PowerW      float64
}

func ReadCPU(prev CPUSample) (float64, CPUSample, bool) { return readCPUPlatform(prev) }
func ReadMemory() (Memory, bool)                        { return readMemoryPlatform() }
func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
