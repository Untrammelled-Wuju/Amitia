package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type BuiltinProviderReconciler struct {
	registry  *ProviderRegistry
	lifecycle *ProviderLifecycleService
}

func NewBuiltinProviderReconciler(registry *ProviderRegistry, lifecycle *ProviderLifecycleService) *BuiltinProviderReconciler {
	return &BuiltinProviderReconciler{
		registry:  registry,
		lifecycle: lifecycle,
	}
}

func (r *BuiltinProviderReconciler) Reconcile() error {
	if err := r.reconcileSearchWeb(); err != nil {
		return fmt.Errorf("reconcile search.web: %w", err)
	}
	return nil
}

func (r *BuiltinProviderReconciler) reconcileSearchWeb() error {
	providerDef := CapabilityProviderDefinition{
		ID:           "com.amitia.builtin.search.provider",
		CapabilityID: "search.web",
		Kind:         ProviderKindBuiltin,
		Placement:    ProviderPlacementCore,
		ExtensionID:  "com.amitia.builtin.search",
		ModuleID:     "search-runtime",
		Runtime: RuntimeBinding{
			RuntimeType:  RuntimeTypeSearch,
			RuntimeID:    "search-runtime",
			HandlerName:  "search.general",
		},
		Priority: 100,
	}

	if r.lifecycle != nil {
		if err := r.lifecycle.RegisterProvider(providerDef); err != nil {
			return fmt.Errorf("register search.web provider: %w", err)
		}
	} else {
		if err := r.registry.RegisterDefinition(providerDef); err != nil {
			return fmt.Errorf("register search.web provider: %w", err)
		}
	}

	instanceID := r.buildInstanceID(providerDef)
	now := time.Now().UTC()
	instance := CapabilityProviderInstance{
		ID:           instanceID,
		ProviderID:   providerDef.ID,
		CapabilityID: providerDef.CapabilityID,
		Placement:    providerDef.Placement,
		ExtensionID:  providerDef.ExtensionID,
		ModuleID:     providerDef.ModuleID,
		Health:       HealthReady,
		Availability: ProviderAvailabilityAvailable,
		RegisteredAt: now,
		UpdatedAt:    now,
	}

	if r.lifecycle != nil {
		if err := r.lifecycle.RegisterInstance(instance); err != nil {
			return fmt.Errorf("register search.web instance: %w", err)
		}
	} else {
		if err := r.registry.RegisterInstance(instance); err != nil {
			return fmt.Errorf("register search.web instance: %w", err)
		}
	}

	return nil
}

func (r *BuiltinProviderReconciler) buildInstanceID(def CapabilityProviderDefinition) ProviderInstanceID {
	seed := string(def.ID) + "|" + def.ModuleID + "|builtin"
	sum := sha256.Sum256([]byte(seed))
	return ProviderInstanceID("provinst_" + hex.EncodeToString(sum[:16]))
}
