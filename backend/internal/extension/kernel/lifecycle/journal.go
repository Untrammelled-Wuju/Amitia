package lifecycle

import (
	"context"
	"sort"
	"sync"
	"time"
)

type StartupJournalEntry struct {
	StartupID    string
	ComponentID  string
	Phase        StartupPhase
	Status       StartupStatus
	Attempt      int
	StartedAt    time.Time
	FinishedAt   *time.Time
	ErrorCode    string
	Error        string
	Metadata     map[string]any
}

type ShutdownJournalEntry struct {
	ShutdownID    string
	ComponentID   string
	Phase         ShutdownPhase
	Status        ShutdownStatus
	StartedAt     time.Time
	FinishedAt    *time.Time
	ErrorCode     string
	Error         string
	RecoveryHint  string
	Metadata      map[string]any
}

type Journal struct {
	mu                sync.Mutex
	startupEntries    []StartupJournalEntry
	shutdownEntries   []ShutdownJournalEntry
	cleanShutdownFlag bool
	lastShutdownID    string
	lastStartupID     string
}

func NewJournal() *Journal {
	return &Journal{}
}

func (j *Journal) RecordStartup(entry StartupJournalEntry) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.startupEntries = append(j.startupEntries, entry)
	if entry.StartupID != "" {
		j.lastStartupID = entry.StartupID
	}
}

func (j *Journal) RecordShutdown(entry ShutdownJournalEntry) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.shutdownEntries = append(j.shutdownEntries, entry)
	if entry.ShutdownID != "" {
		j.lastShutdownID = entry.ShutdownID
	}
}

func (j *Journal) MarkCleanShutdown(shutdownID string, clean bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.cleanShutdownFlag = clean
	if shutdownID != "" {
		j.lastShutdownID = shutdownID
	}
}

func (j *Journal) IsCleanShutdown() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.cleanShutdownFlag
}

func (j *Journal) LastShutdownID() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.lastShutdownID
}

func (j *Journal) LastStartupID() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.lastStartupID
}

func (j *Journal) StartupEntries() []StartupJournalEntry {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := append([]StartupJournalEntry{}, j.startupEntries...)
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out
}

func (j *Journal) ShutdownEntries() []ShutdownJournalEntry {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := append([]ShutdownJournalEntry{}, j.shutdownEntries...)
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out
}

func (j *Journal) InterruptedComponents() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	set := make(map[string]struct{})
	for _, e := range j.startupEntries {
		if e.Status == StartupStatusFailed || e.Status == StartupStatusStarting {
			set[e.ComponentID] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type JournalStore interface {
	PersistStartup(ctx context.Context, entry StartupJournalEntry) error
	PersistShutdown(ctx context.Context, entry ShutdownJournalEntry) error
	LoadLastState(ctx context.Context) (cleanShutdown bool, lastShutdownID string, err error)
}

type InMemoryJournalStore struct {
	mu                sync.Mutex
	startup           []StartupJournalEntry
	shutdown          []ShutdownJournalEntry
	cleanShutdown     bool
	lastShutdownID    string
}

func NewInMemoryJournalStore() *InMemoryJournalStore {
	return &InMemoryJournalStore{}
}

func (s *InMemoryJournalStore) PersistStartup(_ context.Context, entry StartupJournalEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startup = append(s.startup, entry)
	return nil
}

func (s *InMemoryJournalStore) PersistShutdown(_ context.Context, entry ShutdownJournalEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shutdown = append(s.shutdown, entry)
	return nil
}

func (s *InMemoryJournalStore) LoadLastState(_ context.Context) (bool, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cleanShutdown, s.lastShutdownID, nil
}

func (s *InMemoryJournalStore) SetCleanShutdown(clean bool, shutdownID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanShutdown = clean
	s.lastShutdownID = shutdownID
}

var _ JournalStore = (*InMemoryJournalStore)(nil)

var journalTimeNow = func() time.Time { return time.Now().UTC() }
