package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ProviderInstanceReconciler interface {
	ActivateExtension(def domain.ExtensionDefinition, runtimeResults map[domain.ModuleID]RuntimeReadyResult) ([]ProviderInstanceID, error)
	DeactivateExtension(extensionID string) error
	ReconcileRuntimeHealth(instanceID ProviderInstanceID, health HealthStatus) error
}

type RuntimeReadyResult struct {
	InstanceID string
	Health     string
	RuntimeID  runtimeidentity.RuntimeID
}

type providerInstanceReconciler struct {
	lifecycle    *ProviderLifecycleService
	registry     *ProviderRegistry
	runtimeIdent runtimeidentity.Identity
}

func NewProviderInstanceReconciler(lifecycle *ProviderLifecycleService, registry *ProviderRegistry, ident runtimeidentity.Identity) ProviderInstanceReconciler {
	return &providerInstanceReconciler{
		lifecycle:    lifecycle,
		registry:     registry,
		runtimeIdent: ident,
	}
}

func (r *providerInstanceReconciler) ActivateExtension(def domain.ExtensionDefinition, runtimeResults map[domain.ModuleID]RuntimeReadyResult) ([]ProviderInstanceID, error) {
	defs, err := ProviderDefinitionsFromExtension(def)
	if err != nil {
		return nil, fmt.Errorf("activate: build definitions: %w", err)
	}

	var activated []ProviderInstanceID

	for _, d := range defs {
		modID := domain.ModuleID(d.ModuleID)
		result, hasResult := runtimeResults[modID]

		if d.Placement == ProviderPlacementDevice {
			if r.runtimeIdent.DeviceID == "" || r.runtimeIdent.RuntimeID == "" {
				continue
			}
		}

		instanceID := r.buildInstanceID(d, result)

		health := HealthReady
		if hasResult {
			if result.Health == "degraded" {
				health = HealthDegraded
			}
		} else if d.Kind != ProviderKindBuiltin {
			health = HealthUnknown
		}

		availability := ProviderAvailabilityAvailable
		if !hasResult && d.Kind != ProviderKindBuiltin {
			availability = ProviderAvailabilityUnknown
		}

		inst := CapabilityProviderInstance{
			ID:                instanceID,
			ProviderID:        d.ID,
			CapabilityID:      d.CapabilityID,
			Placement:         d.Placement,
			Health:            health,
			Availability:      availability,
			ExtensionID:       string(d.ExtensionID),
			ModuleID:          d.ModuleID,
			UserID:            r.runtimeIdent.UserID,
			DeviceID:          r.runtimeIdent.DeviceID,
			RuntimeID:         r.runtimeIdent.RuntimeID,
			RuntimeInstanceID: result.InstanceID,
		}

		if d.Placement == ProviderPlacementDevice && r.runtimeIdent.DeviceID != "" {
			inst.DeviceID = r.runtimeIdent.DeviceID
			inst.RuntimeID = r.runtimeIdent.RuntimeID
		}

		if r.lifecycle != nil {
			if err := r.lifecycle.RegisterInstance(inst); err != nil {
				return activated, fmt.Errorf("activate: register instance %s: %w", instanceID, err)
			}
		} else {
			if err := r.registry.RegisterInstance(inst); err != nil {
				return activated, fmt.Errorf("activate: register instance %s: %w", instanceID, err)
			}
		}

		activated = append(activated, instanceID)
	}

	return activated, nil
}

func (r *providerInstanceReconciler) DeactivateExtension(extensionID string) error {
	defs := r.registry.ListByExtension(extensionID)
	for _, d := range defs {
		if d == nil {
			continue
		}
		instances := r.registry.ListInstancesByProvider(d.ID)
		for _, inst := range instances {
			if inst == nil {
				continue
			}
			if r.lifecycle != nil {
				if _, err := r.lifecycle.UnregisterInstance(inst.ID); err != nil {
					return fmt.Errorf("deactivate: unregister instance %s: %w", inst.ID, err)
				}
			} else {
				r.registry.DeregisterInstance(inst.ID)
			}
		}
	}
	return nil
}

func (r *providerInstanceReconciler) ReconcileRuntimeHealth(instanceID ProviderInstanceID, health HealthStatus) error {
	if r.lifecycle != nil {
		return r.lifecycle.UpdateInstanceHealth(instanceID, health)
	}
	return r.registry.UpdateInstanceHealth(instanceID, health)
}

func (r *providerInstanceReconciler) buildInstanceID(def CapabilityProviderDefinition, result RuntimeReadyResult) ProviderInstanceID {
	seed := string(def.ID) + "|" + string(r.runtimeIdent.RuntimeID) + "|" + result.InstanceID
	sum := sha256.Sum256([]byte(seed))
	return ProviderInstanceID("provinst_" + hex.EncodeToString(sum[:16]))
}
