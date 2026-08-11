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
	ErrInvalidTransition      = errors.New("interaction: invalid state transition")
	ErrAlreadyTerminal        = errors.New("interaction: already in terminal state")
	ErrVersionConflict        = errors.New("interaction: status version conflict")
	ErrInteractionNotFound    = errors.New("interaction: record not found")
	ErrDuplicateRequest       = errors.New("interaction: duplicate request")
	ErrCommitTokenUnavailable = errors.New("interaction: commit token unavailable")
	ErrInteractionCASConflict = errors.New("interaction: CAS metadata conflict")
)

type InteractionRecord struct {
	ID                 string              `json:"id"`
	Scope              InteractionScope    `json:"scope"`
	Priority           int                 `json:"priority"`
	PathType           string              `json:"pathType,omitempty"`
	Status             InteractionStatus   `json:"status"`
	StatusVersion      int64               `json:"statusVersion"`
	SupersedesID       string              `json:"supersedesId,omitempty"`
	SupersededByID     string              `json:"supersededById,omitempty"`
	CancelReason       string              `json:"cancelReason,omitempty"`
	ErrorCode          string              `json:"errorCode,omitempty"`
	ErrorMessage       string              `json:"errorMessage,omitempty"`
	ResultRef          string              `json:"resultRef,omitempty"`
	CommitID           string              `json:"commitId,omitempty"`
	ExecutorID         string              `json:"executorId,omitempty"`
	OwnerInstanceID    string              `json:"ownerInstanceId,omitempty"`
	HeartbeatAt        time.Time           `json:"heartbeatAt,omitempty"`
	CommitToken        string              `json:"commitToken,omitempty"`
	CommitOwner        string              `json:"commitOwner,omitempty"`
	CommitAcquiredAt   time.Time           `json:"commitAcquiredAt,omitempty"`
	ResultMessageIDs   string              `json:"resultMessageIds,omitempty"`
	DeliveryIntentIDs  string              `json:"deliveryIntentIds,omitempty"`
	CorrelationID      string              `json:"correlationId,omitempty"`
	CausationID        string              `json:"causationId,omitempty"`
	DeadlineAt         time.Time           `json:"deadlineAt,omitempty"`
	CancelRequestedAt  time.Time           `json:"cancelRequestedAt,omitempty"`
	CreatedAt          time.Time           `json:"createdAt"`
	StartedAt          time.Time           `json:"startedAt,omitempty"`
	CommittedAt        time.Time           `json:"committedAt,omitempty"`
	CompletedAt        time.Time           `json:"completedAt,omitempty"`
	UpdatedAt          time.Time           `json:"updatedAt"`
	RecoveryDescriptor *RecoveryDescriptor `json:"recoveryDescriptor,omitempty"`
	mu                 sync.RWMutex        `json:"-"`
	cancel             context.CancelFunc  `json:"-"`
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
