package mcp_migration

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type ResourceOwnership struct {
	ResourceID       string
	ResourceType     string
	OwnerExtensionID string
	OwnerModuleID    string
	OwnerGeneration  int64
	AcquiredAt       time.Time
}

type OwnershipRegistry struct {
	mu          sync.RWMutex
	ownerships  map[string]*ResourceOwnership
}

func NewOwnershipRegistry() *OwnershipRegistry {
	return &OwnershipRegistry{
		ownerships: make(map[string]*ResourceOwnership),
	}
}

func ownershipKey(resourceType, resourceID string) string {
	return fmt.Sprintf("%s:%s", resourceType, resourceID)
}

func (r *OwnershipRegistry) Acquire(resourceType, resourceID, extID, modID string, gen int64) error {
	if resourceType == "" || resourceID == "" || extID == "" {
		return ErrInvalidOwnership
	}
	key := ownershipKey(resourceType, resourceID)
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.ownerships[key]
	if ok {
		if existing.OwnerExtensionID != extID {
			return fmt.Errorf("%w: %s owned by %s", ErrResourceAlreadyOwned, key, existing.OwnerExtensionID)
		}
		existing.OwnerModuleID = modID
		existing.OwnerGeneration = gen
		existing.AcquiredAt = time.Now().UTC()
		return nil
	}
	r.ownerships[key] = &ResourceOwnership{
		ResourceID:       resourceID,
		ResourceType:     resourceType,
		OwnerExtensionID: extID,
		OwnerModuleID:    modID,
		OwnerGeneration:  gen,
		AcquiredAt:       time.Now().UTC(),
	}
	return nil
}

func (r *OwnershipRegistry) Release(resourceType, resourceID, ownerExtID string) error {
	key := ownershipKey(resourceType, resourceID)
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.ownerships[key]
	if !ok {
		return fmt.Errorf("%w: %s", ErrResourceNotOwned, key)
	}
	if existing.OwnerExtensionID != ownerExtID {
		return fmt.Errorf("%w: %s owned by %s", ErrNotResourceOwner, key, existing.OwnerExtensionID)
	}
	delete(r.ownerships, key)
	return nil
}

func (r *OwnershipRegistry) GetOwner(resourceType, resourceID string) (*ResourceOwnership, bool) {
	key := ownershipKey(resourceType, resourceID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	ownership, ok := r.ownerships[key]
	if !ok {
		return nil, false
	}
	copied := *ownership
	return &copied, true
}

func (r *OwnershipRegistry) ReleaseByExtension(extID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for key, ownership := range r.ownerships {
		if ownership.OwnerExtensionID == extID {
			delete(r.ownerships, key)
			count++
		}
	}
	return count
}

var (
	ErrInvalidOwnership     = errors.New("mcp_migration: invalid ownership request")
	ErrResourceAlreadyOwned = errors.New("mcp_migration: resource already owned")
	ErrResourceNotOwned     = errors.New("mcp_migration: resource not owned")
	ErrNotResourceOwner     = errors.New("mcp_migration: not the resource owner")
)
