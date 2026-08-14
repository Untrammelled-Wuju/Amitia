package capability

type HealthStatus string

const (
	HealthUnknown   HealthStatus = "unknown"
	HealthReady     HealthStatus = "ready"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthShutdown  HealthStatus = "shutdown"
)

func (s HealthStatus) IsValid() bool {
	switch s {
	case HealthUnknown, HealthReady, HealthDegraded, HealthUnhealthy, HealthShutdown:
		return true
	}
	return false
}

type ToolState struct {
	Installed         bool         `json:"installed"`
	ModuleEnabled     bool         `json:"moduleEnabled"`
	CapabilityEnabled bool         `json:"capabilityEnabled"`
	ScopeAllowed      bool         `json:"scopeAllowed"`
	PermissionGranted bool         `json:"permissionGranted"`
	RuntimeReady      bool         `json:"runtimeReady"`
	DependencyReady   bool         `json:"dependencyReady"`
	Health            HealthStatus `json:"health"`
}

func (s ToolState) VisibleToModel() bool {
	return s.Installed && s.ModuleEnabled && s.CapabilityEnabled && s.ScopeAllowed
}

func (s ToolState) Executable() bool {
	return s.VisibleToModel() && s.PermissionGranted && s.RuntimeReady && s.DependencyReady && s.Health == HealthReady
}
