package trust

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type RevocationSource string

const (
	RevocationSourceOfficial   RevocationSource = "official"
	RevocationSourceLocal      RevocationSource = "local"
	RevocationSourceUser       RevocationSource = "user"
	RevocationSourceCompromised RevocationSource = "compromised"
)

type RevocationSeverity string

const (
	RevocationSeverityLow      RevocationSeverity = "low"
	RevocationSeverityMedium   RevocationSeverity = "medium"
	RevocationSeverityHigh     RevocationSeverity = "high"
	RevocationSeverityCritical RevocationSeverity = "critical"
)

type RevocationEntry struct {
	EntryID         string             `json:"entry_id"`
	PublisherID     string             `json:"publisher_id,omitempty"`
	KeyID           string             `json:"key_id,omitempty"`
	PackageHash     string             `json:"package_hash,omitempty"`
	ExtensionID     string             `json:"extension_id,omitempty"`
	Version         string             `json:"version,omitempty"`
	Source          RevocationSource   `json:"source"`
	Severity        RevocationSeverity `json:"severity"`
	Reason          string             `json:"reason"`
	RevokedAt       time.Time          `json:"revoked_at"`
	ExpiresAt       *time.Time         `json:"expires_at,omitempty"`
	Superseded      bool               `json:"superseded"`
	SupersededBy    string             `json:"superseded_by,omitempty"`
}

func (e RevocationEntry) IsExpired() bool {
	if e.ExpiresAt == nil {
		return false
	}
	return time.Now().UTC().After(*e.ExpiresAt)
}

func (e RevocationEntry) IsActive() bool {
	return !e.Superseded && !e.IsExpired()
}

func (e RevocationEntry) AffectsKey() bool {
	return e.KeyID != ""
}

func (e RevocationEntry) AffectsPackage() bool {
	return e.PackageHash != ""
}

func (e RevocationEntry) AffectsPublisher() bool {
	return e.KeyID == "" && e.PackageHash == "" && e.PublisherID != ""
}

type RevocationList struct {
	mu      sync.RWMutex
	entries map[string]RevocationEntry
	lastSync time.Time
	source   string
}

func NewRevocationList(source string) *RevocationList {
	return &RevocationList{
		entries: make(map[string]RevocationEntry),
		source:  source,
	}
}

func (l *RevocationList) Add(entry RevocationEntry) error {
	if entry.EntryID == "" {
		return errors.New("trust: entry id required")
	}
	if entry.Source == "" {
		return errors.New("trust: source required")
	}
	if entry.Reason == "" {
		return errors.New("trust: reason required")
	}
	if entry.RevokedAt.IsZero() {
		entry.RevokedAt = time.Now().UTC()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if existing, ok := l.entries[entry.EntryID]; ok && existing.Superseded {
		return fmt.Errorf("trust: entry %s already superseded", entry.EntryID)
	}
	l.entries[entry.EntryID] = entry
	return nil
}

func (l *RevocationList) Remove(entryID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.entries[entryID]; !ok {
		return fmt.Errorf("trust: entry %s not found", entryID)
	}
	delete(l.entries, entryID)
	return nil
}

func (l *RevocationList) Supersede(oldID, newID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	old, ok := l.entries[oldID]
	if !ok {
		return fmt.Errorf("trust: entry %s not found", oldID)
	}
	old.Superseded = true
	old.SupersededBy = newID
	l.entries[oldID] = old
	return nil
}

func (l *RevocationList) CheckKey(publisherID, keyID string) *RevocationEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, entry := range l.entries {
		if !entry.IsActive() {
			continue
		}
		if entry.AffectsKey() && entry.KeyID == keyID {
			if entry.PublisherID == "" || entry.PublisherID == publisherID {
				e := entry
				return &e
			}
		}
	}
	return nil
}

func (l *RevocationList) CheckPackage(packageHash string) *RevocationEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, entry := range l.entries {
		if !entry.IsActive() {
			continue
		}
		if entry.AffectsPackage() && entry.PackageHash == packageHash {
			e := entry
			return &e
		}
	}
	return nil
}

func (l *RevocationList) CheckPublisher(publisherID string) *RevocationEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, entry := range l.entries {
		if !entry.IsActive() {
			continue
		}
		if entry.AffectsPublisher() && entry.PublisherID == publisherID {
			e := entry
			return &e
		}
	}
	return nil
}

func (l *RevocationList) CheckExtension(extensionID, version string) *RevocationEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, entry := range l.entries {
		if !entry.IsActive() {
			continue
		}
		if entry.ExtensionID == extensionID {
			if entry.Version == "" || entry.Version == version {
				e := entry
				return &e
			}
		}
	}
	return nil
}

func (l *RevocationList) List() []RevocationEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]RevocationEntry, 0, len(l.entries))
	for _, entry := range l.entries {
		if entry.IsActive() {
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].RevokedAt.Before(result[j].RevokedAt)
	})
	return result
}

func (l *RevocationList) LastSync() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastSync
}

func (l *RevocationList) MarkSynced(at time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastSync = at
}

func (l *RevocationList) Merge(ctx context.Context, other *RevocationList) int {
	otherEntries := other.List()
	l.mu.Lock()
	defer l.mu.Unlock()
	added := 0
	for _, entry := range otherEntries {
		if _, exists := l.entries[entry.EntryID]; !exists {
			l.entries[entry.EntryID] = entry
			added++
		}
	}
	l.lastSync = time.Now().UTC()
	return added
}

func (l *RevocationList) Snapshot() []RevocationEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]RevocationEntry, 0, len(l.entries))
	for _, entry := range l.entries {
		result = append(result, entry)
	}
	return result
}

func (l *RevocationList) Restore(entries []RevocationEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make(map[string]RevocationEntry, len(entries))
	for _, entry := range entries {
		l.entries[entry.EntryID] = entry
	}
}
