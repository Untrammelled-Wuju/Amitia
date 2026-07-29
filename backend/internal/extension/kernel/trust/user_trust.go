package trust

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type TrustScope string

const (
	TrustScopePackage   TrustScope = "package"
	TrustScopeKey       TrustScope = "key"
	TrustScopePublisher TrustScope = "publisher"
	TrustScopeVersion   TrustScope = "version"
	TrustScopeWorkspace TrustScope = "workspace"
)

type UserTrustDecision struct {
	DecisionID    string     `json:"decision_id"`
	UserID        string     `json:"user_id"`
	PublisherID   string     `json:"publisher_id,omitempty"`
	KeyID         string     `json:"key_id,omitempty"`
	PackageHash   string     `json:"package_hash,omitempty"`
	ExtensionID   string     `json:"extension_id,omitempty"`
	Version       string     `json:"version,omitempty"`
	WorkspacePath string     `json:"workspace_path,omitempty"`
	Scope         TrustScope `json:"scope"`
	GrantedLevel  TrustLevel `json:"granted_level"`
	GrantedAt     time.Time  `json:"granted_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	Reason        string     `json:"reason,omitempty"`
	Revoked       bool       `json:"revoked"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}

func (d UserTrustDecision) IsExpired() bool {
	if d.ExpiresAt == nil {
		return false
	}
	return time.Now().UTC().After(*d.ExpiresAt)
}

func (d UserTrustDecision) IsActive() bool {
	return !d.Revoked && !d.IsExpired()
}

func (d UserTrustDecision) Matches(scope TrustScope, publisherID, keyID, packageHash, extensionID, version string) bool {
	if d.Scope != scope {
		return false
	}
	switch scope {
	case TrustScopePublisher:
		return d.PublisherID == publisherID
	case TrustScopeKey:
		return d.PublisherID == publisherID && d.KeyID == keyID
	case TrustScopePackage:
		return d.PackageHash == packageHash
	case TrustScopeVersion:
		return d.ExtensionID == extensionID && d.Version == version
	case TrustScopeWorkspace:
		return d.WorkspacePath != ""
	}
	return false
}

type UserTrustStore struct {
	mu        sync.RWMutex
	decisions map[string]UserTrustDecision
}

func NewUserTrustStore() *UserTrustStore {
	return &UserTrustStore{
		decisions: make(map[string]UserTrustDecision),
	}
}

func (s *UserTrustStore) Grant(decision UserTrustDecision) error {
	if decision.DecisionID == "" {
		return errors.New("trust: decision id required")
	}
	if decision.Scope == "" {
		return errors.New("trust: scope required")
	}
	if !decision.GrantedLevel.IsValid() {
		return fmt.Errorf("trust: invalid granted level %s", decision.GrantedLevel)
	}
	if decision.GrantedAt.IsZero() {
		decision.GrantedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisions[decision.DecisionID] = decision
	return nil
}

func (s *UserTrustStore) Revoke(decisionID string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.decisions[decisionID]
	if !ok {
		return fmt.Errorf("trust: decision %s not found", decisionID)
	}
	d.Revoked = true
	now := time.Now().UTC()
	d.RevokedAt = &now
	if reason != "" {
		d.Reason = reason
	}
	s.decisions[decisionID] = d
	return nil
}

func (s *UserTrustStore) Lookup(scope TrustScope, publisherID, keyID, packageHash, extensionID, version string) *UserTrustDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var matches []UserTrustDecision
	for _, d := range s.decisions {
		if !d.IsActive() {
			continue
		}
		if d.Matches(scope, publisherID, keyID, packageHash, extensionID, version) {
			matches = append(matches, d)
		}
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].GrantedAt.After(matches[j].GrantedAt)
	})
	d := matches[0]
	return &d
}

func (s *UserTrustStore) List() []UserTrustDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]UserTrustDecision, 0, len(s.decisions))
	for _, d := range s.decisions {
		if d.IsActive() {
			result = append(result, d)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].GrantedAt.After(result[j].GrantedAt)
	})
	return result
}

func (s *UserTrustStore) ListByPublisher(publisherID string) []UserTrustDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []UserTrustDecision
	for _, d := range s.decisions {
		if d.IsActive() && d.PublisherID == publisherID {
			result = append(result, d)
		}
	}
	return result
}

func (s *UserTrustStore) ListByWorkspace(workspacePath string) []UserTrustDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []UserTrustDecision
	for _, d := range s.decisions {
		if d.IsActive() && d.WorkspacePath == workspacePath {
			result = append(result, d)
		}
	}
	return result
}

func (s *UserTrustStore) RevokeWorkspace(ctx context.Context, workspacePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for id, d := range s.decisions {
		if d.WorkspacePath == workspacePath && d.IsActive() {
			d.Revoked = true
			d.RevokedAt = &now
			d.Reason = "workspace revoked"
			s.decisions[id] = d
		}
	}
	return nil
}

func (s *UserTrustStore) Snapshot() []UserTrustDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]UserTrustDecision, 0, len(s.decisions))
	for _, d := range s.decisions {
		result = append(result, d)
	}
	return result
}

func (s *UserTrustStore) Restore(decisions []UserTrustDecision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisions = make(map[string]UserTrustDecision, len(decisions))
	for _, d := range decisions {
		s.decisions[d.DecisionID] = d
	}
}
