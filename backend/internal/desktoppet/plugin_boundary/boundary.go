package plugin_boundary

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type DesktopPetPluginBoundary struct {
	reconciler ContributionReconciler
}

func NewDesktopPetPluginBoundary(reconciler ContributionReconciler) *DesktopPetPluginBoundary {
	return &DesktopPetPluginBoundary{reconciler: reconciler}
}

func (b *DesktopPetPluginBoundary) HandleExtensionInstalled(ctx context.Context, extID domain.ExtensionID, version, operationID string) error {
	return b.reconciler.HandleEvent(ctx, LifecycleEvent{
		Phase:       PhaseInstalled,
		ExtensionID: extID,
		Version:     version,
		OperationID: operationID,
		Timestamp:   time.Now().UTC(),
	})
}

func (b *DesktopPetPluginBoundary) HandleExtensionEnabled(ctx context.Context, extID domain.ExtensionID, version, operationID string) error {
	return b.reconciler.HandleEvent(ctx, LifecycleEvent{
		Phase:       PhaseEnabled,
		ExtensionID: extID,
		Version:     version,
		OperationID: operationID,
		Timestamp:   time.Now().UTC(),
	})
}

func (b *DesktopPetPluginBoundary) HandleExtensionDisabled(ctx context.Context, extID domain.ExtensionID, version, operationID, reason string) error {
	return b.reconciler.HandleEvent(ctx, LifecycleEvent{
		Phase:       PhaseDisabled,
		ExtensionID: extID,
		Version:     version,
		OperationID: operationID,
		Timestamp:   time.Now().UTC(),
	})
}

func (b *DesktopPetPluginBoundary) HandleExtensionUpdated(ctx context.Context, extID domain.ExtensionID, oldVersion, newVersion, operationID string) error {
	return b.reconciler.HandleEvent(ctx, LifecycleEvent{
		Phase:       PhaseUpdated,
		ExtensionID: extID,
		Version:     newVersion,
		OldVersion:  oldVersion,
		OperationID: operationID,
		Timestamp:   time.Now().UTC(),
	})
}

func (b *DesktopPetPluginBoundary) HandleExtensionUninstalled(ctx context.Context, extID domain.ExtensionID, version, operationID, reason string) error {
	return b.reconciler.HandleEvent(ctx, LifecycleEvent{
		Phase:       PhaseUninstalled,
		ExtensionID: extID,
		Version:     version,
		OperationID: operationID,
		Timestamp:   time.Now().UTC(),
	})
}

func (b *DesktopPetPluginBoundary) ReconcileAfterContributionInstall(ctx context.Context, extID domain.ExtensionID) error {
	return b.reconciler.ReconcileExtension(ctx, extID)
}

func (b *DesktopPetPluginBoundary) MarkContributionAvailable(ctx context.Context, extID domain.ExtensionID, contrib domain.ContributionDefinition) error {
	return b.reconciler.HandleEvent(ctx, LifecycleEvent{
		Phase:        PhaseEnabled,
		ExtensionID:  extID,
		Timestamp:    time.Now().UTC(),
		Contribution: contrib,
	})
}

func (b *DesktopPetPluginBoundary) MarkContributionUnavailable(ctx context.Context, extID domain.ExtensionID, contrib domain.ContributionDefinition) error {
	return b.reconciler.HandleEvent(ctx, LifecycleEvent{
		Phase:        PhaseDisabled,
		ExtensionID:  extID,
		Timestamp:    time.Now().UTC(),
		Contribution: contrib,
	})
}

func (b *DesktopPetPluginBoundary) DetachAfterContributionUninstall(ctx context.Context, extID domain.ExtensionID) error {
	return b.reconciler.DetachExtension(ctx, extID)
}

func (b *DesktopPetPluginBoundary) View() ContributionRegistryView {
	return b.reconciler.View()
}

func (b *DesktopPetPluginBoundary) Find(ref ContributionRef) (ContributionRegistration, bool) {
	return b.reconciler.Get(ref)
}

func (b *DesktopPetPluginBoundary) IsExecutable(ref ContributionRef) bool {
	return b.reconciler.IsExecutable(ref)
}

