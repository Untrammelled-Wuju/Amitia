// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantconfig

import (
	"gopkg.in/yaml.v3"
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
	cfg := qdrantDesktopYAMLConfig{
		Service: qdrantDesktopYAMLService{
			HTTPPort: doc.HTTPPort,
			GRPCPort: doc.GRPCPort,
		},
		Storage: qdrantDesktopYAMLStorage{
			StoragePath:   doc.StoragePath,
			SnapshotsPath: doc.SnapshotPath,
		},
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	return out, nil
}

func renderMobile(doc Document) ([]byte, error) {
	s := doc.ResourceProfile

	cfg := qdrantYAMLConfig{
		LogLevel:          s.LogLevel,
		TelemetryDisabled: s.TelemetryDisabled,
		Service: qdrantYAMLService{
			MaxRequestSizeMB:          s.MaxRequestSizeMB,
			MaxWorkers:                s.MaxWorkers,
			Host:                      s.BindHost,
			HTTPPort:                  doc.HTTPPort,
			GRPCPort:                  doc.GRPCPort,
			EnableCORS:                s.EnableCORS,
			EnableTLS:                 s.EnableTLS,
			EnableSnapshotURLRecovery: s.EnableSnapshotURLRecovery,
		},
		Storage: qdrantYAMLStorage{
			StoragePath:       doc.StoragePath,
			SnapshotsPath:     doc.SnapshotPath,
			OnDiskPayload:     s.OnDiskPayload,
			UpdateConcurrency:  s.UpdateConcurrency,
			WAL: qdrantYAMLWAL{
				WalCapacityMB:    s.WALCapacityMB,
				WalSegmentsAhead: s.WALSegmentsAhead,
			},
			Performance: qdrantYAMLPerformance{
				MaxSearchThreads:   s.MaxSearchThreads,
				OptimizerCPUBudget: s.OptimizerCPUBudget,
			},
			Optimizers: qdrantYAMLOptimizers{
				DefaultSegmentNumber:   s.DefaultSegmentNumber,
				IndexingThresholdKB:    s.IndexingThresholdKB,
				FlushIntervalSec:       s.FlushIntervalSeconds,
				MaxOptimizationThreads: s.MaxOptimizationThreads,
			},
			HNSWIndex: qdrantYAMLHNSW{
				MaxIndexingThreads: s.MaxIndexingThreads,
				OnDisk:             s.HNSWOnDisk,
			},
		},
		Cluster: qdrantYAMLCluster{
			Enabled: s.ClusterEnabled,
		},
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	return out, nil
}
