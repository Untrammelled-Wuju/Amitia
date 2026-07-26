package trust

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type BlockReason string

const (
	BlockReasonMalware        BlockReason = "malware"
	BlockReasonVulnerability  BlockReason = "vulnerability"
	BlockReasonPolicy         BlockReason = "policy_violation"
	BlockReasonUserDecision   BlockReason = "user_decision"
	BlockReasonTakedown       BlockReason = "takedown"
	BlockReasonCompromisedKey BlockReason = "compromised_key"
)

type PackageBlockEntry struct {
	PackageHash string      `json:"package_hash"`
	ExtensionID string      `json:"extension_id,omitempty"`
	Version     string      `json:"version,omitempty"`
	PublisherID string      `json:"publisher_id,omitempty"`
	Reason      BlockReason `json:"reason"`
	Details     string      `json:"details,omitempty"`
	BlockedAt   time.Time   `json:"blocked_at"`
	ExpiresAt   *time.Time  `json:"expires_at,omitempty"`
	Source      string      `json:"source,omitempty"`
}

func (e PackageBlockEntry) IsExpired() bool {
	if e.ExpiresAt == nil {
		return false
	}
	return time.Now().UTC().After(*e.ExpiresAt)
}

func (e PackageBlockEntry) IsActive() bool {
	return !e.IsExpired()
}

type PackageBlocklist struct {
	mu      sync.RWMutex
	entries map[string]PackageBlockEntry
}

func NewPackageBlocklist() *PackageBlocklist {
	return &PackageBlocklist{
		entries: make(map[string]PackageBlockEntry),
	}
}

func (b *PackageBlocklist) Block(entry PackageBlockEntry) error {
	if entry.PackageHash == "" {
		return errors.New("trust: package hash required")
	}
	if entry.Reason == "" {
		return errors.New("trust: block reason required")
	}
	if entry.BlockedAt.IsZero() {
		entry.BlockedAt = time.Now().UTC()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[entry.PackageHash] = entry
	return nil
}

func (b *PackageBlocklist) Unblock(packageHash string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.entries[packageHash]; !ok {
		return fmt.Errorf("trust: package %s not blocked", packageHash)
	}
	delete(b.entries, packageHash)
	return nil
}

func (b *PackageBlocklist) Check(packageHash string) *PackageBlockEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	entry, ok := b.entries[packageHash]
	if !ok || !entry.IsActive() {
		return nil
	}
	e := entry
	return &e
}

func (b *PackageBlocklist) List() []PackageBlockEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]PackageBlockEntry, 0, len(b.entries))
	for _, entry := range b.entries {
		if entry.IsActive() {
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].BlockedAt.Before(result[j].BlockedAt)
	})
	return result
}

func (b *PackageBlocklist) Merge(ctx context.Context, other *PackageBlocklist) int {
	otherEntries := other.List()
	b.mu.Lock()
	defer b.mu.Unlock()
	added := 0
	for _, entry := range otherEntries {
		if _, exists := b.entries[entry.PackageHash]; !exists {
			b.entries[entry.PackageHash] = entry
			added++
		}
	}
	return added
}

func (b *PackageBlocklist) Snapshot() []PackageBlockEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]PackageBlockEntry, 0, len(b.entries))
	for _, entry := range b.entries {
		result = append(result, entry)
	}
	return result
}

func (b *PackageBlocklist) Restore(entries []PackageBlockEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = make(map[string]PackageBlockEntry, len(entries))
	for _, entry := range entries {
		b.entries[entry.PackageHash] = entry
	}
}
