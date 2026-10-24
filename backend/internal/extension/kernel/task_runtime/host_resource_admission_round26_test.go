package task_runtime

import (
	"runtime"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func TestRound26CloudResourceAdmissionRequiresEnforcedBudgets(t *testing.T) {
	full := process.ResourceLimits{MaxMemoryBytes: 512 << 20, MaxCPUPercent: 50, MaxProcesses: taskProcessPIDBudget(runtime.GOOS)}
	all := process.ResourceLimitSupport{Memory: true, CPU: true, Processes: true}
	if err := validateCloudHostResourceLimits(runtimeprofile.ProfileCloudCore, false, full, all, ""); err != nil {
		t.Fatalf("valid cloud resource boundaries rejected: %v", err)
	}
	for _, scenario := range []struct {
		name    string
		limits  process.ResourceLimits
		support process.ResourceLimitSupport
		want    string
	}{
		{"no_memory_backend", full, process.ResourceLimitSupport{CPU: true, Processes: true}, "memory"},
		{"no_cpu_backend", full, process.ResourceLimitSupport{Memory: true, Processes: true}, "cpu"},
		{"no_pid_backend", full, process.ResourceLimitSupport{Memory: true, CPU: true}, "pids"},
		{"zero_memory_budget", process.ResourceLimits{MaxCPUPercent: 50, MaxProcesses: taskProcessPIDBudget(runtime.GOOS)}, all, "memory"},
		{"zero_cpu_budget", process.ResourceLimits{MaxMemoryBytes: 512 << 20, MaxProcesses: taskProcessPIDBudget(runtime.GOOS)}, all, "cpu"},
		{"zero_pid_budget", process.ResourceLimits{MaxMemoryBytes: 512 << 20, MaxCPUPercent: 50}, all, "pids"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			err := validateCloudHostResourceLimits(runtimeprofile.ProfileCloudCore, false, scenario.limits, scenario.support, "strict")
			if err == nil || !strings.Contains(err.Error(), scenario.want) {
				t.Fatalf("accepted unenforced %s boundary: %v", scenario.want, err)
			}
			if err := validateCloudHostResourceLimits(runtimeprofile.ProfileCloudCore, false, scenario.limits, scenario.support, "allow-degraded"); err != nil {
				t.Fatalf("explicit operator override rejected: %v", err)
			}
		})
	}
}

func TestRound26CloudResourceAdmissionRejectsInvalidModesAndPreservesLocal(t *testing.T) {
	missing := process.ResourceLimitSupport{}
	zero := process.ResourceLimits{}
	for _, policy := range []string{"unsafe", "allow", "disabled", "0", "ALLOW-DEGRADED"} {
		if err := validateCloudHostResourceLimits(runtimeprofile.ProfileCloudCore, false, zero, missing, policy); err == nil {
			t.Fatalf("unknown cloud resource policy accepted: %q", policy)
		}
	}
	for _, profile := range []runtimeprofile.Profile{runtimeprofile.ProfileLocal, runtimeprofile.ProfileDeviceAgent} {
		if err := validateCloudHostResourceLimits(profile, false, zero, missing, "strict"); err != nil {
			t.Fatalf("legacy %s deployment changed: %v", profile, err)
		}
	}
	if err := validateCloudHostResourceLimits(runtimeprofile.ProfileCloudCore, true, zero, missing, "strict"); err != nil {
		t.Fatalf("independently validated Windows task sandbox blocked: %v", err)
	}
}

func TestRound26CloudResourceStatusIsNotOnlyDeclaration(t *testing.T) {
	wanted := process.ResourceLimits{MaxMemoryBytes: 256 << 20, MaxCPUPercent: 25, MaxProcesses: taskProcessPIDBudget(runtime.GOOS)}
	missing := cloudHostMissingResourceLimits(wanted, process.ResourceLimitSupport{})
	if strings.Join(missing, ",") != "memory,cpu,pids" {
		t.Fatalf("unsupported OS limits silently declared enforceable: %v", missing)
	}
}
