package mindruntime

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
		result := RunHealthCheck(HealthCheckInput{Target: t})
		results = append(results, result)
	}
	return results
}