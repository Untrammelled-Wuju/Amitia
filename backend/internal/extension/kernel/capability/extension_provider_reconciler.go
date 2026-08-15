package capability

import (
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type ExtensionProviderReconciler struct {
	lifecycle *ProviderLifecycleService
	registry  *ProviderRegistry
}

func NewExtensionProviderReconciler(lifecycle *ProviderLifecycleService, registry *ProviderRegistry) *ExtensionProviderReconciler {
	return &ExtensionProviderReconciler{
		lifecycle: lifecycle,
		registry:  registry,
	}
}

func (r *ExtensionProviderReconciler) ReconcileDefinitions(def domain.ExtensionDefinition) error {
	defs, err := ProviderDefinitionsFromExtension(def)
	if err != nil {
		return fmt.Errorf("reconcile provider definitions: %w", err)
	}

	existing := r.registry.ListByExtension(string(def.ID))
	existingByID := make(map[ProviderID]*CapabilityProviderDefinition)
	for _, d := range existing {
		if d != nil {
			existingByID[d.ID] = d
		}
	}

	defByID := make(map[ProviderID]CapabilityProviderDefinition)
	for _, d := range defs {
		defByID[d.ID] = d
	}

	for id := range existingByID {
		if _, ok := defByID[id]; !ok {
			if r.lifecycle != nil {
				if _, err := r.lifecycle.UnregisterProvider(id); err != nil {
					return fmt.Errorf("unregister provider %s: %w", id, err)
				}
			} else {
				r.registry.DeregisterDefinitionCascade(id)
			}
		}
	}

	for _, d := range defs {
		if r.lifecycle != nil {
			if err := r.lifecycle.RegisterProvider(d); err != nil {
				return fmt.Errorf("register provider %s: %w", d.ID, err)
			}
		} else {
			if err := r.registry.RegisterDefinition(d); err != nil {
				return fmt.Errorf("register provider %s: %w", d.ID, err)
			}
		}
	}

	return nil
}

func (r *ExtensionProviderReconciler) RemoveExtension(extensionID string) error {
	defs := r.registry.ListByExtension(extensionID)
	for _, d := range defs {
		if d == nil {
			continue
		}
		if r.lifecycle != nil {
			if _, err := r.lifecycle.UnregisterProvider(d.ID); err != nil {
				return fmt.Errorf("unregister provider %s: %w", d.ID, err)
			}
		} else {
			r.registry.DeregisterDefinitionCascade(d.ID)
		}
	}
	return nil
}

func (r *ExtensionProviderReconciler) SnapshotExtension(extensionID string) []*CapabilityProviderDefinition {
	return r.registry.ListByExtension(extensionID)
}

func (r *ExtensionProviderReconciler) RestoreExtension(defs []*CapabilityProviderDefinition) error {
	if len(defs) == 0 {
		return nil
	}
	extensionID := defs[0].ExtensionID
	current := r.registry.ListByExtension(extensionID)
	currentByID := make(map[ProviderID]*CapabilityProviderDefinition)
	for _, d := range current {
		if d != nil {
			currentByID[d.ID] = d
		}
	}

	for _, d := range current {
		if d == nil {
			continue
		}
		if _, ok := findProviderDefByID(defs, d.ID); !ok {
			if r.lifecycle != nil {
				if _, err := r.lifecycle.UnregisterProvider(d.ID); err != nil {
					return fmt.Errorf("restore: unregister provider %s: %w", d.ID, err)
				}
			} else {
				r.registry.DeregisterDefinitionCascade(d.ID)
			}
		}
	}

	for _, d := range defs {
		if d == nil {
			continue
		}
		if r.lifecycle != nil {
			if err := r.lifecycle.RegisterProvider(*d); err != nil {
				return fmt.Errorf("restore: register provider %s: %w", d.ID, err)
			}
		} else {
			if err := r.registry.RegisterDefinition(*d); err != nil {
				return fmt.Errorf("restore: register provider %s: %w", d.ID, err)
			}
		}
	}
	return nil
}

func findProviderDefByID(defs []*CapabilityProviderDefinition, id ProviderID) (*CapabilityProviderDefinition, bool) {
	for _, d := range defs {
		if d != nil && d.ID == id {
			return d, true
		}
	}
	return nil, false
}
