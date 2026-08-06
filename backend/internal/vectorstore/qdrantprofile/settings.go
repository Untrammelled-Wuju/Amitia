// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

import "fmt"

type Settings struct {
	ID                     ID
	LogLevel               string
	BindHost               string
	MaxRequestSizeMB       uint64
	MaxWorkers             uint64
	EnableCORS             bool
	EnableTLS              bool
	EnableSnapshotURLRecovery bool
	TelemetryDisabled      bool
	ClusterEnabled         bool
	OnDiskPayload          bool
	UpdateConcurrency      uint64
	WALCapacityMB          uint64
	WALSegmentsAhead       uint64
	MaxSearchThreads       uint64
	OptimizerCPUBudget     int64
	DefaultSegmentNumber   uint64
	IndexingThresholdKB    uint64
	FlushIntervalSeconds   uint64
	MaxOptimizationThreads uint64
	MaxIndexingThreads     uint64
	HNSWMemory             string
}

func CompactSettings() Settings {
	return Settings{
		ID:                       ProfileMobileCompact,
		LogLevel:                 "INFO",
		BindHost:                 "127.0.0.1",
		MaxRequestSizeMB:         8,
		MaxWorkers:               1,
		EnableCORS:               false,
		EnableTLS:                false,
		EnableSnapshotURLRecovery: false,
		TelemetryDisabled:        true,
		ClusterEnabled:           false,
		OnDiskPayload:            true,
		UpdateConcurrency:        1,
		WALCapacityMB:            8,
		WALSegmentsAhead:         0,
		MaxSearchThreads:         1,
		OptimizerCPUBudget:       1,
		DefaultSegmentNumber:     1,
		IndexingThresholdKB:      50000,
		FlushIntervalSeconds:     15,
		MaxOptimizationThreads:   1,
		MaxIndexingThreads:       2,
		HNSWMemory:               "cold",
	}
}

func BalancedSettings() Settings {
	return Settings{
		ID:                       ProfileMobileBalanced,
		LogLevel:                 "INFO",
		BindHost:                 "127.0.0.1",
		MaxRequestSizeMB:         16,
		MaxWorkers:               2,
		EnableCORS:               false,
		EnableTLS:                false,
		EnableSnapshotURLRecovery: false,
		TelemetryDisabled:        true,
		ClusterEnabled:           false,
		OnDiskPayload:            true,
		UpdateConcurrency:        1,
		WALCapacityMB:            16,
		WALSegmentsAhead:         0,
		MaxSearchThreads:         2,
		OptimizerCPUBudget:       1,
		DefaultSegmentNumber:     1,
		IndexingThresholdKB:      20000,
		FlushIntervalSeconds:     10,
		MaxOptimizationThreads:   1,
		MaxIndexingThreads:       2,
		HNSWMemory:               "cached",
	}
}

func PerformanceSettings() Settings {
	return Settings{
		ID:                       ProfileMobilePerformance,
		LogLevel:                 "INFO",
		BindHost:                 "127.0.0.1",
		MaxRequestSizeMB:         32,
		MaxWorkers:               2,
		EnableCORS:               false,
		EnableTLS:                false,
		EnableSnapshotURLRecovery: false,
		TelemetryDisabled:        true,
		ClusterEnabled:           false,
		OnDiskPayload:            true,
		UpdateConcurrency:        2,
		WALCapacityMB:            32,
		WALSegmentsAhead:         0,
		MaxSearchThreads:         2,
		OptimizerCPUBudget:       2,
		DefaultSegmentNumber:     1,
		IndexingThresholdKB:      10000,
		FlushIntervalSeconds:     5,
		MaxOptimizationThreads:   1,
		MaxIndexingThreads:       4,
		HNSWMemory:               "cached",
	}
}

