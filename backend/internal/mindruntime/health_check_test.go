package mindruntime

import (
	"testing"
)

func TestRunHealthCheckAffect(t *testing.T) {
	result := RunHealthCheck(HealthCheckInput{
		Target: HealthCheckAffect,
	})
	if !result.Healthy {
		t.Fatal("affect health check should always be healthy")
	}
	if result.Target != HealthCheckAffect {
		t.Fatalf("expected target affect, got %s", result.Target)
	}
	if len(result.Checks) < 3 {
		t.Fatalf("expected at least 3 checks for affect, got %d", len(result.Checks))
	}
	if result.CheckedAt.IsZero() {
		t.Fatal("checkedAt should not be zero")
	}
}

func TestRunHealthCheckBelief(t *testing.T) {
	result := RunHealthCheck(HealthCheckInput{
		Target: HealthCheckBelief,
	})
	if !result.Healthy {
		t.Fatal("belief health check should always be healthy")
	}
	if len(result.Checks) < 3 {
		t.Fatalf("expected at least 3 checks for belief, got %d", len(result.Checks))
	}
}

func TestRunHealthCheckSnapshot(t *testing.T) {
	result := RunHealthCheck(HealthCheckInput{
		Target: HealthCheckSnapshot,
	})
	if !result.Healthy {
		t.Fatal("snapshot health check should always be healthy")
	}
	if len(result.Checks) < 3 {
		t.Fatalf("expected at least 3 checks for snapshot, got %d", len(result.Checks))
	}
}

func TestRunHealthCheckPsyche(t *testing.T) {
	result := RunHealthCheck(HealthCheckInput{
		Target: HealthCheckPsyche,
	})
	if !result.Healthy {
		t.Fatal("psyche health check should always be healthy")
	}
}

func TestRunHealthCheckRelationship(t *testing.T) {
	result := RunHealthCheck(HealthCheckInput{
		Target: HealthCheckRelationship,
	})
	if !result.Healthy {
		t.Fatal("relationship health check should always be healthy")
	}
}

func TestRunHealthCheckUnknownTarget(t *testing.T) {
	result := RunHealthCheck(HealthCheckInput{
		Target: "unknown_target",
	})
	if result.Healthy {
		t.Fatal("unknown target should not be healthy")
	}
	if len(result.Checks) < 1 {
		t.Fatal("should have at least 1 check for unknown target")
	}
	if result.Checks[0].Passed {
		t.Fatal("unknown target check should fail")
	}
}

func TestHealthCheckSummaryFormat(t *testing.T) {
	result := RunHealthCheck(HealthCheckInput{
		Target: HealthCheckAffect,
	})
	if result.Summary == "" {
		t.Fatal("summary should not be empty for a healthy check")
	}
}
