package desktop_update

import (
	"sort"
	"sync"
	"time"
)

type JournalEntry struct {
	OperationID  string
	Step         string
	Status       string
	StartedAt    time.Time
	FinishedAt   *time.Time
	InputHash    string
	OutputHash   string
	Error        string
	Compensation string
}

const (
	JournalStatusPending    = "pending"
	JournalStatusInProgress = "in_progress"
	JournalStatusCompleted  = "completed"
	JournalStatusFailed     = "failed"
	JournalStatusSkipped    = "skipped"
)

type UpdateJournal struct {
	mu      sync.RWMutex
	entries map[string][]JournalEntry
}

func NewUpdateJournal() *UpdateJournal {
	return &UpdateJournal{
		entries: make(map[string][]JournalEntry),
	}
}

func (j *UpdateJournal) Record(entry JournalEntry) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if entry.StartedAt.IsZero() {
		entry.StartedAt = time.Now().UTC()
	}
	j.entries[entry.OperationID] = append(j.entries[entry.OperationID], entry)
}

func (j *UpdateJournal) RecordStep(operationID, step, status string, startedAt time.Time, err string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	entry := JournalEntry{
		OperationID: operationID,
		Step:        step,
		Status:      status,
		StartedAt:   startedAt,
	}
	if startedAt.IsZero() {
		entry.StartedAt = time.Now().UTC()
	}
	if status == JournalStatusCompleted || status == JournalStatusFailed || status == JournalStatusSkipped {
		now := time.Now().UTC()
		entry.FinishedAt = &now
	}
	if err != "" {
		entry.Error = err
	}
	j.entries[operationID] = append(j.entries[operationID], entry)
}

func (j *UpdateJournal) CompleteStep(operationID, step string, outputHash string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	entries := j.entries[operationID]
	now := time.Now().UTC()
	for i := range entries {
		if entries[i].Step == step && entries[i].Status == JournalStatusInProgress {
			entries[i].Status = JournalStatusCompleted
			entries[i].FinishedAt = &now
			entries[i].OutputHash = outputHash
			return
		}
	}
}

func (j *UpdateJournal) FailStep(operationID, step, errMsg, compensation string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	entries := j.entries[operationID]
	now := time.Now().UTC()
	for i := range entries {
		if entries[i].Step == step && entries[i].Status == JournalStatusInProgress {
			entries[i].Status = JournalStatusFailed
			entries[i].FinishedAt = &now
			entries[i].Error = errMsg
			entries[i].Compensation = compensation
			return
		}
	}
}

func (j *UpdateJournal) GetEntries(operationID string) []JournalEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()
	entries := j.entries[operationID]
	out := make([]JournalEntry, len(entries))
	copy(out, entries)
	return out
}

func (j *UpdateJournal) GetLastEntry(operationID string) *JournalEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()
	entries := j.entries[operationID]
	if len(entries) == 0 {
		return nil
	}
	e := entries[len(entries)-1]
	return &e
}

func (j *UpdateJournal) ListPending() []JournalEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()
	var out []JournalEntry
	for _, entries := range j.entries {
		for _, e := range entries {
			if e.Status == JournalStatusPending || e.Status == JournalStatusInProgress {
				out = append(out, e)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out
}

func (j *UpdateJournal) ListAll() []JournalEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()
	var out []JournalEntry
	for _, entries := range j.entries {
		out = append(out, entries...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OperationID != out[j].OperationID {
			return out[i].OperationID < out[j].OperationID
		}
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out
}

func (j *UpdateJournal) ListOperations() []string {
	j.mu.RLock()
	defer j.mu.RUnlock()
	out := make([]string, 0, len(j.entries))
	for opID := range j.entries {
		out = append(out, opID)
	}
	sort.Strings(out)
	return out
}

func (j *UpdateJournal) Clear(operationID string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.entries, operationID)
}

func (j *UpdateJournal) Restore(entries []JournalEntry) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.entries = make(map[string][]JournalEntry)
	for _, entry := range entries {
		j.entries[entry.OperationID] = append(j.entries[entry.OperationID], entry)
	}
}
