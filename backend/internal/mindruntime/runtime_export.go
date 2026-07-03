package mindruntime

import (
	"time"
)

type ModuleRuntimeStatus struct {
	Module          string           `json:"module"`
	Healthy         bool             `json:"healthy"`
	CheckedAt       time.Time        `json:"checkedAt"`
	ComponentChecks []ComponentCheck `json:"componentChecks,omitempty"`
}

type AggregatedRuntimeExport struct {
	ExportedAt      time.Time              `json:"exportedAt"`
	Version         string                 `json:"version"`
	Modules         []ModuleRuntimeStatus  `json:"modules"`
	MetricsSnapshot RuntimeMetricsSnapshot `json:"metricsSnapshot"`
	AllHealthy      bool                   `json:"allHealthy"`
	HealthyCount    int                    `json:"healthyCount"`
	TotalModules    int                    `json:"totalModules"`
}

func ExportRuntimeSnapshot() AggregatedRuntimeExport {
	results := RunAllModuleChecks()

	modules := make([]ModuleRuntimeStatus, 0, len(results))
	healthyCount := 0
	for _, r := range results {
		status := ModuleRuntimeStatus{
			Module:          string(r.Target),
			Healthy:         r.Healthy,
			CheckedAt:       r.CheckedAt,
			ComponentChecks: r.Checks,
		}
		modules = append(modules, status)
		if r.Healthy {
			healthyCount++
		}
	}

	metricsSnapshot := DefaultMetricsCollector.Snapshot()

	allHealthy := healthyCount == len(modules)

	return AggregatedRuntimeExport{
		ExportedAt:      time.Now().UTC(),
		Version:         "aggregated-runtime-export-v1",
		Modules:         modules,
		MetricsSnapshot: metricsSnapshot,
		AllHealthy:      allHealthy,
		HealthyCount:    healthyCount,
		TotalModules:    len(modules),
	}
}
