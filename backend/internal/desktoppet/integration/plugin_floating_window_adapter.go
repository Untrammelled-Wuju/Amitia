package integration

import (
	"context"
	"fmt"
)

type FloatingWindowProductionAdapter struct {
	caps PetFloatingWindowCapability
}

func NewFloatingWindowProductionAdapter(caps PetFloatingWindowCapability) *FloatingWindowProductionAdapter {
	return &FloatingWindowProductionAdapter{caps: caps}
}

func (a *FloatingWindowProductionAdapter) Attach(ctx context.Context, req PluginFloatingWindowAttachRequest) (PetFloatingWindowHandle, error) {
	if a.caps == nil {
		return "", fmt.Errorf("floating window production adapter: PetFloatingWindowCapability is nil")
	}
	return a.caps.AttachPluginFloatingWindow(ctx, req)
}

func (a *FloatingWindowProductionAdapter) Detach(ctx context.Context, handle PetFloatingWindowHandle) error {
	if a.caps == nil {
		return fmt.Errorf("floating window production adapter: PetFloatingWindowCapability is nil")
	}
	return a.caps.DetachPluginFloatingWindow(ctx, handle)
}
