package trust

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type PublisherStore struct {
	mu            sync.RWMutex
	publishers    map[string]*PublisherIdentity
	builtinRoots  map[string]bool
	officialFeeds map[string]bool
}

func NewPublisherStore() *PublisherStore {
	return &PublisherStore{
		publishers:    make(map[string]*PublisherIdentity),
		builtinRoots:  make(map[string]bool),
		officialFeeds: make(map[string]bool),
	}
}

func (s *PublisherStore) RegisterBuiltinRoot(publisherID string, identity PublisherIdentity) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	identity.TrustLevel = TrustLevelOfficial
	identity.Source = TrustSourceBuiltin
	identity.OfficialRoot = true
	if identity.FirstSeenAt.IsZero() {
		identity.FirstSeenAt = time.Now().UTC()
	}
	identity.UpdatedAt = time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.publishers[publisherID] = &identity
	s.builtinRoots[publisherID] = true
	return nil
}

func (s *PublisherStore) RegisterFromOfficialFeed(identity PublisherIdentity) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.publishers[identity.PublisherID]; ok && existing.OfficialRoot {
		return fmt.Errorf("trust: cannot overwrite builtin root %s", identity.PublisherID)
	}
	identity.Source = TrustSourceOfficialFeed
	if identity.FirstSeenAt.IsZero() {
		identity.FirstSeenAt = time.Now().UTC()
	}
	identity.UpdatedAt = time.Now().UTC()
	s.publishers[identity.PublisherID] = &identity
	s.officialFeeds[identity.PublisherID] = true
	return nil
}

func (s *PublisherStore) RegisterUserDecision(identity PublisherIdentity) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.publishers[identity.PublisherID]; ok && existing.OfficialRoot {
		return fmt.Errorf("trust: cannot overwrite builtin root %s", identity.PublisherID)
	}
	identity.Source = TrustSourceUserDecision
	if identity.FirstSeenAt.IsZero() {
		identity.FirstSeenAt = time.Now().UTC()
	}
	identity.UpdatedAt = time.Now().UTC()
	s.publishers[identity.PublisherID] = &identity
	return nil
}

func (s *PublisherStore) RegisterDevelopment(publisherID string, key PublisherKey) error {
	if publisherID == "" {
		return errors.New("trust: publisher id required")
	}
	if err := key.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.publishers[publisherID]; ok && existing.OfficialRoot {
		return fmt.Errorf("trust: cannot overwrite builtin root %s", publisherID)
	}
	now := time.Now().UTC()
	identity := &PublisherIdentity{
		PublisherID: publisherID,
		DisplayName: publisherID,
		Keys:        []PublisherKey{key},
		TrustLevel:  TrustLevelDevelopment,
		Source:      TrustSourceDevelopment,
		FirstSeenAt: now,
		UpdatedAt:   now,
	}
	s.publishers[publisherID] = identity
	return nil
}

func (s *PublisherStore) Get(ctx context.Context, publisherID string) (*PublisherIdentity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	identity, ok := s.publishers[publisherID]
	if !ok {
		return nil, ErrPublisherNotFound
	}
	copied := *identity
	return &copied, nil
}

func (s *PublisherStore) GetKey(ctx context.Context, publisherID, keyID string) (*PublisherKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	identity, ok := s.publishers[publisherID]
	if !ok {
		return nil, ErrPublisherNotFound
	}
	key := identity.FindKey(keyID)
	if key == nil {
		return nil, ErrKeyNotFound
	}
	copied := *key
	return &copied, nil
}

func (s *PublisherStore) Update(ctx context.Context, publisherID string, mutate func(*PublisherIdentity) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, ok := s.publishers[publisherID]
	if !ok {
		return ErrPublisherNotFound
	}
	if identity.OfficialRoot {
		if err := mutate(identity); err != nil {
			return err
		}
		identity.UpdatedAt = time.Now().UTC()
		return nil
	}
	if err := mutate(identity); err != nil {
		return err
	}
	identity.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *PublisherStore) SetTrustLevel(ctx context.Context, publisherID string, level TrustLevel) error {
	if !level.IsValid() {
		return fmt.Errorf("trust: invalid level %s", level)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, ok := s.publishers[publisherID]
	if !ok {
		return ErrPublisherNotFound
	}
	if identity.OfficialRoot && level != TrustLevelOfficial {
		return errors.New("trust: cannot demote official root")
	}
	identity.TrustLevel = level
	identity.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *PublisherStore) RevokeTrust(ctx context.Context, publisherID string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, ok := s.publishers[publisherID]
	if !ok {
		return ErrPublisherNotFound
	}
	if identity.OfficialRoot {
		return errors.New("trust: cannot revoke official root")
	}
	identity.TrustLevel = TrustLevelRevoked
	identity.UpdatedAt = time.Now().UTC()
	for i := range identity.Keys {
		if identity.Keys[i].IsUsable() {
			now := time.Now().UTC()
			identity.Keys[i].State = KeyStateRevoked
			identity.Keys[i].RevokedAt = &now
			identity.Keys[i].RevokedReason = reason
		}
	}
	return nil
}

func (s *PublisherStore) AddKey(ctx context.Context, publisherID string, key PublisherKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, ok := s.publishers[publisherID]
	if !ok {
		return ErrPublisherNotFound
	}
	if err := identity.AddKey(key); err != nil {
		return err
	}
	identity.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *PublisherStore) RotateKey(ctx context.Context, publisherID, oldKeyID, newKeyID string, newPublicKey []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, ok := s.publishers[publisherID]
	if !ok {
		return ErrPublisherNotFound
	}
	return identity.RotateKey(oldKeyID, newKeyID, newPublicKey)
}

func (s *PublisherStore) RevokeKey(ctx context.Context, publisherID, keyID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, ok := s.publishers[publisherID]
	if !ok {
		return ErrPublisherNotFound
	}
	return identity.RevokeKey(keyID, reason)
}

func (s *PublisherStore) List(ctx context.Context) []PublisherIdentity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PublisherIdentity, 0, len(s.publishers))
	for _, identity := range s.publishers {
		result = append(result, *identity)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].PublisherID < result[j].PublisherID
	})
	return result
}

func (s *PublisherStore) IsBuiltinRoot(publisherID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.builtinRoots[publisherID]
}

func (s *PublisherStore) IsOfficialFeed(publisherID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.officialFeeds[publisherID]
}

func (p *PublisherIdentity) Validate() error {
	if p.PublisherID == "" {
		return errors.New("trust: publisher id required")
	}
	if !p.TrustLevel.IsValid() {
		return fmt.Errorf("trust: invalid trust level %s", p.TrustLevel)
	}
	if !p.Source.IsValid() {
		return fmt.Errorf("trust: invalid trust source %s", p.Source)
	}
	for i := range p.Keys {
		if err := p.Keys[i].Validate(); err != nil {
			return err
		}
		if p.Keys[i].PublisherID != p.PublisherID {
			return errors.New("trust: key publisher mismatch")
		}
	}
	return nil
}
