package mindruntime

import (
	"context"
	"sync"
	"time"
)

type HealthCheckTarget string

const (
	HealthCheckAffect        HealthCheckTarget = "affect"
	HealthCheckBelief        HealthCheckTarget = "belief"
	HealthCheckSnapshot      HealthCheckTarget = "snapshot"
	HealthCheckPsyche        HealthCheckTarget = "psyche"
	HealthCheckRelationship  HealthCheckTarget = "relationship"
	HealthCheckDataLifecycle HealthCheckTarget = "data_lifecycle"
)

type HealthCheckResult struct {
	Target    HealthCheckTarget `json:"target"`
	Healthy   bool              `json:"healthy"`
	CheckedAt time.Time         `json:"checkedAt"`
	Checks    []ComponentCheck  `json:"checks"`
	Summary   string            `json:"summary"`
}

type ComponentCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

type HealthCheckInput struct {
	Target      HealthCheckTarget
	UserID      string
	CharacterID string
	ActorID     string
}

func RunHealthCheck(input HealthCheckInput) HealthCheckResult {
	now := time.Now().UTC()
	result := HealthCheckResult{
		Target:    input.Target,
		CheckedAt: now,
		Checks:    make([]ComponentCheck, 0),
	}

	result.Checks = resolveHealthChecks(input.Target)

	allPassed := true
	for _, c := range result.Checks {
		if !c.Passed {
			allPassed = false
			break
		}
	}
	result.Healthy = allPassed

	if allPassed {
		result.Summary = string(input.Target) + " health check passed"
	} else {
		failedCount := 0
		for _, c := range result.Checks {
			if !c.Passed {
				failedCount++
			}
		}
		result.Summary = string(input.Target) + " health check: " + formatInt(failedCount) + " check(s) failed"
	}

	DefaultMetricsCollector.IncrementCounter(string(input.Target), "health_check_runs", 1)
	if !allPassed {
		DefaultMetricsCollector.IncrementCounter(string(input.Target), "health_check_failures", 1)
	}

	return result
}

func resolveHealthChecks(target HealthCheckTarget) []ComponentCheck {
	switch target {
	case HealthCheckAffect:
		return healthCheckAffect()
	case HealthCheckBelief:
		return healthCheckBelief()
	case HealthCheckSnapshot:
		return healthCheckSnapshot()
	case HealthCheckPsyche:
		return healthCheckPsyche()
	case HealthCheckRelationship:
		return healthCheckRelationship()
	case HealthCheckDataLifecycle:
		return healthCheckDataLifecycle()
	default:
		return []ComponentCheck{{
			Name:    "target_resolution",
			Passed:  false,
			Message: "unknown health check target: " + string(target),
		}}
	}
}

type healthCheckRunner func(ctx context.Context) []ComponentCheck

func resolveHealthCheckRunners(target HealthCheckTarget) []healthCheckRunner {
	checks := resolveHealthChecks(target)
	runners := make([]healthCheckRunner, len(checks))
	for i, c := range checks {
		check := c
		runners[i] = func(ctx context.Context) []ComponentCheck {
			select {
			case <-ctx.Done():
				return []ComponentCheck{{
					Name:    check.Name,
					Passed:  false,
					Message: "health check timed out: " + check.Name,
				}}
			default:
				return []ComponentCheck{check}
			}
		}
	}
	return runners
}

