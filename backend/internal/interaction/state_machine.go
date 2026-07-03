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
	ErrDuplicateRequest    = errors.New("interaction: duplicate request")
	ErrCommitTokenUnavailable = errors.New("interaction: commit token unavailable")
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
	OwnerInstanceID   string             `json:"ownerInstanceId,omitempty"`
	HeartbeatAt       time.Time          `json:"heartbeatAt,omitempty"`
	CommitToken       string             `json:"commitToken,omitempty"`
	CommitOwner       string             `json:"commitOwner,omitempty"`
	CommitAcquiredAt  time.Time          `json:"commitAcquiredAt,omitempty"`
	ResultMessageIDs  string             `json:"resultMessageIds,omitempty"`
	DeliveryIntentIDs string             `json:"deliveryIntentIds,omitempty"`
	CorrelationID     string             `json:"correlationId,omitempty"`
	CausationID       string             `json:"causationId,omitempty"`
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

type InteractionMetadataUpdate struct {
	Priority          *int
	PathType          *string
	SupersedesID      *string
	CommitID          *string
	ExecutorID        *string
	OwnerInstanceID   *string
	CommitToken       *string
	CommitOwner       *string
	ResultMessageIDs  *string
	DeliveryIntentIDs *string
	CorrelationID     *string
	CausationID       *string
	DeadlineAt        *time.Time
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
	return isActiveStatus(r.Status)
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

func isActiveStatus(status InteractionStatus) bool {
	switch status {
	case InteractionStatusReceived,
		InteractionStatusNormalized,
		InteractionStatusQueued,
		InteractionStatusProcessing,
		InteractionStatusContextReady,
		InteractionStatusDecided,
		InteractionStatusGenerated,
		InteractionStatusCommitted,
		InteractionStatusDeliveryPending,
		InteractionStatusDelivered:
		return true
	default:
		return false
	}
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
		OwnerInstanceID:   r.OwnerInstanceID,
		HeartbeatAt:       r.HeartbeatAt,
		CommitToken:       r.CommitToken,
		CommitOwner:       r.CommitOwner,
		CommitAcquiredAt:  r.CommitAcquiredAt,
		ResultMessageIDs:  r.ResultMessageIDs,
		DeliveryIntentIDs: r.DeliveryIntentIDs,
		CorrelationID:     r.CorrelationID,
		CausationID:       r.CausationID,
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
	GetByRequestID(ctx context.Context, userID string, requestID string) (*InteractionRecord, bool, error)
	ListActive(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error)
	ListByScope(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error)
	UpdateMetadata(ctx context.Context, id string, update InteractionMetadataUpdate) (*InteractionRecord, error)
	TransitionCAS(ctx context.Context, id string, expectedVersion int64, target InteractionStatus) (*InteractionRecord, error)
	RequestCancel(ctx context.Context, id string, reason string) error
	MarkSuperseded(ctx context.Context, targetID string, supersededByID string) error
	Complete(ctx context.Context, id string, expectedVersion int64, resultRef string) (*InteractionRecord, error)
	Fail(ctx context.Context, id string, expectedVersion int64, code string, message string) (*InteractionRecord, error)
	Archive(ctx context.Context, id string, expectedVersion int64) error
	AcquireCommitToken(ctx context.Context, id string, expectedVersion int64) (*CommitToken, error)
	Range(ctx context.Context, fn func(record *InteractionRecord) bool) error
}

type CommitToken struct {
	InteractionID string
	Version      int64
	Owner        string
	Token        string
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

func canSupersedeStatus(status InteractionStatus) bool {
	switch status {
	case InteractionStatusReceived,
		InteractionStatusNormalized,
		InteractionStatusQueued,
		InteractionStatusProcessing,
		InteractionStatusContextReady,
		InteractionStatusDecided,
		InteractionStatusGenerated:
		return true
	default:
		return false
	}
}

func sameSupersedeScope(a InteractionScope, b InteractionScope) bool {
	a = a.Normalize()
	b = b.Normalize()
	return a.UserID == b.UserID &&
		a.CharacterID == b.CharacterID &&
		a.ConversationID == b.ConversationID &&
		a.Channel == b.Channel &&
		a.PeerID == b.PeerID
}
