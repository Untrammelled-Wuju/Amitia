// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantconfig

type qdrantYAMLConfig struct {
	LogLevel       string                 `yaml:"log_level,omitempty"`
	TelemetryDisabled bool                `yaml:"telemetry_disabled,omitempty"`
	Service        qdrantYAMLService     `yaml:"service"`
	Storage        qdrantYAMLStorage     `yaml:"storage"`
	Cluster        qdrantYAMLCluster     `yaml:"cluster"`
}

type qdrantYAMLService struct {
	MaxRequestSizeMB          uint64 `yaml:"max_request_size_mb,omitempty"`
	MaxWorkers                uint64 `yaml:"max_workers,omitempty"`
	Host                      string `yaml:"host"`
	HTTPPort                  int    `yaml:"http_port"`
	GRPCPort                  int    `yaml:"grpc_port"`
	EnableCORS                bool   `yaml:"enable_cors"`
	EnableTLS                 bool   `yaml:"enable_tls"`
	EnableSnapshotURLRecovery bool   `yaml:"enable_snapshot_url_recovery"`
}

type qdrantYAMLStorage struct {
	StoragePath   string                   `yaml:"storage_path"`
	SnapshotsPath string                   `yaml:"snapshots_path"`
	OnDiskPayload bool                     `yaml:"on_disk_payload"`
	UpdateConcurrency uint64               `yaml:"update_concurrency,omitempty"`
	WAL           qdrantYAMLWAL            `yaml:"wal"`
	Performance   qdrantYAMLPerformance    `yaml:"performance"`
	Optimizers    qdrantYAMLOptimizers     `yaml:"optimizers"`
	HNSWIndex     qdrantYAMLHNSW           `yaml:"hnsw_index"`
}

type qdrantYAMLWAL struct {
	WalCapacityMB    uint64 `yaml:"wal_capacity_mb"`
	WalSegmentsAhead uint64 `yaml:"wal_segments_ahead"`
}

type qdrantYAMLPerformance struct {
	MaxSearchThreads   uint64 `yaml:"max_search_threads"`
	OptimizerCPUBudget int64  `yaml:"optimizer_cpu_budget"`
}

type qdrantYAMLOptimizers struct {
	DefaultSegmentNumber uint64 `yaml:"default_segment_number"`
	IndexingThresholdKB  uint64 `yaml:"indexing_threshold_kb"`
	FlushIntervalSec     uint64 `yaml:"flush_interval_sec"`
	MaxOptimizationThreads uint64 `yaml:"max_optimization_threads"`
}

type qdrantYAMLHNSW struct {
	MaxIndexingThreads uint64 `yaml:"max_indexing_threads"`
	OnDisk             bool   `yaml:"on_disk,omitempty"`
}

type qdrantYAMLCluster struct {
	Enabled bool `yaml:"enabled"`
}

type qdrantDesktopYAMLConfig struct {
	Service qdrantDesktopYAMLService `yaml:"service"`
	Storage qdrantDesktopYAMLStorage `yaml:"storage"`
}

type qdrantDesktopYAMLService struct {
	HTTPPort int `yaml:"http_port"`
	GRPCPort int `yaml:"grpc_port"`
}

type qdrantDesktopYAMLStorage struct {
	StoragePath   string `yaml:"storage_path"`
	SnapshotsPath string `yaml:"snapshots_path"`
}
