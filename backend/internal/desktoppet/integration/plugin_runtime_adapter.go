package integration

import (
	"context"
	"fmt"
)

type RuntimeProductionAdapter struct {
	caps PetRuntimeCapability
}

func NewRuntimeProductionAdapter(caps PetRuntimeCapability) *RuntimeProductionAdapter {
	return &RuntimeProductionAdapter{caps: caps}
}

func (a *RuntimeProductionAdapter) Attach(ctx context.Context, req PluginRuntimeAttachRequest) (PetRuntimeHandle, error) {
	if a.caps == nil {
		return "", fmt.Errorf("runtime production adapter: PetRuntimeCapability is nil")
	}
	return a.caps.AttachPluginRuntime(ctx, req)
}

func (a *RuntimeProductionAdapter) Detach(ctx context.Context, handle PetRuntimeHandle) error {
	if a.caps == nil {
		return fmt.Errorf("runtime production adapter: PetRuntimeCapability is nil")
	}
	return a.caps.DetachPluginRuntime(ctx, handle)
}
