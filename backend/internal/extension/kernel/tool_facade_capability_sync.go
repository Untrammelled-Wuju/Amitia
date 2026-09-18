package kernel

import (
	"context"
)

type CapabilityToolSyncResult struct {
	Registered int
	Skipped    int
	Removed    int
	Total      int
}

func (f *ToolFacade) SyncCapabilityTools(ctx context.Context, scope InvocationScope) (*CapabilityToolSyncResult, error) {
	return &CapabilityToolSyncResult{}, nil
}
