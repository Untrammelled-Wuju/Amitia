// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantconfig

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Renderer interface {
	Render(Document) ([]byte, error)
}

type yamlRenderer struct{}

func NewRenderer() Renderer {
	return &yamlRenderer{}
}

func (r *yamlRenderer) Render(doc Document) ([]byte, error) {
	if err := doc.Validate(); err != nil {
		return nil, newRenderFailed(err)
	}

	if doc.ResourceProfile == nil {
		return renderDesktop(doc)
	}
	return renderMobile(doc)
}

func renderDesktop(doc Document) ([]byte, error) {
	httpPortJSON, err := json.Marshal(doc.HTTPPort)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	grpcPortJSON, err := json.Marshal(doc.GRPCPort)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	storagePathJSON, err := json.Marshal(doc.StoragePath)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	snapshotPathJSON, err := json.Marshal(doc.SnapshotPath)
	if err != nil {
		return nil, newRenderFailed(err)
	}

	var sb strings.Builder
	sb.WriteString("service:\n")
	sb.WriteString(fmt.Sprintf("  http_port: %s\n", string(httpPortJSON)))
	sb.WriteString(fmt.Sprintf("  grpc_port: %s\n", string(grpcPortJSON)))
	sb.WriteString("storage:\n")
	sb.WriteString(fmt.Sprintf("  storage_path: %s\n", string(storagePathJSON)))
	sb.WriteString(fmt.Sprintf("  snapshots_path: %s\n", string(snapshotPathJSON)))

	return []byte(sb.String()), nil
}

func renderMobile(doc Document) ([]byte, error) {
	s := doc.ResourceProfile

	logLevelJSON, err := json.Marshal(s.LogLevel)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	bindHostJSON, err := json.Marshal(s.BindHost)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	storagePathJSON, err := json.Marshal(doc.StoragePath)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	snapshotPathJSON, err := json.Marshal(doc.SnapshotPath)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	hnswMemoryJSON, err := json.Marshal(s.HNSWMemory)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	httpPortJSON, err := json.Marshal(doc.HTTPPort)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	grpcPortJSON, err := json.Marshal(doc.GRPCPort)
	if err != nil {
		return nil, newRenderFailed(err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("log_level: %s\n", string(logLevelJSON)))
	sb.WriteString(fmt.Sprintf("telemetry_disabled: %t\n", s.TelemetryDisabled))

	sb.WriteString("service:\n")
	sb.WriteString(fmt.Sprintf("  max_request_size_mb: %d\n", s.MaxRequestSizeMB))
	sb.WriteString(fmt.Sprintf("  max_workers: %d\n", s.MaxWorkers))
	sb.WriteString(fmt.Sprintf("  host: %s\n", string(bindHostJSON)))
	sb.WriteString(fmt.Sprintf("  http_port: %s\n", string(httpPortJSON)))
	sb.WriteString(fmt.Sprintf("  grpc_port: %s\n", string(grpcPortJSON)))
	sb.WriteString(fmt.Sprintf("  enable_cors: %t\n", s.EnableCORS))
	sb.WriteString(fmt.Sprintf("  enable_tls: %t\n", s.EnableTLS))
	sb.WriteString(fmt.Sprintf("  enable_snapshot_url_recovery: %t\n", s.EnableSnapshotURLRecovery))

	sb.WriteString("storage:\n")
	sb.WriteString(fmt.Sprintf("  storage_path: %s\n", string(storagePathJSON)))
	sb.WriteString(fmt.Sprintf("  snapshots_path: %s\n", string(snapshotPathJSON)))
	sb.WriteString(fmt.Sprintf("  on_disk_payload: %t\n", s.OnDiskPayload))
	sb.WriteString(fmt.Sprintf("  update_concurrency: %d\n", s.UpdateConcurrency))

	sb.WriteString("  wal:\n")
	sb.WriteString(fmt.Sprintf("    wal_capacity_mb: %d\n", s.WALCapacityMB))
	sb.WriteString(fmt.Sprintf("    wal_segments_ahead: %d\n", s.WALSegmentsAhead))

	sb.WriteString("  performance:\n")
	sb.WriteString(fmt.Sprintf("    max_search_threads: %d\n", s.MaxSearchThreads))
	sb.WriteString(fmt.Sprintf("    optimizer_cpu_budget: %d\n", s.OptimizerCPUBudget))

	sb.WriteString("  optimizers:\n")
	sb.WriteString(fmt.Sprintf("    default_segment_number: %d\n", s.DefaultSegmentNumber))
	sb.WriteString(fmt.Sprintf("    indexing_threshold_kb: %d\n", s.IndexingThresholdKB))
	sb.WriteString(fmt.Sprintf("    flush_interval_sec: %d\n", s.FlushIntervalSeconds))
	sb.WriteString(fmt.Sprintf("    max_optimization_threads: %d\n", s.MaxOptimizationThreads))

	sb.WriteString("  hnsw_index:\n")
	sb.WriteString(fmt.Sprintf("    max_indexing_threads: %d\n", s.MaxIndexingThreads))
	sb.WriteString(fmt.Sprintf("    memory: %s\n", string(hnswMemoryJSON)))

	sb.WriteString("cluster:\n")
	sb.WriteString(fmt.Sprintf("  enabled: %t\n", s.ClusterEnabled))

	return []byte(sb.String()), nil
}