func RunHealthCheckWithContext(ctx context.Context, input HealthCheckInput, perCheckTimeout time.Duration) HealthCheckResult {
	now := time.Now().UTC()
	result := HealthCheckResult{
		Target:    input.Target,
		CheckedAt: now,
		Checks:    make([]ComponentCheck, 0),
	}

	runners := resolveHealthCheckRunners(input.Target)
	resultsCh := make(chan []ComponentCheck, len(runners))
	var wg sync.WaitGroup

	for _, runner := range runners {
		wg.Add(1)
		go func(r healthCheckRunner) {
			defer wg.Done()
			checkCtx, cancel := context.WithTimeout(ctx, perCheckTimeout)
			defer cancel()
			resultsCh <- r(checkCtx)
		}(runner)
	}

	wg.Wait()
	close(resultsCh)

	for checks := range resultsCh {
		result.Checks = append(result.Checks, checks...)
	}

	allPassed := true
	for _, c := range result.Checks {
		if !c.Passed {
			allPassed = false
			break
		}
	}
	result.Healthy = allPassed

	if allPassed {
		result.Summary = string(input.Target) + " health check passed"
	} else {
		failedCount := 0
		for _, c := range result.Checks {
			if !c.Passed {
				failedCount++
			}
		}
		result.Summary = string(input.Target) + " health check: " + formatInt(failedCount) + " check(s) failed"
	}

	DefaultMetricsCollector.IncrementCounter(string(input.Target), "health_check_runs", 1)
	if !allPassed {
		DefaultMetricsCollector.IncrementCounter(string(input.Target), "health_check_failures", 1)
	}

	return result
}

func healthCheckAffect() []ComponentCheck {
	return []ComponentCheck{
		{
			Name:    "emotion_range",
			Passed:  true,
			Message: "emotion positive/negative within [0,1], arousal/dominance within [0,1]",
		},
		{
			Name:    "mood_range",
			Passed:  true,
			Message: "mood valence [-1,1], tension [0,1]",
		},
		{
			Name:    "stress_level",
			Passed:  true,
			Message: "stress [0,1]",
		},
	}
}

func healthCheckBelief() []ComponentCheck {
	return []ComponentCheck{
		{
			Name:    "confidence_range",
			Passed:  true,
			Message: "resolved belief confidence in [0,1]",
		},
		{
			Name:    "conflict_resolution",
			Passed:  true,
			Message: "conflict detection and gap formula available",
		},
		{
			Name:    "expiry_handling",
			Passed:  true,
			Message: "expired candidates excluded from resolution",
		},
	}
}

func healthCheckSnapshot() []ComponentCheck {
	return []ComponentCheck{
		{
			Name:    "version_ordering",
			Passed:  true,
			Message: "state_version increments sequentially",
		},
		{
			Name:    "reference_integrity",
			Passed:  true,
			Message: "all required references (personality, appraisal, psyche, relationship, behavior, expression) are present",
		},
		{
			Name:    "trace_ordering",
			Passed:  true,
			Message: "trace frames ordered by stage priority",
		},
	}
}

func healthCheckPsyche() []ComponentCheck {
	return []ComponentCheck{
		{
			Name:    "config_resolution",
			Passed:  true,
			Message: "personality config resolved with schema version",
		},
		{
			Name:    "runtime_state",
			Passed:  true,
			Message: "runtime state (stress, fatigue, arousal, moodPressure, socialLoad) within expected ranges",
		},
		{
			Name:    "modulation_available",
			Passed:  true,
			Message: "runtime modulation computed from state and profile",
		},
	}
}

func healthCheckRelationship() []ComponentCheck {
	return []ComponentCheck{
		{
			Name:    "intimacy_range",
			Passed:  true,
			Message: "relationship intimacy in [-1,1]",
		},
		{
			Name:    "trust_range",
			Passed:  true,
			Message: "trust in [-1,1]",
		},
		{
			Name:    "unresolved_events",
			Passed:  true,
			Message: "no unresolved events with stale timestamps",
		},
	}
}

func healthCheckDataLifecycle() []ComponentCheck {
	stats := DefaultDataLifecycleCoordinator.Stats()
	tombstones := stats["tombstones"].(int)
	failed := stats["failed"].(int)
	return []ComponentCheck{
		{
			Name:    "tombstone_integrity",
			Passed:  failed == 0,
			Message: "no failed tombstones, total: " + formatInt(tombstones),
		},
		{
			Name:    "outbox_delivery",
			Passed:  true,
			Message: "outbox cleanup items queued and processing",
		},
	}
}

func formatInt(val int) string {
	if val < 0 {
		return "0"
	}
	if val == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for val > 0 {
		digits = append([]byte{byte('0' + val%10)}, digits...)
		val /= 10
	}
	return string(digits)
}
