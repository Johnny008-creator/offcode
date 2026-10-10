//go:build !windows

// Package sysmon reads local system load (CPU, memory, GPU) for the
// offcode monitor view. Readers are Linux-focused today (procfs/sysfs) —
// on any other OS, or if a reading isn't available, they return ok=false
// rather than erroring, so the monitor just shows "n/a" for that panel.
package sysmon

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadCPU returns CPU usage percent since the previous sample. Pass the
// zero CPUSample on the first call; from then on, pass back whatever this
// function last returned.
func readCPUPlatform(prev CPUSample) (percent float64, next CPUSample, ok bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, prev, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, prev, false
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, prev, false
	}

	var total uint64
	var idle uint64
	for i, field := range fields[1:] {
		v, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			continue
		}
		total += v
		if i == 3 { // idle
			idle = v
		}
	}

	next = CPUSample{idle: idle, total: total, valid: true}
	if !prev.valid {
		return 0, next, true
	}

	totalDelta := total - prev.total
	idleDelta := idle - prev.idle
	if totalDelta == 0 {
		return 0, next, true
	}
	percent = float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	return percent, next, true
}

func readMemoryPlatform() (Memory, bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return Memory{}, false
	}
	defer f.Close()

	var totalKB, availKB uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			totalKB = v
		case "MemAvailable:":
			availKB = v
		}
	}
	if totalKB == 0 {
		return Memory{}, false
	}
	return Memory{
		UsedBytes:  (totalKB - availKB) * 1024,
		TotalBytes: totalKB * 1024,
	}, true
}

var cachedGPUCardDir string
var cachedGPUName string
var gpuProbed bool

// ReadGPU looks for the first AMD GPU exposing amdgpu's sysfs stats
// (gpu_busy_percent). Returns ok=false if none is found (non-Linux,
// non-AMD, or a kernel/driver without these files).
func ReadGPU() (GPU, bool) {
	if !gpuProbed {
		cachedGPUCardDir = findAMDGPUCardDir()
		cachedGPUName = gpuDisplayName()
		gpuProbed = true
	}
	if cachedGPUDir := cachedGPUCardDir; cachedGPUDir != "" {
		return readAMDGPU(cachedGPUDir, cachedGPUName)
	}
	return GPU{}, false
}

func findAMDGPUCardDir() string {
	matches, err := filepath.Glob("/sys/class/drm/card*/device/gpu_busy_percent")
	if err != nil || len(matches) == 0 {
		return ""
	}
	return filepath.Dir(matches[0])
}

func gpuDisplayName() string {
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return "GPU"
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "VGA compatible controller") && !strings.Contains(line, "Display controller") {
			continue
		}
		// Prefer the last [bracketed] marketing name (e.g. lspci prints
		// "... [AMD/ATI] Navi 24 [Radeon RX 6500 XT] (rev c1)") over the
		// full vendor/codename string, which is too long for a status line.
		last := -1
		for i := 0; i < len(line); i++ {
			if line[i] == '[' {
				last = i
			}
		}
		if last != -1 {
			if end := strings.IndexByte(line[last:], ']'); end != -1 {
				return line[last+1 : last+end]
			}
		}
		if idx := strings.Index(line, ": "); idx != -1 {
			return strings.TrimSpace(line[idx+2:])
		}
	}
	return "GPU"
}

func readAMDGPU(dir, name string) (GPU, bool) {
	busy, ok := readUintFile(filepath.Join(dir, "gpu_busy_percent"))
	if !ok {
		return GPU{}, false
	}
	vramUsed, _ := readUintFile(filepath.Join(dir, "mem_info_vram_used"))
	vramTotal, _ := readUintFile(filepath.Join(dir, "mem_info_vram_total"))

	g := GPU{
		Name:        name,
		BusyPercent: float64(busy),
		VRAMUsed:    vramUsed,
		VRAMTotal:   vramTotal,
	}

	hwmonDirs, _ := filepath.Glob(filepath.Join(dir, "hwmon", "hwmon*"))
	if len(hwmonDirs) > 0 {
		if temp, ok := readUintFile(filepath.Join(hwmonDirs[0], "temp1_input")); ok {
			g.TempC = float64(temp) / 1000
		}
		if power, ok := readUintFile(filepath.Join(hwmonDirs[0], "power1_average")); ok {
			g.PowerW = float64(power) / 1_000_000
		}
	}
	return g, true
}

func readUintFile(path string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
