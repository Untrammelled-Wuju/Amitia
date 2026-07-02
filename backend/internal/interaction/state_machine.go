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
	InteractionStatusReceived        InteractionStatus = "received"
	InteractionStatusNormalized      InteractionStatus = "normalized"
	InteractionStatusQueued          InteractionStatus = "queued"
	InteractionStatusProcessing      InteractionStatus = "processing"
	InteractionStatusContextReady    InteractionStatus = "context_ready"
	InteractionStatusDecided         InteractionStatus = "decided"
	InteractionStatusGenerated       InteractionStatus = "generated"
	InteractionStatusCommitted       InteractionStatus = "committed"
	InteractionStatusDeliveryPending InteractionStatus = "delivery_pending"
	InteractionStatusDelivered       InteractionStatus = "delivered"
	InteractionStatusCompleted       InteractionStatus = "completed"
	InteractionStatusSuperseded      InteractionStatus = "superseded"
	InteractionStatusCancelled       InteractionStatus = "cancelled"
	InteractionStatusFailed          InteractionStatus = "failed"
	InteractionStatusInterrupted     InteractionStatus = "interrupted"
	InteractionStatusArchived        InteractionStatus = "archived"
	InteractionStatusPending         InteractionStatus = InteractionStatusReceived
)

var validTransitions = map[InteractionStatus][]InteractionStatus{
	InteractionStatusReceived:        {InteractionStatusNormalized, InteractionStatusQueued, InteractionStatusProcessing, InteractionStatusCancelled, InteractionStatusSuperseded, InteractionStatusFailed},
	InteractionStatusNormalized:      {InteractionStatusQueued, InteractionStatusProcessing, InteractionStatusCancelled, InteractionStatusSuperseded, InteractionStatusFailed},
	InteractionStatusQueued:          {InteractionStatusProcessing, InteractionStatusCancelled, InteractionStatusSuperseded, InteractionStatusFailed},
	InteractionStatusProcessing:      {InteractionStatusContextReady, InteractionStatusGenerated, InteractionStatusCommitted, InteractionStatusCompleted, InteractionStatusFailed, InteractionStatusSuperseded, InteractionStatusCancelled, InteractionStatusInterrupted},
	InteractionStatusContextReady:    {InteractionStatusDecided, InteractionStatusGenerated, InteractionStatusCommitted, InteractionStatusFailed, InteractionStatusSuperseded, InteractionStatusCancelled},
	InteractionStatusDecided:         {InteractionStatusGenerated, InteractionStatusFailed, InteractionStatusSuperseded, InteractionStatusCancelled},
	InteractionStatusGenerated:       {InteractionStatusCommitted, InteractionStatusFailed, InteractionStatusSuperseded, InteractionStatusCancelled},
	InteractionStatusCommitted:       {InteractionStatusDeliveryPending, InteractionStatusDelivered, InteractionStatusCompleted, InteractionStatusFailed},
	InteractionStatusDeliveryPending: {InteractionStatusDelivered, InteractionStatusCompleted, InteractionStatusFailed},
	InteractionStatusDelivered:       {InteractionStatusCompleted},
	InteractionStatusCompleted:       {InteractionStatusArchived},
	InteractionStatusFailed:          {InteractionStatusArchived},
	InteractionStatusSuperseded:      {InteractionStatusArchived},
	InteractionStatusCancelled:       {InteractionStatusArchived},
	InteractionStatusInterrupted:     {InteractionStatusArchived},
	InteractionStatusArchived:        {},
}

var (
	ErrInvalidTransition   = errors.New("interaction: invalid state transition")
	ErrAlreadyTerminal     = errors.New("interaction: already in terminal state")
	ErrVersionConflict     = errors.New("interaction: status version conflict")
	ErrInteractionNotFound = errors.New("interaction: record not found")
)

