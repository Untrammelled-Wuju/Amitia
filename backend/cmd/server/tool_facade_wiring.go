package main

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/mcpruntime"
)

type mcpDuplicateMetricAdapter struct {
	store *mcp.DuplicateStore
}

var _ kernel.MCPDuplicateMetricProvider = (*mcpDuplicateMetricAdapter)(nil)

func (a *mcpDuplicateMetricAdapter) CountUnresolved(ctx context.Context) (int64, error) {
	return a.store.CountUnresolved(ctx)
}

func (a *mcpDuplicateMetricAdapter) ListUnresolved(ctx context.Context) ([]kernel.MCPDuplicateDetail, error) {
	records, err := a.store.ListUnresolved(ctx)
	if err != nil {
		return nil, err
	}
	details := make([]kernel.MCPDuplicateDetail, len(records))
	for i, r := range records {
		details[i] = kernel.MCPDuplicateDetail{
			ToolID:     r.ToolID,
			ServerID:   r.ServerID,
			Owner:      r.Owner,
			Generation: r.Generation,
			DetectedAt: r.DetectedAt,
		}
	}
	return details, nil
}
