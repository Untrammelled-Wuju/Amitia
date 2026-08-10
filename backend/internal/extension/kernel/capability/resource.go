package capability

import "context"

type ResourceDimension string

const (
	ResourceMemoryBytes ResourceDimension = "memory_bytes"
	ResourceCPUPercent  ResourceDimension = "cpu_percent"
	ResourceProcesses   ResourceDimension = "processes"
	ResourceOpenFiles   ResourceDimension = "open_files"
	ResourceConnections ResourceDimension = "connections"
	ResourceDiskBytes   ResourceDimension = "disk_bytes"
)

type ResourceEnforcementMode string

const (
	ResourceEnforcementUnsupported  ResourceEnforcementMode = "unsupported"
	ResourceEnforcementDeclaredOnly ResourceEnforcementMode = "declared_only"
	ResourceEnforcementMeasured     ResourceEnforcementMode = "measured"
	ResourceEnforcementEnforced     ResourceEnforcementMode = "enforced"
)

type ResourceLimitSource string

const (
	ResourceLimitSourceToolPolicy        ResourceLimitSource = "tool_policy"
	ResourceLimitSourceRuntime           ResourceLimitSource = "runtime"
	ResourceLimitSourceRuntimeSupervisor ResourceLimitSource = "runtime_supervisor"
	ResourceLimitSourceProvider          ResourceLimitSource = "provider"
	ResourceLimitSourcePlatform          ResourceLimitSource = "platform"
)

type EffectiveResourceLimit struct {
	Dimension ResourceDimension       `json:"dimension"`
	Limit     int64                   `json:"limit"`
	Mode      ResourceEnforcementMode `json:"mode"`
	Source    ResourceLimitSource     `json:"source"`
}

type EffectiveResourceLimits struct {
	Limits []EffectiveResourceLimit `json:"limits"`
}

func (e EffectiveResourceLimits) ForDimension(dim ResourceDimension) (EffectiveResourceLimit, bool) {
	for _, l := range e.Limits {
		if l.Dimension == dim {
			return l, true
		}
	}
	return EffectiveResourceLimit{}, false
}

func (e EffectiveResourceLimits) StrictestLimit(dim ResourceDimension) (EffectiveResourceLimit, bool) {
	var best *EffectiveResourceLimit
	for i := range e.Limits {
		l := &e.Limits[i]
		if l.Dimension != dim {
			continue
		}
		if l.Mode == ResourceEnforcementUnsupported {
			continue
		}
		if l.Limit <= 0 {
			continue
		}
		if best == nil {
			best = l
			continue
		}
		if l.Limit < best.Limit {
			best = l
			continue
		}
		if l.Limit == best.Limit && enforcementRank(l.Mode) > enforcementRank(best.Mode) {
			best = l
		}
	}
	if best == nil {
		return EffectiveResourceLimit{}, false
	}
	return *best, true
}

func enforcementRank(mode ResourceEnforcementMode) int {
	switch mode {
	case ResourceEnforcementEnforced:
		return 4
	case ResourceEnforcementMeasured:
		return 3
	case ResourceEnforcementDeclaredOnly:
		return 2
	default:
		return 1
	}
}

type ResourceLimitRequirement string

const (
	ResourceLimitBestEffort      ResourceLimitRequirement = "best_effort"
	ResourceLimitRequireEnforced ResourceLimitRequirement = "require_enforced"
)

type ResourceUsage struct {
	PeakMemoryBytes    int64               `json:"peakMemoryBytes"`
	CPUPercentPeak     float64             `json:"cpuPercentPeak"`
	ProcessCountPeak   int                 `json:"processCountPeak"`
	OpenFilesPeak      int                 `json:"openFilesPeak"`
	ConnectionCountPeak int                `json:"connectionCountPeak"`
	DiskBytesPeak      int64               `json:"diskBytesPeak"`
	MeasuredDimensions []ResourceDimension `json:"measuredDimensions"`
}

type RuntimeResourceCapabilities struct {
	MemoryMode  ResourceEnforcementMode `json:"memoryMode"`
	CPUMode     ResourceEnforcementMode `json:"cpuMode"`
	MaxMemory   int64                   `json:"maxMemory"`
	MaxCPU      int                     `json:"maxCPU"`
	RuntimeID   string                  `json:"runtimeID"`
}

type RuntimeResourceCapabilityResolver interface {
	ResolveRuntime(
		ctx context.Context,
		binding RuntimeBinding,
	) (RuntimeResourceCapabilities, error)
}

type PlatformIsolationProvider interface {
	IsolationReport() PlatformIsolationReport
}

type PlatformIsolationReport struct {
	MemoryLimit bool `json:"memoryLimit"`
	CPULimit    bool `json:"cpuLimit"`
}

type ResourceAwareRuntimeAdapter interface {
	RuntimeAdapter

	ResourceCapabilities(
		ctx context.Context,
		binding RuntimeBinding,
	) (RuntimeResourceCapabilities, error)
}

type RuntimeExecutionResourcePolicy struct {
	Limits  EffectiveResourceLimits `json:"limits"`
	Usage  *ResourceUsage           `json:"usage,omitempty"`
	Exceeded *ResourceDimension      `json:"exceeded,omitempty"`
}
