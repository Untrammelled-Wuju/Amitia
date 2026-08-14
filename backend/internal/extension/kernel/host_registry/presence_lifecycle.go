package host_registry

import (
	"context"
	"errors"
	"time"
)

type HostPresenceLifecycleService struct {
	registry *Registry
	events   PresenceEventSink
}

func NewHostPresenceLifecycleService(registry *Registry, events PresenceEventSink) *HostPresenceLifecycleService {
	return &HostPresenceLifecycleService{
		registry: registry,
		events:   events,
	}
}

func (s *HostPresenceLifecycleService) Bind(ctx context.Context, binding RuntimeSessionBinding) (*RuntimeEntry, error) {
	if binding.UserID == "" || binding.DeviceID == "" || binding.RuntimeID == "" {
		return nil, ErrInvalidRegistryEntry
	}

	at := binding.At
	if at.IsZero() {
		at = time.Now().UTC()
	}

	entry, err := s.registry.BindRuntimeSession(ctx, binding)
	if err != nil {
		return entry, err
	}

	if entry == nil {
		return nil, nil
	}

	if s.events != nil && entry.PresenceState == PresenceStateReady {
		if err := s.events.PresenceReady(ctx, PresenceDomainEvent{
			Type:  PresenceEventReady,
			Entry: *entry,
			At:    at,
		}); err != nil {
			return entry, err
		}
	}

	return entry, nil
}

func (s *HostPresenceLifecycleService) Disconnect(ctx context.Context, binding RuntimeSessionBinding) error {
	if binding.RuntimeSessionID == "" || binding.ConnectionGeneration < 1 {
		return ErrStaleRuntimeSessionBinding
	}

	at := binding.At
	if at.IsZero() {
		at = time.Now().UTC()
	}

	entryID := RuntimeEntryID(binding.UserID, binding.DeviceID, binding.RuntimeID)
	existing, err := s.registry.GetEntry(ctx, entryID)
	if err != nil && !errors.Is(err, ErrRegistryEntryNotFound) {
		return err
	}

	if err := s.registry.DisconnectRuntimeSession(ctx, binding); err != nil {
		return err
	}

	if s.events != nil && existing != nil && existing.PresenceState != PresenceStateDisconnected {
		cloned := *existing
		cloned.PresenceState = PresenceStateDisconnected
		cloned.LastHeartbeat = at
		if err := s.events.PresenceDisconnected(ctx, PresenceDomainEvent{
			Type:  PresenceEventDisconnected,
			Entry: cloned,
			At:    at,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *HostPresenceLifecycleService) SetDisconnected(ctx context.Context, entryID string) error {
	existing, err := s.registry.GetEntry(ctx, entryID)
	if err != nil && !errors.Is(err, ErrRegistryEntryNotFound) {
		return err
	}

	if err := s.registry.SetEntryDisconnected(ctx, entryID); err != nil {
		return err
	}

	if s.events != nil && existing != nil && existing.PresenceState != PresenceStateDisconnected {
		cloned := *existing
		cloned.PresenceState = PresenceStateDisconnected
		if err := s.events.PresenceDisconnected(ctx, PresenceDomainEvent{
			Type:  PresenceEventDisconnected,
			Entry: cloned,
			At:    time.Now().UTC(),
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *HostPresenceLifecycleService) RegisterEntry(ctx context.Context, entry *RuntimeEntry) error {
	if err := s.registry.RegisterEntry(ctx, entry); err != nil {
		return err
	}

	if s.events != nil && entry.PresenceState == PresenceStateReady {
		cloned := *entry
		if err := s.events.PresenceReady(ctx, PresenceDomainEvent{
			Type:  PresenceEventReady,
			Entry: cloned,
			At:    time.Now().UTC(),
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *HostPresenceLifecycleService) UnregisterEntry(ctx context.Context, entryID string) error {
	existing, err := s.registry.GetEntry(ctx, entryID)
	if err != nil && !errors.Is(err, ErrRegistryEntryNotFound) {
		return err
	}

	if err := s.registry.UnregisterEntry(ctx, entryID); err != nil {
		return err
	}

	if s.events != nil && existing != nil && existing.PresenceState == PresenceStateReady {
		cloned := *existing
		cloned.PresenceState = PresenceStateDisconnected
		if err := s.events.PresenceDisconnected(ctx, PresenceDomainEvent{
			Type:  PresenceEventDisconnected,
			Entry: cloned,
			At:    time.Now().UTC(),
		}); err != nil {
			return err
		}
	}

	return nil
}
