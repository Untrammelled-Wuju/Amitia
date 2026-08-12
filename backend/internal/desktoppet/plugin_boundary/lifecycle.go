package plugin_boundary

import (
	"context"
	"fmt"
)

type LifecycleHook func(ctx context.Context, evt LifecycleEvent) error

type LifecycleHookRegistry struct {
	hooks []LifecycleHook
}

func NewLifecycleHookRegistry() *LifecycleHookRegistry {
	return &LifecycleHookRegistry{}
}

func (r *LifecycleHookRegistry) Register(hook LifecycleHook) {
	if hook == nil {
		return
	}
	r.hooks = append(r.hooks, hook)
}

func (r *LifecycleHookRegistry) Dispatch(ctx context.Context, evt LifecycleEvent) error {
	var lastErr error
	for _, hook := range r.hooks {
		if err := hook(ctx, evt); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func BoundaryHook(boundary *DesktopPetPluginBoundary) LifecycleHook {
	return func(ctx context.Context, evt LifecycleEvent) error {
		return boundary.internalHandleEvent(ctx, evt)
	}
}

func (b *DesktopPetPluginBoundary) internalHandleEvent(ctx context.Context, evt LifecycleEvent) error {
	switch evt.Phase {
	case PhaseInstalled:
		return b.HandleExtensionInstalled(ctx, evt.ExtensionID, evt.Version, evt.OperationID)
	case PhaseEnabled:
		return b.HandleExtensionEnabled(ctx, evt.ExtensionID, evt.Version, evt.OperationID)
	case PhaseDisabled:
		return b.HandleExtensionDisabled(ctx, evt.ExtensionID, evt.Version, evt.OperationID, "")
	case PhaseUpdated:
		return b.HandleExtensionUpdated(ctx, evt.ExtensionID, evt.OldVersion, evt.Version, evt.OperationID)
	case PhaseUninstalled:
		return b.HandleExtensionUninstalled(ctx, evt.ExtensionID, evt.Version, evt.OperationID, "")
	case PhaseReconcileAll:
		return b.ReconcileAfterContributionInstall(ctx, evt.ExtensionID)
	default:
		return fmt.Errorf("plugin_boundary: unknown phase: %s", evt.Phase)
	}
}
