package kernel

import (
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type CandidateNamespaceEntry struct {
	CandidateID    string
	ExtensionID    domain.ExtensionID
	InstanceIDs    []string
	GenerationID   string
	Generation     int64
	Keys           []CandidateKey
	Contribs       []domain.ContributionDefinition
	DefinitionHash string
	ArtifactPath   string
	RegisteredAt   time.Time
	ValidatedAt    *time.Time
}

type CandidateNamespace struct {
	mu      sync.RWMutex
	entries map[string]*CandidateNamespaceEntry
}

func NewCandidateNamespace() *CandidateNamespace {
	return &CandidateNamespace{
		entries: make(map[string]*CandidateNamespaceEntry),
	}
}

func (ns *CandidateNamespace) Store(entry *CandidateNamespaceEntry) error {
	if ns == nil {
		return fmt.Errorf("candidate-namespace: not initialized")
	}
	if entry == nil || entry.CandidateID == "" {
		return fmt.Errorf("candidate-namespace: entry with candidateID is required")
	}
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if _, exists := ns.entries[entry.CandidateID]; exists {
		return fmt.Errorf("candidate-namespace: candidate %s already exists", entry.CandidateID)
	}
	if entry.RegisteredAt.IsZero() {
		entry.RegisteredAt = time.Now().UTC()
	}
	ns.entries[entry.CandidateID] = entry
	return nil
}

func (ns *CandidateNamespace) Load(candidateID string) (*CandidateNamespaceEntry, bool) {
	if ns == nil {
		return nil, false
	}
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	entry, ok := ns.entries[candidateID]
	if !ok {
		return nil, false
	}
	copied := *entry
	return &copied, true
}

func (ns *CandidateNamespace) Remove(candidateID string) {
	if ns == nil {
		return
	}
	ns.mu.Lock()
	defer ns.mu.Unlock()
	delete(ns.entries, candidateID)
}

func (ns *CandidateNamespace) HasCandidate(candidateID string) bool {
	if ns == nil {
		return false
	}
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	_, ok := ns.entries[candidateID]
	return ok
}

func (ns *CandidateNamespace) MarkValidated(candidateID string) error {
	if ns == nil {
		return fmt.Errorf("candidate-namespace: not initialized")
	}
	ns.mu.Lock()
	defer ns.mu.Unlock()
	entry, ok := ns.entries[candidateID]
	if !ok {
		return fmt.Errorf("candidate-namespace: candidate %s not found", candidateID)
	}
	now := time.Now().UTC()
	entry.ValidatedAt = &now
	return nil
}

func (ns *CandidateNamespace) IsValidated(candidateID string) bool {
	if ns == nil {
		return false
	}
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	entry, ok := ns.entries[candidateID]
	if !ok {
		return false
	}
	return entry.ValidatedAt != nil
}

func (ns *CandidateNamespace) ListAll() []*CandidateNamespaceEntry {
	if ns == nil {
		return nil
	}
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	out := make([]*CandidateNamespaceEntry, 0, len(ns.entries))
	for _, entry := range ns.entries {
		copied := *entry
		out = append(out, &copied)
	}
	return out
}

func (ns *CandidateNamespace) ListByExtension(extID domain.ExtensionID) []*CandidateNamespaceEntry {
	if ns == nil {
		return nil
	}
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	var out []*CandidateNamespaceEntry
	for _, entry := range ns.entries {
		if entry.ExtensionID == extID {
			copied := *entry
			out = append(out, &copied)
		}
	}
	return out
}

func (ns *CandidateNamespace) Count() int {
	if ns == nil {
		return 0
	}
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return len(ns.entries)
}

func (ns *CandidateNamespace) Clear() {
	if ns == nil {
		return
	}
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.entries = make(map[string]*CandidateNamespaceEntry)
}
