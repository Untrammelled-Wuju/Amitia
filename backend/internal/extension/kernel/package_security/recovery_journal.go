package package_security

import (
	"context"
	"sync"
	"time"
)

type RecoveryJournalEntry struct {
	OperationID string    `json:"operation_id"`
	PackageID   string    `json:"package_id"`
	Version     string    `json:"version"`
	Step        string    `json:"step"`
	State       string    `json:"state"`
	StagingID   string    `json:"staging_id"`
	SnapshotID  string    `json:"snapshot_id"`
	TargetPath  string    `json:"target_path"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RecoveryJournal struct {
	mu      sync.RWMutex
	entries map[string][]RecoveryJournalEntry
}

func NewRecoveryJournal() *RecoveryJournal {
	return &RecoveryJournal{
		entries: make(map[string][]RecoveryJournalEntry),
	}
}

func (j *RecoveryJournal) Record(ctx context.Context, entry RecoveryJournalEntry) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = time.Now()
	}

	j.entries[entry.OperationID] = append(j.entries[entry.OperationID], entry)
}

func (j *RecoveryJournal) GetEntries(ctx context.Context, operationID string) []RecoveryJournalEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return j.entries[operationID]
}

func (j *RecoveryJournal) ListPending(ctx context.Context) []RecoveryJournalEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()

	var pending []RecoveryJournalEntry
	for _, entries := range j.entries {
		if len(entries) == 0 {
			continue
		}
		last := entries[len(entries)-1]
		if last.State == "in_progress" || last.State == "pending" {
			pending = append(pending, last)
		}
	}
	return pending
}

func (j *RecoveryJournal) DeleteOperation(ctx context.Context, operationID string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	delete(j.entries, operationID)
}

func (j *RecoveryJournal) ListAll(ctx context.Context) map[string][]RecoveryJournalEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()

	result := make(map[string][]RecoveryJournalEntry)
	for k, v := range j.entries {
		result[k] = v
	}
	return result
}