func (s Settings) Validate() error {
	if !s.ID.isMobileProfile() {
		return fmt.Errorf("%w: settings ID must be mobile, got %q", ErrInvalidProfileSettings, s.ID)
	}
	if s.LogLevel != "INFO" {
		return fmt.Errorf("%w: logLevel must be INFO, got %q", ErrInvalidProfileSettings, s.LogLevel)
	}
	if s.BindHost != "127.0.0.1" {
		return fmt.Errorf("%w: bindHost must be 127.0.0.1, got %q", ErrInvalidProfileSettings, s.BindHost)
	}
	if s.MaxRequestSizeMB < 1 || s.MaxRequestSizeMB > 64 {
		return fmt.Errorf("%w: maxRequestSizeMB out of range: %d", ErrInvalidProfileSettings, s.MaxRequestSizeMB)
	}
	if s.MaxWorkers < 1 || s.MaxWorkers > 4 {
		return fmt.Errorf("%w: maxWorkers out of range: %d", ErrInvalidProfileSettings, s.MaxWorkers)
	}
	if s.EnableCORS {
		return fmt.Errorf("%w: enableCORS must be false", ErrInvalidProfileSettings)
	}
	if s.EnableTLS {
		return fmt.Errorf("%w: enableTLS must be false", ErrInvalidProfileSettings)
	}
	if s.EnableSnapshotURLRecovery {
		return fmt.Errorf("%w: enableSnapshotURLRecovery must be false", ErrInvalidProfileSettings)
	}
	if !s.TelemetryDisabled {
		return fmt.Errorf("%w: telemetryDisabled must be true", ErrInvalidProfileSettings)
	}
	if s.ClusterEnabled {
		return fmt.Errorf("%w: clusterEnabled must be false", ErrInvalidProfileSettings)
	}
	if !s.OnDiskPayload {
		return fmt.Errorf("%w: onDiskPayload must be true", ErrInvalidProfileSettings)
	}
	if s.UpdateConcurrency < 1 || s.UpdateConcurrency > 2 {
		return fmt.Errorf("%w: updateConcurrency out of range: %d", ErrInvalidProfileSettings, s.UpdateConcurrency)
	}
	if s.WALCapacityMB != 8 && s.WALCapacityMB != 16 && s.WALCapacityMB != 32 {
		return fmt.Errorf("%w: walCapacityMB must be 8, 16, or 32, got %d", ErrInvalidProfileSettings, s.WALCapacityMB)
	}
	if s.WALSegmentsAhead != 0 {
		return fmt.Errorf("%w: walSegmentsAhead must be 0, got %d", ErrInvalidProfileSettings, s.WALSegmentsAhead)
	}
	if s.MaxSearchThreads < 1 || s.MaxSearchThreads > 2 {
		return fmt.Errorf("%w: maxSearchThreads out of range: %d", ErrInvalidProfileSettings, s.MaxSearchThreads)
	}
	if s.OptimizerCPUBudget < 1 || s.OptimizerCPUBudget > 2 {
		return fmt.Errorf("%w: optimizerCPUBudget out of range: %d", ErrInvalidProfileSettings, s.OptimizerCPUBudget)
	}
	if s.DefaultSegmentNumber != 1 {
		return fmt.Errorf("%w: defaultSegmentNumber must be 1, got %d", ErrInvalidProfileSettings, s.DefaultSegmentNumber)
	}
	if s.IndexingThresholdKB < 10000 || s.IndexingThresholdKB > 50000 {
		return fmt.Errorf("%w: indexingThresholdKB out of range: %d", ErrInvalidProfileSettings, s.IndexingThresholdKB)
	}
	if s.FlushIntervalSeconds < 5 || s.FlushIntervalSeconds > 15 {
		return fmt.Errorf("%w: flushIntervalSeconds out of range: %d", ErrInvalidProfileSettings, s.FlushIntervalSeconds)
	}
	if s.MaxOptimizationThreads != 1 {
		return fmt.Errorf("%w: maxOptimizationThreads must be 1, got %d", ErrInvalidProfileSettings, s.MaxOptimizationThreads)
	}
	if s.MaxIndexingThreads < 2 || s.MaxIndexingThreads > 4 {
		return fmt.Errorf("%w: maxIndexingThreads out of range: %d", ErrInvalidProfileSettings, s.MaxIndexingThreads)
	}
	if s.HNSWMemory != "cold" && s.HNSWMemory != "cached" {
		return fmt.Errorf("%w: hnswMemory must be cold or cached, got %q", ErrInvalidProfileSettings, s.HNSWMemory)
	}
	return nil
}

func (id ID) isMobileProfile() bool {
	return id == ProfileMobileCompact || id == ProfileMobileBalanced || id == ProfileMobilePerformance
}
