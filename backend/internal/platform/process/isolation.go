package process

// ResourceLimits describes process-tree limits that the platform process
// backend may enforce. Zero means no requested limit.
type ResourceLimits struct {
	MaxMemoryBytes uint64
	MaxCPUPercent  uint32
	MaxProcesses   uint32
}

// ResourceLimitSupport reports only limits that are actually enforced by the
// current platform backend. It is used by higher layers to avoid advertising
// declarative limits as hard guarantees.
type ResourceLimitSupport struct {
	Memory    bool
	CPU       bool
	Processes bool
}

type PlatformIsolationReport struct {
	Platform             string   `json:"platform"`
	ProcessTreeIsolation bool     `json:"process_tree_isolation"`
	MemoryLimit          bool     `json:"memory_limit"`
	CPULimit             bool     `json:"cpu_limit"`
	FilesystemIsolation  bool     `json:"filesystem_isolation"`
	NetworkIsolation     bool     `json:"network_isolation"`
	UserNamespace        bool     `json:"user_namespace"`
	Seccomp              bool     `json:"seccomp"`
	AppContainer         bool     `json:"app_container"`
	SandboxProfile       bool     `json:"sandbox_profile"`
	Limitations          []string `json:"limitations"`
}

func (r PlatformIsolationReport) IsFullyIsolated() bool {
	return r.ProcessTreeIsolation &&
		r.MemoryLimit &&
		r.CPULimit &&
		r.FilesystemIsolation &&
		r.NetworkIsolation
}

func (r PlatformIsolationReport) HasLimitations() bool {
	return len(r.Limitations) > 0
}
