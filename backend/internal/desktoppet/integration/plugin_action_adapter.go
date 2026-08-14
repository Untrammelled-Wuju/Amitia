package integration

import (
	"context"
	"fmt"
)

type ActionProductionAdapter struct {
	caps     PetActionCapability
	resolver ActionTargetResolver
}

func NewActionProductionAdapter(caps PetActionCapability, resolver ActionTargetResolver) *ActionProductionAdapter {
	return &ActionProductionAdapter{caps: caps, resolver: resolver}
}

func (a *ActionProductionAdapter) Attach(ctx context.Context, req PluginActionAttachRequest) (PetActionHandle, error) {
	if a.caps == nil {
		return "", fmt.Errorf("action production adapter: PetActionCapability is nil")
	}
	return a.caps.AttachPluginAction(ctx, req)
}

func (a *ActionProductionAdapter) Detach(ctx context.Context, handle PetActionHandle) error {
	if a.caps == nil {
		return fmt.Errorf("action production adapter: PetActionCapability is nil")
	}
	return a.caps.DetachPluginAction(ctx, handle)
}

func (a *ActionProductionAdapter) ResolveTarget(ctx context.Context, extensionID string, contributionID string, revision int) (ExistingPetActionTarget, error) {
	if a.resolver == nil {
		return ExistingPetActionTarget{}, fmt.Errorf("action production adapter: ActionTargetResolver is nil")
	}
	return a.resolver.ResolveActionTarget(ctx, extensionID, contributionID, revision)
}
