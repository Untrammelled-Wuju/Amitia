package kernel

import (
	"context"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

// stopExtensionRuntimeInstances stops every running runtime instance owned by the
// extension and drops the matching service runtime definitions. Dropping the
// definitions is required because a package swap replaces the bundle directory
// that the definitions point at, and a definition can only be replaced after its
// instances have reached a terminal state.
func (r *Runtime) stopExtensionRuntimeInstances(ctx context.Context, extID domain.ExtensionID, reason runtime_supervisor.StopReason) error {
	if r.container == nil {
		return nil
	}
	modules, err := r.container.ModuleRepository.ListModules(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: list modules for runtime shutdown: %w", err)
	}
	gameHostOwned, err := r.gameHostOwnedRuntimeModules(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: resolve game runtime ownership: %w", err)
	}
	extensionID := string(extID)
	var failures []error
	for _, mod := range modules {
		if isGameHostOwnedRuntimeModule(gameHostOwned, mod.ID) {
			continue
		}
		if mod.Runtime == nil || mod.Runtime.Type == "" || mod.Runtime.Type == domain.RuntimeTypeBuiltin {
			continue
		}
		definitionID := runtime_supervisor.BuildRuntimeDefinitionID(extensionID, string(mod.ID), mod.Runtime.Type)
		if r.container.RuntimeSupervisor != nil {
			snapshot := r.container.RuntimeSupervisor.Snapshot(ctx, definitionID)
			for _, instance := range snapshot.Instances {
				if stopErr := r.container.RuntimeSupervisor.Stop(ctx, instance.InstanceID, reason); stopErr != nil {
					failures = append(failures, fmt.Errorf("stop runtime instance %s: %w", instance.InstanceID, stopErr))
				}
			}
		}
		if mod.Runtime.Type != domain.RuntimeTypeService || r.container.TrustedServiceSupervisor == nil {
			continue
		}
		if _, getErr := r.container.TrustedServiceSupervisor.GetDefinition(string(definitionID)); getErr != nil {
			continue
		}
		if err := r.container.TrustedServiceSupervisor.Unregister(string(definitionID)); err != nil {
			failures = append(failures, fmt.Errorf("unregister service definition %s: %w", definitionID, err))
		}
	}
	return errors.Join(failures...)
}

// quiesceExtensionForPackageSwap stops the extension runtime and tears down its
// in-memory registrations so that a package update or rollback can install the
// next generation without serving definitions or UI schemas of the old bundle.
func (r *Runtime) quiesceExtensionForPackageSwap(ctx context.Context, extID domain.ExtensionID, reason runtime_supervisor.StopReason) error {
	if r.container == nil {
		return nil
	}
	if r.container.ProviderInstanceReconciler != nil && r.container.ProviderLifecycle != nil {
		r.container.ProviderLifecycle.DrainExtension(string(extID))
	}
	if err := r.stopExtensionRuntimeInstances(ctx, extID, reason); err != nil {
		return err
	}
	if r.container.ContributionInstaller != nil {
		if err := r.container.ContributionInstaller.UninstallContributions(ctx, extID); err != nil {
			return fmt.Errorf("kernel: uninstall contributions before package swap: %w", err)
		}
	}
	if r.container.ProviderInstanceReconciler != nil {
		if err := r.container.ProviderInstanceReconciler.DeactivateExtension(string(extID)); err != nil {
			return fmt.Errorf("kernel: deactivate provider instances before package swap: %w", err)
		}
	}
	return nil
}

// restoreExtensionAfterPackageSwap brings an extension back to its pre-update
// enablement. Enabled extensions start the new generation runtime and refresh
// every definition, page and schema. Disabled extensions stay inert, exactly
// like an extension that was never activated, so the next enable activation
// installs the new generation definitions on its own.
func (r *Runtime) restoreExtensionAfterPackageSwap(ctx context.Context, extID domain.ExtensionID, wasEnabled bool) error {
	if !wasEnabled {
		return nil
	}
	if err := r.Enable(ctx, string(extID)); err != nil {
		return fmt.Errorf("kernel: activate %s after package swap: %w", extID, err)
	}
	return nil
}