type InteractionRecord struct {
	ID                string             `json:"id"`
	Scope             InteractionScope   `json:"scope"`
	Priority          int                `json:"priority"`
	PathType          string             `json:"pathType,omitempty"`
	Status            InteractionStatus  `json:"status"`
	StatusVersion     int64              `json:"statusVersion"`
	SupersedesID      string             `json:"supersedesId,omitempty"`
	SupersededByID    string             `json:"supersededById,omitempty"`
	CancelReason      string             `json:"cancelReason,omitempty"`
	ErrorCode         string             `json:"errorCode,omitempty"`
	ErrorMessage      string             `json:"errorMessage,omitempty"`
	ResultRef         string             `json:"resultRef,omitempty"`
	CommitID          string             `json:"commitId,omitempty"`
	ExecutorID        string             `json:"executorId,omitempty"`
	DeadlineAt        time.Time          `json:"deadlineAt,omitempty"`
	CancelRequestedAt time.Time          `json:"cancelRequestedAt,omitempty"`
	CreatedAt         time.Time          `json:"createdAt"`
	StartedAt         time.Time          `json:"startedAt,omitempty"`
	CommittedAt       time.Time          `json:"committedAt,omitempty"`
	CompletedAt       time.Time          `json:"completedAt,omitempty"`
	UpdatedAt         time.Time          `json:"updatedAt"`
	mu                sync.RWMutex       `json:"-"`
	cancel            context.CancelFunc `json:"-"`
}

func NewInteractionRecord(scope InteractionScope) *InteractionRecord {
	return &InteractionRecord{
		ID:        uuid.New().String(),
		Scope:     scope.Normalize(),
		Status:    InteractionStatusReceived,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (r *InteractionRecord) IsTerminal() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return isTerminalStatus(r.Status)
}

func (r *InteractionRecord) IsActive() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return !isTerminalStatus(r.Status)
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
	if isTerminalStatus(r.Status) && target != InteractionStatusArchived {
		return ErrAlreadyTerminal
	}
	r.Status = target
	r.StatusVersion++
	r.UpdatedAt = time.Now()
	switch target {
	case InteractionStatusProcessing:
		r.StartedAt = time.Now()
	case InteractionStatusCommitted:
		r.CommittedAt = time.Now()
	case InteractionStatusCompleted, InteractionStatusFailed, InteractionStatusCancelled, InteractionStatusSuperseded, InteractionStatusInterrupted:
		r.CompletedAt = time.Now()
	}
	return nil
}

func isTerminalStatus(status InteractionStatus) bool {
	return status == InteractionStatusCompleted ||
		status == InteractionStatusSuperseded ||
		status == InteractionStatusCancelled ||
		status == InteractionStatusFailed ||
		status == InteractionStatusInterrupted ||
		status == InteractionStatusArchived
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
		ID:                r.ID,
		Scope:             r.Scope,
		Priority:          r.Priority,
		PathType:          r.PathType,
		Status:            r.Status,
		StatusVersion:     r.StatusVersion,
		SupersedesID:      r.SupersedesID,
		SupersededByID:    r.SupersededByID,
		CancelReason:      r.CancelReason,
		ErrorCode:         r.ErrorCode,
		ErrorMessage:      r.ErrorMessage,
		ResultRef:         r.ResultRef,
		CommitID:          r.CommitID,
		ExecutorID:        r.ExecutorID,
		DeadlineAt:        r.DeadlineAt,
		CancelRequestedAt: r.CancelRequestedAt,
		CreatedAt:         r.CreatedAt,
		StartedAt:         r.StartedAt,
		CommittedAt:       r.CommittedAt,
		CompletedAt:       r.CompletedAt,
		UpdatedAt:         r.UpdatedAt,
	}
}

type InteractionTracker interface {
	Create(ctx context.Context, record *InteractionRecord) error
	Get(ctx context.Context, id string) (*InteractionRecord, bool, error)
	ListActive(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error)
	ListByScope(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error)
	TransitionCAS(ctx context.Context, id string, expectedVersion int64, target InteractionStatus) (*InteractionRecord, error)
	RequestCancel(ctx context.Context, id string, reason string) error
	MarkSuperseded(ctx context.Context, targetID string, supersededByID string) error
	Complete(ctx context.Context, id string, resultRef string) (*InteractionRecord, error)
	Fail(ctx context.Context, id string, code string, message string) (*InteractionRecord, error)
	Archive(ctx context.Context, id string) error
	Range(ctx context.Context, fn func(record *InteractionRecord) bool) error
}

