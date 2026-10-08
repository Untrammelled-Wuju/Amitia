package task_runtime

import "github.com/u-ai/backend/internal/platform/process"

func taskProcessLimits(definition *TaskDefinition) process.ResourceLimits {
	limits := process.ResourceLimits{MaxMemoryBytes: 512 << 20, MaxCPUPercent: 50, MaxProcesses: 1}
	if definition != nil {
		if value := definition.ResourceLimits.MaxMemoryMB; value > 0 && value < 512 {
			limits.MaxMemoryBytes = uint64(value) << 20
		}
		if value := definition.ResourceLimits.MaxCPUPercent; value > 0 && value < 50 {
			limits.MaxCPUPercent = uint32(value)
		}
	}
	support := process.ResourceLimitsSupported()
	if !support.Memory {
		limits.MaxMemoryBytes = 0
	}
	if !support.CPU {
		limits.MaxCPUPercent = 0
	}
	if !support.Processes {
		limits.MaxProcesses = 0
	}
	return limits
}
