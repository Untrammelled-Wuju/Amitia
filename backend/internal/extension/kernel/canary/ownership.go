package canary

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type BackgroundOwnershipResolver struct {
	ownerships map[string]*BackgroundOwnership
	mu         sync.RWMutex
}

func NewBackgroundOwnershipResolver() *BackgroundOwnershipResolver {
	return &BackgroundOwnershipResolver{
		ownerships: make(map[string]*BackgroundOwnership),
	}
}

func (r *BackgroundOwnershipResolver) ownershipKey(extensionID string, bgType BackgroundType, resourceID string) string {
	return extensionID + ":" + string(bgType) + ":" + resourceID
}

func (r *BackgroundOwnershipResolver) AcquireOwnership(ctx context.Context, extensionID string, bgType BackgroundType, resourceID string, generation int64, leaseDuration time.Duration) (*BackgroundOwnership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.ownershipKey(extensionID, bgType, resourceID)
	existing, ok := r.ownerships[key]
	if ok {
		if existing.LeaseExpiresAt != nil && existing.LeaseExpiresAt.After(time.Now().UTC()) && existing.OwnerGeneration != generation {
			return nil, fmt.Errorf("canary: ownership held by generation %d until %v", existing.OwnerGeneration, existing.LeaseExpiresAt)
		}
	}

	now := time.Now().UTC()
	leaseExpiry := now.Add(leaseDuration)
	ownership := &BackgroundOwnership{
		ExtensionID:     extensionID,
		BGType:          bgType,
		ResourceID:      resourceID,
		OwnerGeneration: generation,
		AcquiredAt:      now,
		LeaseExpiresAt:  &leaseExpiry,
	}
	r.ownerships[key] = ownership
	return ownership, nil
}

func (r *BackgroundOwnershipResolver) ReleaseOwnership(ctx context.Context, extensionID string, bgType BackgroundType, resourceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.ownershipKey(extensionID, bgType, resourceID)
	delete(r.ownerships, key)
	return nil
}

func (r *BackgroundOwnershipResolver) CheckOwnership(ctx context.Context, extensionID string, bgType BackgroundType, resourceID string) (*BackgroundOwnership, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := r.ownershipKey(extensionID, bgType, resourceID)
	ownership, ok := r.ownerships[key]
	if !ok {
		return nil, nil
	}
	if ownership.LeaseExpiresAt != nil && ownership.LeaseExpiresAt.Before(time.Now().UTC()) {
		return nil, nil
	}
	return ownership, nil
}

func (r *BackgroundOwnershipResolver) TransferOwnership(ctx context.Context, extensionID string, bgType BackgroundType, resourceID string, fromGen, toGen int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.ownershipKey(extensionID, bgType, resourceID)
	existing, ok := r.ownerships[key]
	if !ok {
		return fmt.Errorf("canary: no ownership to transfer for %s", key)
	}
	if existing.OwnerGeneration != fromGen {
		return fmt.Errorf("canary: current owner is generation %d, expected %d", existing.OwnerGeneration, fromGen)
	}
	existing.OwnerGeneration = toGen
	existing.AcquiredAt = time.Now().UTC()
	return nil
}

func (r *BackgroundOwnershipResolver) ListOwnershipByExtension(ctx context.Context, extensionID string) ([]BackgroundOwnership, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []BackgroundOwnership
	for _, o := range r.ownerships {
		if o.ExtensionID == extensionID {
			out = append(out, *o)
		}
	}
	return out, nil
}
