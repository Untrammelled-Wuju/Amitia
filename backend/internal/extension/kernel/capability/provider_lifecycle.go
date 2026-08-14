package capability

import (
	"context"
	"sort"
	"time"
)

type ProviderLifecycleService struct {
	registry *ProviderRegistry
	events   ProviderEventSink
}

func NewProviderLifecycleService(registry *ProviderRegistry, events ProviderEventSink) *ProviderLifecycleService {
	return &ProviderLifecycleService{
		registry: registry,
		events:   events,
	}
}

func (s *ProviderLifecycleService) RegisterProvider(def CapabilityProviderDefinition) error {
	if s.events == nil {
		return s.registry.RegisterDefinition(def)
	}

	now := time.Now().UTC()
	old, found := s.registry.GetByID(def.ID)

	if err := s.registry.RegisterDefinition(def); err != nil {
		return err
	}

	newDef, _ := s.registry.GetByID(def.ID)
	if newDef != nil {
		if found {
			newDef.Revision = old.Revision + 1
		} else {
			newDef.Revision = 1
		}
		s.registry.setDefinition(string(newDef.ID), newDef)
	}

	if found {
		changed := diffDefinitionFields(old, newDef)
		if len(changed) == 0 {
			return nil
		}
		err := s.events.ProviderUpdated(context.Background(), ProviderUpdatedPayload{
			ProviderID:    newDef.ID,
			CapabilityID:  newDef.CapabilityID,
			Kind:          newDef.Kind,
			Placement:     newDef.Placement,
			ExtensionID:   newDef.ExtensionID,
			ModuleID:      newDef.ModuleID,
			Priority:      newDef.Priority,
			ChangedFields: changed,
			Revision:      newDef.Revision,
			OccurredAt:    now,
		})
		if err != nil {
			s.registry.setDefinition(string(old.ID), old)
			return err
		}
		return nil
	}

	err := s.events.ProviderRegistered(context.Background(), ProviderRegisteredPayload{
		ProviderID:   newDef.ID,
		CapabilityID: newDef.CapabilityID,
		Kind:         newDef.Kind,
		Placement:    newDef.Placement,
		ExtensionID:  newDef.ExtensionID,
		ModuleID:     newDef.ModuleID,
		Priority:     newDef.Priority,
		Revision:     newDef.Revision,
		OccurredAt:   now,
	})
	if err != nil {
		s.registry.delDefinition(string(def.ID))
		if old != nil {
			s.registry.setDefinition(string(old.ID), old)
		}
		return err
	}
	return nil
}

func (s *ProviderLifecycleService) UnregisterProvider(id ProviderID) (bool, error) {
	if s.events == nil {
		return s.registry.DeregisterDefinition(id)
	}

	oldDef, found := s.registry.GetByID(id)
	if !found {
		return false, nil
	}

	removed, err := s.registry.DeregisterDefinition(id)
	if err != nil {
		return removed, err
	}

	now := time.Now().UTC()
	err = s.events.ProviderUnregistered(context.Background(), ProviderUnregisteredPayload{
		ProviderID:   oldDef.ID,
		CapabilityID: oldDef.CapabilityID,
		ExtensionID:  oldDef.ExtensionID,
		ModuleID:     oldDef.ModuleID,
		OccurredAt:   now,
	})
	if err != nil {
		s.registry.setDefinition(string(oldDef.ID), oldDef)
		s.registry.rebuildProviderInstanceIndex()
		return false, err
	}
	return true, nil
}

func (s *ProviderLifecycleService) RegisterInstance(inst CapabilityProviderInstance) error {
	if s.events == nil {
		return s.registry.RegisterInstance(inst)
	}

	now := time.Now().UTC()
	old, found := s.registry.GetInstanceByID(inst.ID)

	if err := s.registry.RegisterInstance(inst); err != nil {
		return err
	}

	newInst, _ := s.registry.GetInstanceByID(inst.ID)
	if newInst != nil {
		if found {
			newInst.Revision = old.Revision + 1
		} else {
			newInst.Revision = 1
		}
		s.registry.setInstance(string(newInst.ID), newInst)
	}

	if found {
		emitPayload := toInstanceEventPayload(newInst, now)
		err := s.events.ProviderInstanceUpdated(context.Background(), emitPayload)
		if err != nil {
			s.registry.setInstance(string(old.ID), old)
			s.registry.rebuildProviderInstanceIndex()
			return err
		}
		return nil
	}

	err := s.events.ProviderInstanceRegistered(context.Background(), toInstanceEventPayload(newInst, now))
	if err != nil {
		s.registry.delInstance(string(inst.ID))
		return err
	}
	return nil
}