type InMemoryTracker struct {
	mu      sync.RWMutex
	records map[string]*InteractionRecord
	byConv  map[string]map[string]struct{}
	byChar  map[string]map[string]struct{}
}

func NewInMemoryTracker() *InMemoryTracker {
	return &InMemoryTracker{
		records: make(map[string]*InteractionRecord),
		byConv:  make(map[string]map[string]struct{}),
		byChar:  make(map[string]map[string]struct{}),
	}
}

func (t *InMemoryTracker) Create(ctx context.Context, record *InteractionRecord) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
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
	return nil
}

func (t *InMemoryTracker) Get(ctx context.Context, id string) (*InteractionRecord, bool, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	rec, ok := t.records[id]
	return rec, ok, nil
}

func (t *InMemoryTracker) ListActive(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
	return result, nil
}

func (t *InMemoryTracker) ListByScope(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
	return result, nil
}

func (t *InMemoryTracker) TransitionCAS(ctx context.Context, id string, expectedVersion int64, target InteractionStatus) (*InteractionRecord, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rec, ok := t.records[id]
	if !ok {
		return nil, ErrInteractionNotFound
	}
	if rec.StatusVersion != expectedVersion {
		return nil, ErrVersionConflict
	}
	if err := rec.Transition(target); err != nil {
		return nil, err
	}
	snap := rec.Snapshot()
	return &snap, nil
}

func (t *InMemoryTracker) RequestCancel(ctx context.Context, id string, reason string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	rec, ok := t.records[id]
	if !ok {
		return ErrInteractionNotFound
	}
	rec.CancelReason = reason
	rec.CancelRequestedAt = time.Now()
	rec.UpdatedAt = rec.CancelRequestedAt
	return nil
}

func (t *InMemoryTracker) MarkSuperseded(ctx context.Context, targetID string, supersededByID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	rec, ok := t.records[targetID]
	if !ok {
		return ErrInteractionNotFound
	}
	rec.SupersededByID = supersededByID
	rec.Status = InteractionStatusSuperseded
	rec.StatusVersion++
	rec.CompletedAt = time.Now()
	rec.UpdatedAt = rec.CompletedAt
	return nil
}

func (t *InMemoryTracker) Complete(ctx context.Context, id string, resultRef string) (*InteractionRecord, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rec, ok := t.records[id]
	if !ok {
		return nil, ErrInteractionNotFound
	}
	rec.ResultRef = resultRef
	rec.Status = InteractionStatusCompleted
	rec.StatusVersion++
	rec.CompletedAt = time.Now()
	rec.UpdatedAt = rec.CompletedAt
	snap := rec.Snapshot()
	return &snap, nil
}

func (t *InMemoryTracker) Fail(ctx context.Context, id string, code string, message string) (*InteractionRecord, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rec, ok := t.records[id]
	if !ok {
		return nil, ErrInteractionNotFound
	}
	rec.ErrorCode = code
	rec.ErrorMessage = message
	rec.Status = InteractionStatusFailed
	rec.StatusVersion++
	rec.CompletedAt = time.Now()
	rec.UpdatedAt = rec.CompletedAt
	snap := rec.Snapshot()
	return &snap, nil
}

func (t *InMemoryTracker) Archive(ctx context.Context, id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	rec, ok := t.records[id]
	if !ok {
		return ErrInteractionNotFound
	}
	rec.Status = InteractionStatusArchived
	rec.StatusVersion++
	rec.UpdatedAt = time.Now()
	return nil
}

func (t *InMemoryTracker) Range(ctx context.Context, fn func(record *InteractionRecord) bool) error {
	t.mu.RLock()
	if err := ctx.Err(); err != nil {
		t.mu.RUnlock()
		return err
	}
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
	return nil
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
