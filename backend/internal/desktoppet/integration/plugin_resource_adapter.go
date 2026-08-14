package integration

import (
	"context"
	"fmt"
)

type ResourceProductionAdapter struct {
	caps PetResourceCapability
}

func NewResourceProductionAdapter(caps PetResourceCapability) *ResourceProductionAdapter {
	return &ResourceProductionAdapter{caps: caps}
}

func (a *ResourceProductionAdapter) Attach(ctx context.Context, req PluginResourceAttachRequest) (PetResourceHandle, error) {
	if a.caps == nil {
		return "", fmt.Errorf("resource production adapter: PetResourceCapability is nil")
	}
	return a.caps.AttachPluginResource(ctx, req)
}

func (a *ResourceProductionAdapter) Detach(ctx context.Context, handle PetResourceHandle) error {
	if a.caps == nil {
		return fmt.Errorf("resource production adapter: PetResourceCapability is nil")
	}
	return a.caps.DetachPluginResource(ctx, handle)
}
