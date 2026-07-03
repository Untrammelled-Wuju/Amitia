package mindruntime

import (
	"context"
	"time"
)

func RunAllModuleChecks() []HealthCheckResult {
	targets := []HealthCheckTarget{
		HealthCheckAffect,
		HealthCheckBelief,
		HealthCheckSnapshot,
		HealthCheckDataLifecycle,
		HealthCheckPsyche,
		HealthCheckRelationship,
	}
	results := make([]HealthCheckResult, 0, len(targets))
	for _, t := range targets {
		result := RunHealthCheckWithContext(context.Background(), HealthCheckInput{Target: t}, 5*time.Second)
		results = append(results, result)
	}
	return results
}
