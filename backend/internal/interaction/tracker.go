package interaction

import (
	"context"
	"time"
)

type InteractionMetadataUpdate struct {
	Priority              *int
	PathType              *string
	SupersedesID          *string
	CommitID              *string
	ExecutorID            *string
	OwnerInstanceID       *string
	CommitToken           *string
	CommitOwner           *string
	ResultMessageIDs      *string
	DeliveryIntentIDs     *string
	CorrelationID         *string
	CausationID           *string
	DeadlineAt            *time.Time
	ExpectedStatusVersion *int64
	ExpectedOwner         *string
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
	Version       int64
	Owner         string
	Token         string
}

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
