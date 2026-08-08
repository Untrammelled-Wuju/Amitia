package domain

import "testing"

func TestHealthStateHelpers(t *testing.T) {
	healthy := HealthState{Status: HealthHealthy}
	if !healthy.IsHealthy() {
		t.Error("expected IsHealthy() to return true")
	}
	if healthy.IsDegraded() || healthy.IsUnhealthy() {
		t.Error("unhealthy/degraded should be false")
	}

	degraded := HealthState{Status: HealthDegraded}
	if !degraded.IsDegraded() {
		t.Error("expected IsDegraded() to return true")
	}
	if degraded.IsHealthy() || degraded.IsUnhealthy() {
		t.Error("healthy/unhealthy should be false")
	}

	unhealthy := HealthState{Status: HealthUnhealthy}
	if !unhealthy.IsUnhealthy() {
		t.Error("expected IsUnhealthy() to return true")
	}
	if unhealthy.IsHealthy() || unhealthy.IsDegraded() {
		t.Error("healthy/degraded should be false")
	}

	unknown := HealthState{Status: HealthUnknown}
	if unknown.IsHealthy() || unknown.IsDegraded() || unknown.IsUnhealthy() {
		t.Error("unknown state should return false for all checks")
	}
}
