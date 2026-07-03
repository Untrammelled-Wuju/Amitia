package mindruntime

import (
	"testing"
)

func TestExportRuntimeSnapshotReturnsAllModules(t *testing.T) {
	export := ExportRuntimeSnapshot()

	if export.Version != "aggregated-runtime-export-v1" {
		t.Fatalf("unexpected version: %s", export.Version)
	}
	if export.TotalModules < 5 {
		t.Fatalf("expected at least 5 modules, got %d", export.TotalModules)
	}
	if len(export.Modules) != export.TotalModules {
		t.Fatalf("module count mismatch: %d vs %d", len(export.Modules), export.TotalModules)
	}
	if export.ExportedAt.IsZero() {
		t.Fatal("exportedAt should not be zero")
	}
	if export.MetricsSnapshot.Version == "" {
		t.Fatal("metrics snapshot version should not be empty")
	}
}

func TestExportRuntimeSnapshotAggregatesHealth(t *testing.T) {
	export := ExportRuntimeSnapshot()

	healthyCount := 0
	for _, mod := range export.Modules {
		if mod.Module == "" {
			t.Fatal("module name should not be empty")
		}
		if mod.CheckedAt.IsZero() {
			t.Fatalf("checkedAt for module %s should not be zero", mod.Module)
		}
		if mod.Healthy {
			healthyCount++
		}
	}
	if export.HealthyCount != healthyCount {
		t.Fatalf("healthyCount mismatch: reported %d, actual %d", export.HealthyCount, healthyCount)
	}
	allHealthy := healthyCount == export.TotalModules
	if export.AllHealthy != allHealthy {
		t.Fatalf("allHealthy mismatch: reported %v, actual %v", export.AllHealthy, allHealthy)
	}
}

func TestExportRuntimeSnapshotIncludesMetricsSnapshot(t *testing.T) {
	export := ExportRuntimeSnapshot()

	if export.MetricsSnapshot.CollectedAt == "" {
		t.Fatal("metrics snapshot collectedAt should not be empty")
	}
	if export.MetricsSnapshot.Version != "runtime-metrics-v1" {
		t.Fatalf("unexpected metrics version: %s", export.MetricsSnapshot.Version)
	}
}
