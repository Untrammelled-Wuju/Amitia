package process

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
