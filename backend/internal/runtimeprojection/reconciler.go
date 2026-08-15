package runtimeprojection

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ProviderInstanceReconciler interface {
	ReconcileOnOnline(ctx context.Context, projection RuntimeProjection) error
	ReconcileOnOffline(ctx context.Context, runtimeID runtimeidentity.RuntimeID) error
}

type CapabilityRefreshPort interface {
	RefreshAvailability(ctx context.Context, runtimeID runtimeidentity.RuntimeID, capabilities []capability.CapabilityID) error
}

type RuntimeProjectionReconciler struct {
	presence     RuntimePresencePort
	providers    ProviderInstanceReconciler
	capabilities CapabilityRefreshPort
}

func NewRuntimeProjectionReconciler(
	presence RuntimePresencePort,
	providers ProviderInstanceReconciler,
	capabilities CapabilityRefreshPort,
) *RuntimeProjectionReconciler {
	return &RuntimeProjectionReconciler{
		presence:     presence,
		providers:    providers,
		capabilities: capabilities,
	}
}

func (r *RuntimeProjectionReconciler) HandleHello(ctx context.Context, event PresenceEvent) error {
	if err := r.presence.UpsertPresence(ctx, event); err != nil {
		return fmt.Errorf("hello: upsert presence: %w", err)
	}

	proj := RuntimeProjection{
		RuntimeID:   event.RuntimeID,
		SessionID:   event.SessionID,
		Identity:    event.Identity,
		Placement:   event.Placement,
		Online:      true,
		Health:      event.Health,
		UpdatedAt:   event.Timestamp,
	}

	if r.providers != nil {
		if err := r.providers.ReconcileOnOnline(ctx, proj); err != nil {
			return fmt.Errorf("hello: provider reconcile: %w", err)
		}
	}

	if r.capabilities != nil {
		if err := r.capabilities.RefreshAvailability(ctx, event.RuntimeID, event.Capabilities); err != nil {
			return fmt.Errorf("hello: capability refresh: %w", err)
		}
	}

	return nil
}

func (r *RuntimeProjectionReconciler) HandleHeartbeat(ctx context.Context, event PresenceEvent) error {
	return r.presence.UpsertPresence(ctx, event)
}

func (r *RuntimeProjectionReconciler) HandleOffline(ctx context.Context, runtimeID runtimeidentity.RuntimeID) error {
	if err := r.presence.RemovePresence(ctx, runtimeID); err != nil {
		return fmt.Errorf("offline: remove presence: %w", err)
	}

	if r.providers != nil {
		if err := r.providers.ReconcileOnOffline(ctx, runtimeID); err != nil {
			return fmt.Errorf("offline: provider reconcile: %w", err)
		}
	}

	return nil
}

func (r *RuntimeProjectionReconciler) ReconcileProjection(ctx context.Context, projection RuntimeProjection) (RuntimeProjection, error) {
	projection.UpdatedAt = time.Now().UTC()

	for i, ext := range projection.ExtensionInstances {
		ext.Observed = DeriveExtensionObserved(
			ExtensionDesiredState{ExtensionID: ext.ExtensionID, ModuleID: ext.ModuleID, Enabled: ext.Desired},
			ExtensionObservedState{ExtensionID: ext.ExtensionID, Ready: projection.Online && projection.Health == "healthy"},
		)
		projection.ExtensionInstances[i] = ext
	}

	return projection, nil
}

type ReconcileHook interface {
	AfterReconcile(ctx context.Context, projection RuntimeProjection, execCtx *execution.ExecutionContext) error
}
