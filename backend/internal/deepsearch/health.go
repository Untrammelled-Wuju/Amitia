package deepsearch

type HealthStatus string

const (
	HealthReady     HealthStatus = "ready"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnavailable HealthStatus = "unavailable"
)

type HealthReport struct {
	Enabled               bool        `json:"enabled"`
	Status                HealthStatus `json:"status"`
	TaskRuntimeReady      bool        `json:"taskRuntimeReady"`
	TaskDefinitionReady   bool        `json:"taskDefinitionReady"`
	GeneralSearchReady    bool        `json:"generalSearchReady"`
}

func NewHealthReport() *HealthReport {
	return &HealthReport{
		Status: HealthUnavailable,
	}
}
