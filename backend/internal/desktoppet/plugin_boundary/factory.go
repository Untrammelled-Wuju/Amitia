package plugin_boundary

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/integration"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type BoundaryAdapters struct {
	Resource       ContributionAdapter
	Action         ContributionAdapter
	Runtime        ContributionAdapter
	FloatingWindow ContributionAdapter
}

func (a BoundaryAdapters) validate() error {
	if a.Resource == nil {
		return fmt.Errorf("plugin_boundary: Resource adapter is required")
	}
	if a.Action == nil {
		return fmt.Errorf("plugin_boundary: Action adapter is required")
	}
	if a.Runtime == nil {
		return fmt.Errorf("plugin_boundary: Runtime adapter is required")
	}
	if a.FloatingWindow == nil {
		return fmt.Errorf("plugin_boundary: FloatingWindow adapter is required")
	}
	return nil
}

func (a BoundaryAdapters) toSlice() []ContributionAdapter {
	return []ContributionAdapter{
		a.Resource,
		a.Action,
		a.Runtime,
		a.FloatingWindow,
	}
}

func NewBoundaryWith(source KernelContributionSource, adapters BoundaryAdapters, opts ...ReconcilerOption) (*DesktopPetPluginBoundary, error) {
	if err := adapters.validate(); err != nil {
		return nil, err
	}
	reconciler := NewReconciler(source, adapters.toSlice(), opts...)
	return NewDesktopPetPluginBoundary(reconciler), nil
}

func NewProductionBoundary(source KernelContributionSource, caps integration.DesktopPetPluginCapabilities) (*DesktopPetPluginBoundary, error) {
	if caps.InstallationCleanup == nil {
		return nil, fmt.Errorf("plugin_boundary: InstallationCleanup capability is required")
	}
	prod, err := newProductionAdapters(caps)
	if err != nil {
		return nil, err
	}
	cleanup := caps.InstallationCleanup
	reconciler := NewReconciler(source, prod.toAdapters(), WithUninstallCleanup(func(ctx context.Context, extID domain.ExtensionID, spaceID string) error {
		return cleanup.CleanupOwnedInstallations(ctx, integration.OwnedInstallationCleanupRequest{ExtensionID: string(extID), SpaceID: spaceID})
	}))
	return NewDesktopPetPluginBoundary(reconciler), nil
}

func NewBoundary(source KernelContributionSource) *DesktopPetPluginBoundary {
	return NewDesktopPetPluginBoundary(NewReconciler(source, DefaultAdapters().toSlice()))
}

func DefaultAdapters() BoundaryAdapters {
	return BoundaryAdapters{
		Resource:       &resourceContributionAdapter{},
		Action:         &actionContributionAdapter{},
		Runtime:        &runtimeCapabilityContributionAdapter{},
		FloatingWindow: &floatingWindowCapabilityContributionAdapter{},
	}
}
