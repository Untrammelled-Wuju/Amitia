package kernel

import (
	"context"
)

type LegacyToolSyncResult struct {
	Registered int
	Skipped    int
	Removed    int
	Total      int
}

func (f *ToolFacade) SyncLegacyTools(ctx context.Context, scope LegacyScope) (*LegacyToolSyncResult, error) {
	return &LegacyToolSyncResult{}, nil
}
