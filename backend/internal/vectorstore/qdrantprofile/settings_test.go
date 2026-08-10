// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

import "testing"

func TestCompactSettings_Validates(t *testing.T) {
	s := CompactSettings()
	if err := s.Validate(); err != nil {
		t.Errorf("CompactSettings.Validate: %v", err)
	}
}

func TestBalancedSettings_Validates(t *testing.T) {
	s := BalancedSettings()
	if err := s.Validate(); err != nil {
		t.Errorf("BalancedSettings.Validate: %v", err)
	}
}

func TestPerformanceSettings_Validates(t *testing.T) {
	s := PerformanceSettings()
	if err := s.Validate(); err != nil {
		t.Errorf("PerformanceSettings.Validate: %v", err)
	}
}

func TestSettings_CompactValues(t *testing.T) {
	s := CompactSettings()
	if s.MaxRequestSizeMB != 8 {
		t.Errorf("Compact MaxRequestSizeMB = %d", s.MaxRequestSizeMB)
	}
	if s.MaxWorkers != 1 {
		t.Errorf("Compact MaxWorkers = %d", s.MaxWorkers)
	}
	if s.MaxSearchThreads != 1 {
		t.Errorf("Compact MaxSearchThreads = %d", s.MaxSearchThreads)
	}
	if s.MaxOptimizationThreads != 1 {
		t.Errorf("Compact MaxOptimizationThreads = %d", s.MaxOptimizationThreads)
	}
	if s.MaxIndexingThreads != 2 {
		t.Errorf("Compact MaxIndexingThreads = %d", s.MaxIndexingThreads)
	}
	if !s.HNSWOnDisk {
		t.Errorf("Compact HNSWOnDisk should be true for memory-constrained profile")
	}
}

func TestSettings_BalancedValues(t *testing.T) {
	s := BalancedSettings()
	if s.MaxRequestSizeMB != 16 {
		t.Errorf("Balanced MaxRequestSizeMB = %d", s.MaxRequestSizeMB)
	}
	if s.MaxWorkers != 2 {
		t.Errorf("Balanced MaxWorkers = %d", s.MaxWorkers)
	}
	if s.MaxSearchThreads != 2 {
		t.Errorf("Balanced MaxSearchThreads = %d", s.MaxSearchThreads)
	}
	if s.WALCapacityMB != 16 {
		t.Errorf("Balanced WALCapacityMB = %d", s.WALCapacityMB)
	}
	if s.HNSWOnDisk {
		t.Errorf("Balanced HNSWOnDisk should be false for balanced profile")
	}
}

func TestSettings_PerformanceValues(t *testing.T) {
	s := PerformanceSettings()
	if s.MaxRequestSizeMB != 32 {
		t.Errorf("Performance MaxRequestSizeMB = %d", s.MaxRequestSizeMB)
	}
	if s.MaxIndexingThreads != 4 {
		t.Errorf("Performance MaxIndexingThreads = %d", s.MaxIndexingThreads)
	}
	if s.OptimizerCPUBudget != 2 {
		t.Errorf("Performance OptimizerCPUBudget = %d", s.OptimizerCPUBudget)
	}
}

func TestSettings_BindHostMustBeLoopback(t *testing.T) {
	s := BalancedSettings()
	s.BindHost = "0.0.0.0"
	if err := s.Validate(); err == nil {
		t.Error("expected error for non-loopback bind host")
	}
}

func TestSettings_CORSMustBeFalse(t *testing.T) {
	s := BalancedSettings()
	s.EnableCORS = true
	if err := s.Validate(); err == nil {
		t.Error("expected error for CORS enabled")
	}
}

func TestSettings_ClusterMustBeFalse(t *testing.T) {
	s := BalancedSettings()
	s.ClusterEnabled = true
	if err := s.Validate(); err == nil {
		t.Error("expected error for cluster enabled")
	}
}

func TestSettings_TelemetryMustBeDisabled(t *testing.T) {
	s := BalancedSettings()
	s.TelemetryDisabled = false
	if err := s.Validate(); err == nil {
		t.Error("expected error for telemetry enabled")
	}
}

func TestSettings_SnapshotURLRecoveryMustBeFalse(t *testing.T) {
	s := BalancedSettings()
	s.EnableSnapshotURLRecovery = true
	if err := s.Validate(); err == nil {
		t.Error("expected error for snapshot URL recovery enabled")
	}
}

func TestSettings_OnDiskPayloadMustBeTrue(t *testing.T) {
	s := BalancedSettings()
	s.OnDiskPayload = false
	if err := s.Validate(); err == nil {
		t.Error("expected error for on_disk_payload=false")
	}
}

func TestSettings_MaxWorkersZeroRejected(t *testing.T) {
	s := BalancedSettings()
	s.MaxWorkers = 0
	if err := s.Validate(); err == nil {
		t.Error("expected error for max_workers=0")
	}
}

func TestSettings_MaxSearchThreadsZeroRejected(t *testing.T) {
	s := BalancedSettings()
	s.MaxSearchThreads = 0
	if err := s.Validate(); err == nil {
		t.Error("expected error for max_search_threads=0")
	}
}

func TestSettings_OptimizerCPUBudgetZeroRejected(t *testing.T) {
	s := BalancedSettings()
	s.OptimizerCPUBudget = 0
	if err := s.Validate(); err == nil {
		t.Error("expected error for optimizer_cpu_budget=0")
	}
}

func TestSettings_MaxOptimizationThreadsZeroRejected(t *testing.T) {
	s := BalancedSettings()
	s.MaxOptimizationThreads = 0
	if err := s.Validate(); err == nil {
		t.Error("expected error for max_optimization_threads=0")
	}
}

func TestSettings_MaxIndexingThreadsZeroRejected(t *testing.T) {
	s := BalancedSettings()
	s.MaxIndexingThreads = 0
	if err := s.Validate(); err == nil {
		t.Error("expected error for max_indexing_threads=0")
	}
}

func TestSettings_MaxIndexingThreadsTooLarge(t *testing.T) {
	s := BalancedSettings()
	s.MaxIndexingThreads = 8
	if err := s.Validate(); err == nil {
		t.Error("expected error for max_indexing_threads > 4")
	}
}


func TestSettings_WALCapacityInvalid(t *testing.T) {
	s := BalancedSettings()
	s.WALCapacityMB = 64
	if err := s.Validate(); err == nil {
		t.Error("expected error for WAL capacity not in {8,16,32}")
	}
}

func TestSettings_FlushIntervalInvalid(t *testing.T) {
	s := BalancedSettings()
	s.FlushIntervalSeconds = 30
	if err := s.Validate(); err == nil {
		t.Error("expected error for flush interval > 15")
	}
}

func TestSettings_IndexingThresholdInvalid(t *testing.T) {
	s := BalancedSettings()
	s.IndexingThresholdKB = 5000
	if err := s.Validate(); err == nil {
		t.Error("expected error for indexing threshold < 10000")
	}
}

func TestSettings_NonDesktopProfileValidation(t *testing.T) {
	s := BalancedSettings()
	s.ID = ProfileDesktopDefault
	if err := s.Validate(); err == nil {
		t.Error("expected error: settings must use mobile profile ID")
	}
}
