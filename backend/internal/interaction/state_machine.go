package interaction

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type InteractionStatus string

const (
	InteractionStatusPending    InteractionStatus = "pending"
	InteractionStatusProcessing InteractionStatus = "processing"
	InteractionStatusCompleted  InteractionStatus = "completed"
	InteractionStatusSuperseded InteractionStatus = "superseded"
	InteractionStatusCancelled  InteractionStatus = "cancelled"
	InteractionStatusFailed     InteractionStatus = "failed"
)

var validTransitions = map[InteractionStatus][]InteractionStatus{
	InteractionStatusPending:    {InteractionStatusProcessing, InteractionStatusCancelled, InteractionStatusSuperseded},
	InteractionStatusProcessing: {InteractionStatusCompleted, InteractionStatusFailed, InteractionStatusSuperseded, InteractionStatusCancelled},
	InteractionStatusCompleted:  {InteractionStatusSuperseded},
	InteractionStatusFailed:     {InteractionStatusPending},
	InteractionStatusSuperseded: {},
	InteractionStatusCancelled:  {InteractionStatusPending},
}

var (
	ErrInvalidTransition = errors.New("interaction: invalid state transition")
	ErrAlreadyTerminal   = errors.New("interaction: already in terminal state")
)

type InteractionRecord struct {
	ID              string            `json:"id"`
	Scope           InteractionScope  `json:"scope"`
	Status          InteractionStatus `json:"status"`
	SupersedesID    string            `json:"supersedesId,omitempty"`
	SupersededByID  string            `json:"supersededById,omitempty"`
	CancelReason    string            `json:"cancelReason,omitempty"`
	ErrorMessage    string            `json:"errorMessage,omitempty"`
	ResultRef       string            `json:"resultRef,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	StartedAt       time.Time         `json:"startedAt,omitempty"`
	CompletedAt     time.Time         `json:"completedAt,omitempty"`
	mu              sync.RWMutex      `json:"-"`
	cancel          context.CancelFunc `json:"-"`
}

func NewInteractionRecord(scope InteractionScope) *InteractionRecord {
	return &InteractionRecord{
		ID:        uuid.New().String(),
		Scope:     scope.Normalize(),
		Status:    InteractionStatusPending,
		CreatedAt: time.Now(),
	}
}

func (r *InteractionRecord) IsTerminal() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Status == InteractionStatusCompleted ||
		r.Status == InteractionStatusSuperseded ||
		r.Status == InteractionStatusCancelled ||
		r.Status == InteractionStatusFailed
}

func (r *InteractionRecord) IsActive() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Status == InteractionStatusPending || r.Status == InteractionStatusProcessing
}

func (r *InteractionRecord) Transition(target InteractionStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.transitionLocked(target)
}

func (r *InteractionRecord) transitionLocked(target InteractionStatus) error {
	if r.Status == target {
		return nil
	}
	allowed, ok := validTransitions[r.Status]
	if !ok {
		return ErrInvalidTransition
	}
	found := false
	for _, a := range allowed {
		if a == target {
			found = true
			break
		}
	}
	if !found {
		return ErrInvalidTransition
	}
	if r.IsTerminal() && target != InteractionStatusPending {
		return ErrAlreadyTerminal
	}
	r.Status = target
	switch target {
	case InteractionStatusProcessing:
		r.StartedAt = time.Now()
	case InteractionStatusCompleted, InteractionStatusFailed, InteractionStatusCancelled, InteractionStatusSuperseded:
		r.CompletedAt = time.Now()
	}
	return nil
}

func (r *InteractionRecord) SetCancel(cancel context.CancelFunc) {
	r.mu.Lock()
	r.cancel = cancel
	r.mu.Unlock()
}

func (r *InteractionRecord) Cancel() bool {
	r.mu.Lock()
	cancelFn := r.cancel
	r.mu.Unlock()
	if cancelFn != nil {
		cancelFn()
		return true
	}
	return false
}

func (r *InteractionRecord) SetError(errMsg string) {
	r.mu.Lock()
	r.ErrorMessage = errMsg
	r.mu.Unlock()
}

func (r *InteractionRecord) SetSupersededBy(supersederID string) {
	r.mu.Lock()
	r.SupersededByID = supersederID
	r.mu.Unlock()
}

func (r *InteractionRecord) Snapshot() InteractionRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return InteractionRecord{
		ID:             r.ID,
		Scope:          r.Scope,
		Status:         r.Status,
		SupersedesID:   r.SupersedesID,
		SupersededByID: r.SupersededByID,
		CancelReason:   r.CancelReason,
		ErrorMessage:   r.ErrorMessage,
		ResultRef:      r.ResultRef,
		CreatedAt:      r.CreatedAt,
		StartedAt:      r.StartedAt,
		CompletedAt:    r.CompletedAt,
	}
}

type InteractionTracker interface {
	Track(record *InteractionRecord)
	Get(id string) (*InteractionRecord, bool)
	GetActiveByScope(scope InteractionScope) []*InteractionRecord
	GetByScope(scope InteractionScope) []*InteractionRecord
	Remove(id string)
	Range(fn func(record *InteractionRecord) bool)
}

type InMemoryTracker struct {
	mu       sync.RWMutex
	records  map[string]*InteractionRecord
	byConv   map[string]map[string]struct{}
	byChar   map[string]map[string]struct{}
}

func NewInMemoryTracker() *InMemoryTracker {
	return &InMemoryTracker{
		records: make(map[string]*InteractionRecord),
		byConv:  make(map[string]map[string]struct{}),
		byChar:  make(map[string]map[string]struct{}),
	}
}

func (t *InMemoryTracker) Track(record *InteractionRecord) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records[record.ID] = record
	if record.Scope.ConversationID != "" {
		if t.byConv[record.Scope.ConversationID] == nil {
			t.byConv[record.Scope.ConversationID] = make(map[string]struct{})
		}
		t.byConv[record.Scope.ConversationID][record.ID] = struct{}{}
	}
	if record.Scope.CharacterID != "" {
		if t.byChar[record.Scope.CharacterID] == nil {
			t.byChar[record.Scope.CharacterID] = make(map[string]struct{})
		}
		t.byChar[record.Scope.CharacterID][record.ID] = struct{}{}
	}
}

func (t *InMemoryTracker) Get(id string) (*InteractionRecord, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	rec, ok := t.records[id]
	return rec, ok
}

func (t *InMemoryTracker) GetActiveByScope(scope InteractionScope) []*InteractionRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()
	scope = scope.Normalize()
	var result []*InteractionRecord
	candidates := t.scopeCandidatesLocked(scope)
	for _, id := range candidates {
		rec, ok := t.records[id]
		if !ok {
			continue
		}
		if rec.IsActive() && rec.Scope.ConversationID == scope.ConversationID {
			result = append(result, rec)
		}
	}
	return result
}

func (t *InMemoryTracker) GetByScope(scope InteractionScope) []*InteractionRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()
	scope = scope.Normalize()
	var result []*InteractionRecord
	candidates := t.scopeCandidatesLocked(scope)
	for _, id := range candidates {
		rec, ok := t.records[id]
		if !ok {
			continue
		}
		if rec.Scope.ConversationID == scope.ConversationID {
			result = append(result, rec)
		}
	}
	return result
}

func (t *InMemoryTracker) Remove(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	rec, ok := t.records[id]
	if !ok {
		return
	}
	delete(t.byConv[rec.Scope.ConversationID], id)
	delete(t.byChar[rec.Scope.CharacterID], id)
	delete(t.records, id)
}

func (t *InMemoryTracker) Range(fn func(record *InteractionRecord) bool) {
	t.mu.RLock()
	snaps := make([]*InteractionRecord, 0, len(t.records))
	for _, rec := range t.records {
		snap := rec.Snapshot()
		snaps = append(snaps, &snap)
	}
	t.mu.RUnlock()
	for _, snap := range snaps {
		if !fn(snap) {
			break
		}
	}
}

func (t *InMemoryTracker) scopeCandidatesLocked(scope InteractionScope) []string {
	var ids []string
	if scope.ConversationID != "" {
		for id := range t.byConv[scope.ConversationID] {
			ids = append(ids, id)
		}
		return ids
	}
	if scope.CharacterID != "" {
		for id := range t.byChar[scope.CharacterID] {
			ids = append(ids, id)
		}
		return ids
	}
	for id := range t.records {
		ids = append(ids, id)
	}
	return ids
}

var _ InteractionTracker = (*InMemoryTracker)(nil)
