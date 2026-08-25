package trusted_service

import (
	"testing"

	platformprocess "github.com/u-ai/backend/internal/platform/process"
)

func TestPlatformProcessResourceLimitsClampsAndCountsRootProcess(t *testing.T) {
	limits := platformProcessResourceLimits(ServiceResourceLimits{
		MaxMemoryMB:     512,
		MaxCPUPercent:   500,
		MaxSubprocesses: 3,
	})
	if limits.MaxMemoryBytes != 512*1024*1024 {
		t.Fatalf("MaxMemoryBytes = %d", limits.MaxMemoryBytes)
	}
	if limits.MaxCPUPercent != 100 {
		t.Fatalf("MaxCPUPercent = %d, want 100", limits.MaxCPUPercent)
	}
	if limits.MaxProcesses != 4 {
		t.Fatalf("MaxProcesses = %d, want root + 3 subprocesses", limits.MaxProcesses)
	}
}

func TestEnforcedResourceLimitsDoesNotAdvertiseUnsupportedDeclarations(t *testing.T) {
	got := enforcedResourceLimits(ServiceResourceLimits{
		MaxMemoryMB:        256,
		MaxCPUPercent:      50,
		MaxFileDescriptors: 64,
		MaxDiskMB:          1024,
		MaxSubprocesses:    2,
	}, platformprocess.ResourceLimitSupport{Memory: true, Processes: true})

	if got["max_memory_mb"] != int64(256) {
		t.Fatalf("memory limit missing: %#v", got)
	}
	if got["max_subprocesses"] != 2 {
		t.Fatalf("subprocess limit missing: %#v", got)
	}
	for _, unsupported := range []string{"max_cpu_percent", "max_file_descriptors", "max_disk_mb", "cpu_time_ms"} {
		if _, exists := got[unsupported]; exists {
			t.Fatalf("unsupported limit %s must not be advertised: %#v", unsupported, got)
		}
	}
}
