package scope

import (
	"fmt"
	"time"
)

func NewBinding(subjectType ScopeSubjectType, subjectID string, scope ScopeRef, source ScopeBindingSource) (*ScopeBinding, error) {
	if err := scope.Validate(); err != nil {
		return nil, fmt.Errorf("invalid scope: %w", err)
	}
	if subjectType == "" {
		return nil, fmt.Errorf("subject type is required")
	}
	if subjectID == "" {
		return nil, fmt.Errorf("subject ID is required")
	}
	now := time.Now()
	return &ScopeBinding{
		BindingID:   generateBindingID(),
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Scope:       scope,
		State:       StateActive,
		Source:      source,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (b *ScopeBinding) Revoke() {
	b.State = StateRevoked
	b.UpdatedAt = time.Now()
}

func (b *ScopeBinding) Deactivate() {
	b.State = StateInactive
	b.UpdatedAt = time.Now()
}

func (b *ScopeBinding) Activate() {
	b.State = StateActive
	b.UpdatedAt = time.Now()
}

func (b *ScopeBinding) IsActive() bool {
	if b.State != StateActive {
		return false
	}
	if b.ExpiresAt != nil && time.Now().After(*b.ExpiresAt) {
		return false
	}
	return true
}

func (b *ScopeBinding) SetExpiry(expiresAt time.Time) {
	b.ExpiresAt = &expiresAt
	b.UpdatedAt = time.Now()
}

var bindingIDCounter uint64

func generateBindingID() string {
	bindingIDCounter++
	return fmt.Sprintf("sb-%d-%d", time.Now().UnixNano(), bindingIDCounter)
}