func (b *DesktopPetPluginBoundary) FindByExt(extID string) []ContributionRegistration {
	return b.View().FindByExt(extID)
}

func (b *DesktopPetPluginBoundary) FindByPlugin(extID, pluginID string) []ContributionRegistration {
	return b.View().FindByPlugin(extID, pluginID)
}

func (b *DesktopPetPluginBoundary) ValidateActionInvocation(ctx context.Context, ref ContributionRef) error {
	reg, ok := b.reconciler.Get(ref)
	if !ok {
		return fmtNotFound(ref)
	}
	if reg.Status == ContributionStatusInvalid {
		return fmt.Errorf("%w: contribution %s is invalid: %s", ErrInvalidContribution, ref.Key(), reg.ErrorMessage)
	}
	if !reg.IsExecutable() {
		return fmt.Errorf("%w: contribution %s is not executable (status=%s)", ErrUnavailable, ref.Key(), reg.Status)
	}
	if reg.Action == nil {
		return fmt.Errorf("%w: contribution %s has no action descriptor", ErrInvalidContribution, ref.Key())
	}
	return nil
}

func (b *DesktopPetPluginBoundary) ValidateResourceAccess(ctx context.Context, ref ContributionRef) error {
	reg, ok := b.reconciler.Get(ref)
	if !ok {
		return fmtNotFound(ref)
	}
	if reg.Status == ContributionStatusInvalid {
		return fmt.Errorf("%w: contribution %s is invalid: %s", ErrInvalidContribution, ref.Key(), reg.ErrorMessage)
	}
	if reg.Status == ContributionStatusDetached {
		return fmt.Errorf("%w: contribution %s has been detached", ErrUnavailable, ref.Key())
	}
	if reg.Resource == nil {
		return fmt.Errorf("%w: contribution %s has no resource descriptor", ErrInvalidContribution, ref.Key())
	}
	return nil
}

func (b *DesktopPetPluginBoundary) ValidateRuntimeCapability(ctx context.Context, ref ContributionRef) error {
	reg, ok := b.reconciler.Get(ref)
	if !ok {
		return fmtNotFound(ref)
	}
	if reg.Status == ContributionStatusInvalid {
		return fmt.Errorf("%w: contribution %s is invalid: %s", ErrInvalidContribution, ref.Key(), reg.ErrorMessage)
	}
	if !reg.IsExecutable() {
		return fmt.Errorf("%w: contribution %s is not executable (status=%s)", ErrUnavailable, ref.Key(), reg.Status)
	}
	if reg.Runtime == nil {
		return fmt.Errorf("%w: contribution %s has no runtime descriptor", ErrInvalidContribution, ref.Key())
	}
	return nil
}

func (b *DesktopPetPluginBoundary) ValidateFloatingWindowScope(ctx context.Context, ref ContributionRef, requestedOp string) error {
	reg, ok := b.reconciler.Get(ref)
	if !ok {
		return fmtNotFound(ref)
	}
	if reg.Status == ContributionStatusInvalid {
		return fmt.Errorf("%w: contribution %s is invalid: %s", ErrInvalidContribution, ref.Key(), reg.ErrorMessage)
	}
	if !reg.IsExecutable() {
		return fmt.Errorf("%w: contribution %s is not executable for window op '%s'", ErrUnavailable, ref.Key(), requestedOp)
	}
	if reg.Window == nil {
		return fmt.Errorf("%w: contribution %s has no floating window descriptor", ErrInvalidContribution, ref.Key())
	}
	if !windowOpSupported(reg.Window, requestedOp) {
		return fmt.Errorf("%w: floating window op '%s' not supported by %s", ErrScopeDenied, requestedOp, ref.Key())
	}
	return nil
}

func windowOpSupported(fd *FloatingWindowCapabilityDescriptor, op string) bool {
	switch op {
	case "show":
		return fd.SupportsShow
	case "hide":
		return fd.SupportsHide
	case "position":
		return fd.SupportsPosition
	case "size":
		return fd.SupportsSize
	case "opacity":
		return fd.SupportsOpacity
	case "content":
		return fd.SupportsContent
	default:
		return false
	}
}

func fmtNotFound(ref ContributionRef) error {
	return fmt.Errorf("%w: %s", ErrNotFound, ref.Key())
}
