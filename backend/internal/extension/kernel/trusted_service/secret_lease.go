package trusted_service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

type SecretLease struct {
	LeaseID         string
	SecretName      string
	Purpose         string
	RuntimeInstance string
	ExtensionID     string
	ModuleID        string
	Value           string
	IssuedAt        time.Time
	ExpiresAt       time.Time
	MaxUses         int
	UsedCount       int
	Revoked         bool
	RevokedAt       *time.Time
	RevokedReason   string
}

func (l *SecretLease) IsExpired() bool {
	return time.Now().After(l.ExpiresAt)
}

func (l *SecretLease) IsRevoked() bool {
	return l.Revoked
}

func (l *SecretLease) CanUse() bool {
	if l.Revoked || l.IsExpired() {
		return false
	}
	if l.MaxUses > 0 && l.UsedCount >= l.MaxUses {
		return false
	}
	return true
}

func (l *SecretLease) Use() error {
	if !l.CanUse() {
		return ErrSecretLeaseExpired
	}
	l.UsedCount++
	return nil
}

type SecretLeaseRequest struct {
	SecretName      string
	Purpose         string
	RuntimeInstance string
	ExtensionID     string
	ModuleID        string
	TTL             time.Duration
	MaxUses         int
}

type SecretLeaseAuditEntry struct {
	LeaseID   string
	Action    string
	Timestamp time.Time
	Detail    string
}

type SecretLeaseManager struct {
	mu       sync.RWMutex
	leases   map[string]*SecretLease
	audit    []SecretLeaseAuditEntry
	provider SecretProvider
}

type SecretProvider interface {
	GetSecret(name string) (string, error)
}

func NewSecretLeaseManager(provider SecretProvider) *SecretLeaseManager {
	return &SecretLeaseManager{
		leases:   make(map[string]*SecretLease),
		provider: provider,
	}
}

func (m *SecretLeaseManager) Issue(req SecretLeaseRequest) (*SecretLease, error) {
	if req.SecretName == "" {
		return nil, errors.New("trusted_service: secret name required")
	}
	if req.Purpose == "" {
		return nil, errors.New("trusted_service: secret purpose required")
	}
	if req.RuntimeInstance == "" {
		return nil, errors.New("trusted_service: runtime instance required")
	}

	var value string
	if m.provider != nil {
		v, err := m.provider.GetSecret(req.SecretName)
		if err != nil {
			return nil, fmt.Errorf("trusted_service: get secret: %w", err)
		}
		value = v
	}

	ttl := req.TTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	leaseID := generateLeaseID()
	lease := &SecretLease{
		LeaseID:         leaseID,
		SecretName:      req.SecretName,
		Purpose:         req.Purpose,
		RuntimeInstance: req.RuntimeInstance,
		ExtensionID:     req.ExtensionID,
		ModuleID:        req.ModuleID,
		Value:           value,
		IssuedAt:        time.Now().UTC(),
		ExpiresAt:       time.Now().UTC().Add(ttl),
		MaxUses:         req.MaxUses,
	}

	m.mu.Lock()
	m.leases[leaseID] = lease
	m.audit = append(m.audit, SecretLeaseAuditEntry{
		LeaseID:   leaseID,
		Action:    "issue",
		Timestamp: time.Now().UTC(),
		Detail:    fmt.Sprintf("secret=%s purpose=%s instance=%s", req.SecretName, req.Purpose, req.RuntimeInstance),
	})
	m.mu.Unlock()

	return lease, nil
}

func (m *SecretLeaseManager) Consume(leaseID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	lease, exists := m.leases[leaseID]
	if !exists {
		return "", ErrSecretLeaseNotFound
	}
	if !lease.CanUse() {
		m.audit = append(m.audit, SecretLeaseAuditEntry{
			LeaseID:   leaseID,
			Action:    "consume_denied",
			Timestamp: time.Now().UTC(),
			Detail:    fmt.Sprintf("revoked=%v expired=%v uses=%d/%d", lease.Revoked, lease.IsExpired(), lease.UsedCount, lease.MaxUses),
		})
		return "", ErrSecretLeaseExpired
	}
	lease.UsedCount++
	m.audit = append(m.audit, SecretLeaseAuditEntry{
		LeaseID:   leaseID,
		Action:    "consume",
		Timestamp: time.Now().UTC(),
	})
	return lease.Value, nil
}

func (m *SecretLeaseManager) Revoke(leaseID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	lease, exists := m.leases[leaseID]
	if !exists {
		return ErrSecretLeaseNotFound
	}
	lease.Revoked = true
	now := time.Now().UTC()
	lease.RevokedAt = &now
	lease.RevokedReason = reason
	m.audit = append(m.audit, SecretLeaseAuditEntry{
		LeaseID:   leaseID,
		Action:    "revoke",
		Timestamp: now,
		Detail:    reason,
	})
	return nil
}

func (m *SecretLeaseManager) RevokeByInstance(instanceID, reason string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, lease := range m.leases {
		if lease.RuntimeInstance == instanceID && !lease.Revoked {
			lease.Revoked = true
			now := time.Now().UTC()
			lease.RevokedAt = &now
			lease.RevokedReason = reason
			count++
			m.audit = append(m.audit, SecretLeaseAuditEntry{
				LeaseID:   lease.LeaseID,
				Action:    "revoke_by_instance",
				Timestamp: now,
				Detail:    fmt.Sprintf("instance=%s reason=%s", instanceID, reason),
			})
		}
	}
	return count
}

func (m *SecretLeaseManager) Get(leaseID string) (*SecretLease, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	lease, exists := m.leases[leaseID]
	if !exists {
		return nil, ErrSecretLeaseNotFound
	}
	return lease, nil
}

func (m *SecretLeaseManager) ListByInstance(instanceID string) []*SecretLease {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*SecretLease, 0)
	for _, lease := range m.leases {
		if lease.RuntimeInstance == instanceID {
			out = append(out, lease)
		}
	}
	return out
}

func (m *SecretLeaseManager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for id, lease := range m.leases {
		if lease.Revoked || lease.IsExpired() {
			delete(m.leases, id)
			count++
		}
	}
	return count
}

func (m *SecretLeaseManager) AuditEntries() []SecretLeaseAuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]SecretLeaseAuditEntry, len(m.audit))
	copy(out, m.audit)
	return out
}

func generateLeaseID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "lease-" + hex.EncodeToString(b)
}

var (
	ErrSecretLeaseNotFound = errors.New("trusted_service: secret lease not found")
	ErrSecretLeaseExpired  = errors.New("trusted_service: secret lease expired or revoked")
)
