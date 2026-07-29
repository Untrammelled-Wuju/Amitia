package trusted_service

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type QuarantineReason string

const (
	QuarantineSignatureFailure      QuarantineReason = "signature_failure"
	QuarantineBinaryHashChanged     QuarantineReason = "binary_hash_changed"
	QuarantinePublisherRevoked      QuarantineReason = "publisher_revoked"
	QuarantineProcessTreeUnkillable QuarantineReason = "process_tree_unkillable"
	QuarantineUndeclaredChild       QuarantineReason = "undeclared_child_process"
	QuarantineUndeclaredPort        QuarantineReason = "undeclared_public_port"
	QuarantineProtocolMismatch      QuarantineReason = "protocol_identity_mismatch"
	QuarantineHostAPIViolation      QuarantineReason = "host_api_violation"
	QuarantineFrequentCrash         QuarantineReason = "frequent_crash"
	QuarantineResourceExceeded      QuarantineReason = "resource_limit_exceeded"
	QuarantinePackageTampered       QuarantineReason = "package_tampered"
)

type QuarantineRecord struct {
	ServiceID     string
	InstanceID    string
	Reason        QuarantineReason
	Detail        string
	Evidence      map[string]any
	QuarantinedAt time.Time
	ReleasedAt    *time.Time
	ReleasedBy    string
	ReleaseReason string
}

func (r *QuarantineRecord) IsActive() bool {
	return r.ReleasedAt == nil
}

type QuarantineManager struct {
	mu      sync.RWMutex
	records map[string]*QuarantineRecord
	history []*QuarantineRecord
}

func NewQuarantineManager() *QuarantineManager {
	return &QuarantineManager{
		records: make(map[string]*QuarantineRecord),
	}
}

func (m *QuarantineManager) Quarantine(serviceID, instanceID string, reason QuarantineReason, detail string, evidence map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, exists := m.records[serviceID]; exists && existing.IsActive() {
		return fmt.Errorf("%w: %s already quarantined", ErrAlreadyQuarantined, serviceID)
	}

	record := &QuarantineRecord{
		ServiceID:     serviceID,
		InstanceID:    instanceID,
		Reason:        reason,
		Detail:        detail,
		Evidence:      evidence,
		QuarantinedAt: time.Now().UTC(),
	}
	m.records[serviceID] = record
	m.history = append(m.history, record)
	return nil
}

func (m *QuarantineManager) Release(serviceID, releasedBy, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, exists := m.records[serviceID]
	if !exists {
		return ErrNotQuarantined
	}
	if !record.IsActive() {
		return ErrNotQuarantined
	}

	now := time.Now().UTC()
	record.ReleasedAt = &now
	record.ReleasedBy = releasedBy
	record.ReleaseReason = reason
	delete(m.records, serviceID)
	return nil
}

func (m *QuarantineManager) IsQuarantined(serviceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, exists := m.records[serviceID]
	return exists && record.IsActive()
}

func (m *QuarantineManager) Get(serviceID string) (*QuarantineRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, exists := m.records[serviceID]
	if !exists || !record.IsActive() {
		return nil, ErrNotQuarantined
	}
	return record, nil
}

func (m *QuarantineManager) ListActive() []*QuarantineRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*QuarantineRecord, 0, len(m.records))
	for _, record := range m.records {
		if record.IsActive() {
			out = append(out, record)
		}
	}
	return out
}

func (m *QuarantineManager) History() []*QuarantineRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*QuarantineRecord, len(m.history))
	copy(out, m.history)
	return out
}

var (
	ErrAlreadyQuarantined = errors.New("trusted_service: service already quarantined")
	ErrNotQuarantined     = errors.New("trusted_service: service not quarantined")
)
