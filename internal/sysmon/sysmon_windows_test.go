package sysmon

import "testing"

func TestWindowsSystemCounters(t *testing.T) {
	memory, ok := ReadMemory()
	if !ok || memory.TotalBytes == 0 || memory.UsedBytes > memory.TotalBytes {
		t.Fatalf("invalid memory counters: %+v, %v", memory, ok)
	}
	_, sample, ok := ReadCPU(CPUSample{})
	if !ok || !sample.valid {
		t.Fatal("GetSystemTimes failed")
	}
	percent, _, ok := ReadCPU(sample)
	if !ok || percent < 0 || percent > 100 {
		t.Fatalf("invalid CPU sample: %f, %v", percent, ok)
	}
}
