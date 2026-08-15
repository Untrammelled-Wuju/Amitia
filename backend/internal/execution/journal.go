package execution

import (
	"sync"
	"time"
)

type JournalEntryKind string

const (
	JournalEntryExecutionStarted   JournalEntryKind = "execution.started"
	JournalEntryExecutionCompleted JournalEntryKind = "execution.completed"
	JournalEntryExecutionFailed    JournalEntryKind = "execution.failed"
	JournalEntryAcquisitionStarted JournalEntryKind = "acquisition.started"
	JournalEntryAcquisitionReady   JournalEntryKind = "acquisition.ready"
	JournalEntryAcquisitionFailed  JournalEntryKind = "acquisition.failed"
	JournalEntryResumeCreated      JournalEntryKind = "resume.created"
	JournalEntryResumeCompleted    JournalEntryKind = "resume.completed"
	JournalEntryUIRefinement       JournalEntryKind = "ui.refinement"
	JournalEntryToolInvoke         JournalEntryKind = "tool.invoke"
	JournalEntryTaskStateChange    JournalEntryKind = "task.state_change"
	JournalEntryProviderReady      JournalEntryKind = "provider.ready"
	JournalEntryProviderUnavailable JournalEntryKind = "provider.unavailable"
)

type JournalEntry struct {
	EntryID          string          `json:"entryId"`
	RootExecutionID  string          `json:"rootExecutionId"`
	ExecutionID      string          `json:"executionId"`
	Kind             JournalEntryKind `json:"kind"`
	TraceID          string          `json:"traceId,omitempty"`
	InvocationID     string          `json:"invocationId,omitempty"`
	Summary          string          `json:"summary,omitempty"`
	Metadata         map[string]any  `json:"metadata,omitempty"`
	Sequence         int64           `json:"sequence"`
	RecordedAt       time.Time       `json:"recordedAt"`
}

type InMemoryJournal struct {
	mu      sync.RWMutex
	entries []JournalEntry
	seq     int64
}

func NewInMemoryJournal() *InMemoryJournal {
	return &InMemoryJournal{
		entries: make([]JournalEntry, 0),
	}
}

func (j *InMemoryJournal) Record(entry JournalEntry) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.seq++
	entry.Sequence = j.seq
	entry.RecordedAt = time.Now().UTC()
	j.entries = append(j.entries, entry)
}

func (j *InMemoryJournal) EntriesForRoot(rootID string) []JournalEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()
	out := make([]JournalEntry, 0)
	for _, e := range j.entries {
		if e.RootExecutionID == rootID {
			out = append(out, e)
		}
	}
	return out
}

func (j *InMemoryJournal) AllEntries() []JournalEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()
	out := make([]JournalEntry, len(j.entries))
	copy(out, j.entries)
	return out
}
