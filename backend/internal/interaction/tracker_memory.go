package interaction

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

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
	if !ok {
		return nil, false, nil
	}
	snap := rec.Snapshot()
	return &snap, true, nil
}

func (t *InMemoryTracker) GetByRequestID(ctx context.Context, userID string, requestID string) (*InteractionRecord, bool, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	scope := InteractionScope{UserID: userID, RequestID: requestID}.Normalize()
	if scope.RequestID == "" {
		return nil, false, nil
	}
	var selected *InteractionRecord
	for _, rec := range t.records {
		if rec.Scope.UserID != scope.UserID || rec.Scope.RequestID != scope.RequestID {
			continue
		}
		if selected == nil || rec.CreatedAt.After(selected.CreatedAt) {
			selected = rec
		}
	}
	if selected == nil {
		return nil, false, nil
	}
	snap := selected.Snapshot()
	return &snap, true, nil
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
			snap := rec.Snapshot()
			result = append(result, &snap)
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
			snap := rec.Snapshot()
			result = append(result, &snap)
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

func (t *InMemoryTracker) UpdateMetadata(ctx context.Context, id string, update InteractionMetadataUpdate) (*InteractionRecord, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rec, ok := t.records[id]
	if !ok {
		return nil, ErrInteractionNotFound
	}
	rec.mu.Lock()
	if update.Priority != nil {
		rec.Priority = *update.Priority
	}
	if update.PathType != nil {
		rec.PathType = *update.PathType
	}
	if update.SupersedesID != nil {
		rec.SupersedesID = *update.SupersedesID
	}
	if update.CommitID != nil {
		rec.CommitID = *update.CommitID
	}
	if update.ExecutorID != nil {
		rec.ExecutorID = *update.ExecutorID
	}
	if update.DeadlineAt != nil {
		rec.DeadlineAt = *update.DeadlineAt
	}
	rec.UpdatedAt = time.Now()
	rec.mu.Unlock()
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
	if rec.Status == InteractionStatusCancelled || rec.Status == InteractionStatusSuperseded || rec.IsTerminal() {
		return nil
	}
	if !canSupersedeStatus(rec.Status) {
		return ErrInvalidTransition
	}
	rec.mu.Lock()
	rec.CancelReason = reason
	rec.CancelRequestedAt = time.Now()
	rec.StatusVersion++
	rec.UpdatedAt = rec.CancelRequestedAt
	rec.mu.Unlock()
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
	if rec.IsTerminal() {
		return ErrAlreadyTerminal
	}
	if !canSupersedeStatus(rec.Status) {
		return ErrInvalidTransition
	}
	if rec.CommitID != "" || !rec.CommittedAt.IsZero() {
		return ErrInvalidTransition
	}
	if supersededByID == "" || supersededByID == targetID {
		return ErrInvalidTransition
	}
	superseder, ok := t.records[supersededByID]
	if !ok {
		return ErrInteractionNotFound
	}
	if superseder.IsTerminal() || !sameSupersedeScope(rec.Scope, superseder.Scope) {
		return ErrInvalidTransition
	}
	rec.mu.Lock()
	rec.SupersededByID = supersededByID
	rec.Status = InteractionStatusSuperseded
	rec.StatusVersion++
	rec.CompletedAt = time.Now()
	rec.UpdatedAt = rec.CompletedAt
	rec.mu.Unlock()
	return nil
}

func (t *InMemoryTracker) Complete(ctx context.Context, id string, expectedVersion int64, resultRef string) (*InteractionRecord, error) {
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
	if rec.IsTerminal() {
		return nil, ErrAlreadyTerminal
	}
	if err := rec.Transition(InteractionStatusCompleted); err != nil {
		return nil, err
	}
	rec.ResultRef = resultRef
	snap := rec.Snapshot()
	return &snap, nil
}

func (t *InMemoryTracker) Fail(ctx context.Context, id string, expectedVersion int64, code string, message string) (*InteractionRecord, error) {
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
	if rec.IsTerminal() {
		return nil, ErrAlreadyTerminal
	}
	if err := rec.Transition(InteractionStatusFailed); err != nil {
		return nil, err
	}
	rec.ErrorCode = code
	rec.ErrorMessage = message
	snap := rec.Snapshot()
	return &snap, nil
}

func (t *InMemoryTracker) AcquireCommitToken(ctx context.Context, id string, expectedVersion int64) (*CommitToken, error) {
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
	if rec.Status != InteractionStatusGenerated {
		return nil, ErrCommitTokenUnavailable
	}
	token := uuid.New().String()
	owner := uuid.New().String()
	now := time.Now()
	rec.CommitToken = token
	rec.CommitOwner = owner
	rec.CommitAcquiredAt = now
	return &CommitToken{
		InteractionID: id,
		Version:       expectedVersion,
		Owner:         owner,
		Token:         token,
	}, nil
}
func (t *InMemoryTracker) Archive(ctx context.Context, id string, expectedVersion int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	rec, ok := t.records[id]
	if !ok {
		return ErrInteractionNotFound
	}
	if rec.StatusVersion != expectedVersion {
		return ErrVersionConflict
	}
	return rec.Transition(InteractionStatusArchived)
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