func (s *ProviderLifecycleService) UnregisterInstance(id ProviderInstanceID) (bool, error) {
	if s.events == nil {
		return s.registry.DeregisterInstance(id)
	}

	old, found := s.registry.GetInstanceByID(id)
	if !found {
		return false, nil
	}

	removed, err := s.registry.DeregisterInstance(id)
	if err != nil {
		return removed, err
	}

	now := time.Now().UTC()
	payload := toInstanceEventPayload(old, now)
	err = s.events.ProviderInstanceUnregistered(context.Background(), payload)
	if err != nil {
		s.registry.setInstance(string(old.ID), old)
		s.registry.rebuildProviderInstanceIndex()
		return false, err
	}
	return true, nil
}

func (s *ProviderLifecycleService) UpdateInstanceAvailability(id ProviderInstanceID, availability ProviderAvailabilityState) error {
	if s.events == nil {
		return s.registry.UpdateInstanceAvailability(id, availability)
	}

	old, found := s.registry.GetInstanceByID(id)
	if !found {
		return ErrProviderInstanceNotFound
	}

	if old.Availability == availability {
		return nil
	}

	oldClone := *old

	if err := s.registry.UpdateInstanceAvailability(id, availability); err != nil {
		return err
	}

	newInst, _ := s.registry.GetInstanceByID(id)
	now := time.Now().UTC()
	err := s.events.ProviderInstanceAvailabilityChanged(context.Background(), ProviderInstanceAvailabilityChangedPayload{
		ProviderInstanceEventPayload: toInstanceEventPayload(newInst, now),
		Previous:                     oldClone.Availability,
		Current:                      availability,
	})
	if err != nil {
		s.registry.setInstance(string(old.ID), &oldClone)
		s.registry.rebuildProviderInstanceIndex()
		return err
	}
	return nil
}

func (s *ProviderLifecycleService) UpdateInstanceHealth(id ProviderInstanceID, health HealthStatus) error {
	if s.events == nil {
		return s.registry.UpdateInstanceHealth(id, health)
	}

	old, found := s.registry.GetInstanceByID(id)
	if !found {
		return ErrProviderInstanceNotFound
	}

	if old.Health == health {
		return nil
	}

	oldClone := *old

	if err := s.registry.UpdateInstanceHealth(id, health); err != nil {
		return err
	}

	newInst, _ := s.registry.GetInstanceByID(id)
	now := time.Now().UTC()
	err := s.events.ProviderInstanceHealthChanged(context.Background(), ProviderInstanceHealthChangedPayload{
		ProviderInstanceEventPayload: toInstanceEventPayload(newInst, now),
		Previous:                     oldClone.Health,
		Current:                      health,
	})
	if err != nil {
		s.registry.setInstance(string(old.ID), &oldClone)
		s.registry.rebuildProviderInstanceIndex()
		return err
	}
	return nil
}

func toInstanceEventPayload(inst *CapabilityProviderInstance, at time.Time) ProviderInstanceEventPayload {
	if inst == nil {
		return ProviderInstanceEventPayload{}
	}
	return ProviderInstanceEventPayload{
		ProviderInstanceID: inst.ID,
		ProviderID:         inst.ProviderID,
		CapabilityID:       inst.CapabilityID,
		Placement:          inst.Placement,
		UserID:             inst.UserID,
		DeviceID:           inst.DeviceID,
		RuntimeID:          inst.RuntimeID,
		RuntimeInstanceID:  inst.RuntimeInstanceID,
		Health:             inst.Health,
		Availability:       inst.Availability,
		Revision:           inst.Revision,
		OccurredAt:         at,
	}
}

func diffDefinitionFields(old, new *CapabilityProviderDefinition) []string {
	var changed []string
	if old.Kind != new.Kind {
		changed = append(changed, "kind")
	}
	if old.Placement != new.Placement {
		changed = append(changed, "placement")
	}
	if old.ExtensionID != new.ExtensionID {
		changed = append(changed, "extensionId")
	}
	if old.ModuleID != new.ModuleID {
		changed = append(changed, "moduleId")
	}
	if old.Priority != new.Priority {
		changed = append(changed, "priority")
	}
	if old.CapabilityID != new.CapabilityID {
		changed = append(changed, "capabilityId")
	}
	if len(changed) > 0 {
		sort.Strings(changed)
	}
	return changed
}
