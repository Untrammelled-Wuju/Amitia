package task_runtime

import (
	"runtime"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func TestRound27PIDBudgetsDistinguishLinuxThreadsFromWindowsProcesses(t *testing.T) {
	if got := taskProcessPIDBudget("linux"); got < 16 || got > 64 {
		t.Fatalf("Linux cgroup pids.max must leave headroom for Node/V8 threads: %d", got)
	}
	if got := taskProcessPIDBudget("windows"); got != 1 {
		t.Fatalf("Windows job object child-process budget changed: %d", got)
	}
	if got := taskProcessPIDBudget("darwin"); got != 1 {
		t.Fatalf("unexpected unsupported platform budget: %d", got)
	}
}

func TestRound27CloudAdmissionRefusesUndersizedEnforcedPIDLimit(t *testing.T) {
	support := process.ResourceLimitSupport{Memory: true, CPU: true, Processes: true}
	recommended := taskProcessPIDBudget(runtime.GOOS)
	full := process.ResourceLimits{MaxMemoryBytes: 512 << 20, MaxCPUPercent: 50, MaxProcesses: recommended}
	if err := validateCloudHostResourceLimits(runtimeprofile.ProfileCloudCore, false, full, support, "strict"); err != nil {
		t.Fatalf("supported Node runtime wrongly rejected: %v", err)
	}
	if recommended == 1 {
		t.Skip("the platform limits OS processes rather than kernel threads")
	}
	full.MaxProcesses = 1
	for _, policy := range []string{"strict", "allow-degraded"} {
		if err := validateCloudHostResourceLimits(runtimeprofile.ProfileCloudCore, false, full, support, policy); err == nil || !strings.Contains(err.Error(), "pids") {
			t.Fatalf("cloud runtime admitted pids.max=1 on %s with policy %s: %v", runtime.GOOS, policy, err)
		}
	}
	if err := validateCloudHostResourceLimits(runtimeprofile.ProfileLocal, false, full, support, "strict"); err != nil {
		t.Fatalf("local compatibility regressed: %v", err)
	}
}
