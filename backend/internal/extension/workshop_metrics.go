package extension

import "sync"

const (
	WorkshopMetricSessionCreated      = "workshop_session_created_total"
	WorkshopMetricGeneration          = "workshop_generation_total"
	WorkshopMetricGenerationFailure   = "workshop_generation_failure_total"
	WorkshopMetricValidationFailure   = "workshop_validation_failure_total"
	WorkshopMetricTest                = "workshop_test_total"
	WorkshopMetricTestFailure         = "workshop_test_failure_total"
	WorkshopMetricInstall             = "workshop_install_total"
	WorkshopMetricInstallFailure      = "workshop_install_failure_total"
	WorkshopMetricRollback            = "workshop_rollback_total"
	WorkshopMetricWorkflowStep        = "workshop_workflow_step_total"
	WorkshopMetricWorkflowStepFailure = "workshop_workflow_step_failure_total"
	WorkshopMetricSandboxLimit        = "workshop_sandbox_limit_total"
	WorkshopMetricSecretDetected      = "workshop_secret_detected_total"
	WorkshopMetricNetworkDenied       = "workshop_network_denied_total"
)

var workshopMetricNames = []string{
	WorkshopMetricSessionCreated,
	WorkshopMetricGeneration,
	WorkshopMetricGenerationFailure,
	WorkshopMetricValidationFailure,
	WorkshopMetricTest,
	WorkshopMetricTestFailure,
	WorkshopMetricInstall,
	WorkshopMetricInstallFailure,
	WorkshopMetricRollback,
	WorkshopMetricWorkflowStep,
	WorkshopMetricWorkflowStepFailure,
	WorkshopMetricSandboxLimit,
	WorkshopMetricSecretDetected,
	WorkshopMetricNetworkDenied,
}

type workshopMetrics struct {
	mu       sync.RWMutex
	counters map[string]uint64
}

var defaultWorkshopMetrics = &workshopMetrics{counters: map[string]uint64{}}

func incrementWorkshopMetric(name string) {
	defaultWorkshopMetrics.mu.Lock()
	defaultWorkshopMetrics.counters[name]++
	defaultWorkshopMetrics.mu.Unlock()
}

func recordWorkshopErrorMetric(err error) {
	if err == nil {
		return
	}
	switch asExtensionError(err).Code {
	case ErrWorkshopSandboxLimit:
		incrementWorkshopMetric(WorkshopMetricSandboxLimit)
	case ErrWorkshopSecretDetected:
		incrementWorkshopMetric(WorkshopMetricSecretDetected)
	case ErrWorkshopNetworkDenied:
		incrementWorkshopMetric(WorkshopMetricNetworkDenied)
	}
}

func WorkshopMetricsSnapshot() map[string]uint64 {
	defaultWorkshopMetrics.mu.RLock()
	defer defaultWorkshopMetrics.mu.RUnlock()
	result := make(map[string]uint64, len(workshopMetricNames))
	for _, name := range workshopMetricNames {
		result[name] = defaultWorkshopMetrics.counters[name]
	}
	return result
}

func resetWorkshopMetrics() {
	defaultWorkshopMetrics.mu.Lock()
	defaultWorkshopMetrics.counters = map[string]uint64{}
	defaultWorkshopMetrics.mu.Unlock()
}
