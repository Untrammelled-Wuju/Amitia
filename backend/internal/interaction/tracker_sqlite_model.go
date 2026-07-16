package interaction

import "time"

type InteractionRecordModel struct {
	ID                string    `gorm:"primaryKey;column:id"`
	UserID            string    `gorm:"column:user_id;index:idx_interaction_scope_active,priority:1;index:idx_interaction_request,priority:1"`
	CharacterID       string    `gorm:"column:character_id;index:idx_interaction_scope_active,priority:2"`
	ConversationID    string    `gorm:"column:conversation_id;index:idx_interaction_scope_active,priority:3"`
	Channel           string    `gorm:"column:channel;index"`
	PeerID            string    `gorm:"column:peer_id;index"`
	SessionID         string    `gorm:"column:session_id;index"`
	Source            string    `gorm:"column:source"`
	RequestID         string    `gorm:"column:request_id;index:idx_interaction_request,priority:2"`
	Priority          int       `gorm:"column:priority;index"`
	PathType          string    `gorm:"column:path_type"`
	Status            string    `gorm:"column:status;index:idx_interaction_scope_active,priority:4"`
	StatusVersion     int64     `gorm:"column:status_version;not null;default:0"`
	SupersedesID      string    `gorm:"column:supersedes_id;index"`
	SupersededByID    string    `gorm:"column:superseded_by_id;index"`
	CancelReason      string    `gorm:"column:cancel_reason"`
	ErrorCode         string    `gorm:"column:error_code"`
	ErrorMessage      string    `gorm:"column:error_message"`
	ResultRef         string    `gorm:"column:result_ref"`
	CommitID          string    `gorm:"column:commit_id"`
	ExecutorID        string    `gorm:"column:executor_id;index"`
	OwnerInstanceID   string    `gorm:"column:owner_instance_id"`
	HeartbeatAt       time.Time `gorm:"column:heartbeat_at"`
	CommitToken       string    `gorm:"column:commit_token"`
	CommitOwner       string    `gorm:"column:commit_owner"`
	CommitAcquiredAt  time.Time `gorm:"column:commit_acquired_at"`
	ResultMessageIDs  string    `gorm:"column:result_message_ids"`
	DeliveryIntentIDs string    `gorm:"column:delivery_intent_ids"`
	CorrelationID     string    `gorm:"column:correlation_id"`
	CausationID       string    `gorm:"column:causation_id"`
	DeadlineAt        time.Time `gorm:"column:deadline_at"`
	CancelRequestedAt time.Time `gorm:"column:cancel_requested_at"`
	CreatedAt         time.Time `gorm:"column:created_at;index"`
	StartedAt         time.Time `gorm:"column:started_at"`
	CommittedAt       time.Time `gorm:"column:committed_at"`
	CompletedAt       time.Time `gorm:"column:completed_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at;index"`
}

func (InteractionRecordModel) TableName() string {
	return "interaction_records"
}

func recordToModel(r *InteractionRecord) InteractionRecordModel {
	scope := r.Scope.Normalize()
	return InteractionRecordModel{ID: r.ID, UserID: scope.UserID, CharacterID: scope.CharacterID, ConversationID: scope.ConversationID, Channel: scope.Channel, PeerID: scope.PeerID, SessionID: scope.SessionID, Source: scope.Source, RequestID: scope.RequestID, Priority: r.Priority, PathType: r.PathType, Status: string(r.Status), StatusVersion: r.StatusVersion, SupersedesID: r.SupersedesID, SupersededByID: r.SupersededByID, CancelReason: r.CancelReason, ErrorCode: r.ErrorCode, ErrorMessage: r.ErrorMessage, ResultRef: r.ResultRef, CommitID: r.CommitID, ExecutorID: r.ExecutorID, OwnerInstanceID: r.OwnerInstanceID, HeartbeatAt: r.HeartbeatAt, CommitToken: r.CommitToken, CommitOwner: r.CommitOwner, CommitAcquiredAt: r.CommitAcquiredAt, ResultMessageIDs: r.ResultMessageIDs, DeliveryIntentIDs: r.DeliveryIntentIDs, CorrelationID: r.CorrelationID, CausationID: r.CausationID, DeadlineAt: r.DeadlineAt, CancelRequestedAt: r.CancelRequestedAt, CreatedAt: r.CreatedAt, StartedAt: r.StartedAt, CommittedAt: r.CommittedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt}
}

func modelToInteractionRecord(m InteractionRecordModel) *InteractionRecord {
	return &InteractionRecord{ID: m.ID, Scope: InteractionScope{UserID: m.UserID, CharacterID: m.CharacterID, ConversationID: m.ConversationID, Channel: m.Channel, PeerID: m.PeerID, SessionID: m.SessionID, Source: m.Source, RequestID: m.RequestID}.Normalize(), Priority: m.Priority, PathType: m.PathType, Status: InteractionStatus(m.Status), StatusVersion: m.StatusVersion, SupersedesID: m.SupersedesID, SupersededByID: m.SupersededByID, CancelReason: m.CancelReason, ErrorCode: m.ErrorCode, ErrorMessage: m.ErrorMessage, ResultRef: m.ResultRef, CommitID: m.CommitID, ExecutorID: m.ExecutorID, OwnerInstanceID: m.OwnerInstanceID, HeartbeatAt: m.HeartbeatAt, CommitToken: m.CommitToken, CommitOwner: m.CommitOwner, CommitAcquiredAt: m.CommitAcquiredAt, ResultMessageIDs: m.ResultMessageIDs, DeliveryIntentIDs: m.DeliveryIntentIDs, CorrelationID: m.CorrelationID, CausationID: m.CausationID, DeadlineAt: m.DeadlineAt, CancelRequestedAt: m.CancelRequestedAt, CreatedAt: m.CreatedAt, StartedAt: m.StartedAt, CommittedAt: m.CommittedAt, CompletedAt: m.CompletedAt, UpdatedAt: m.UpdatedAt}
}

func transitionUpdates(r *InteractionRecord) map[string]interface{} {
	return map[string]interface{}{"status": string(r.Status), "status_version": r.StatusVersion, "started_at": r.StartedAt, "committed_at": r.CommittedAt, "completed_at": r.CompletedAt, "updated_at": r.UpdatedAt}
}

func activeStatusStrings() []string {
	statuses := []InteractionStatus{InteractionStatusReceived, InteractionStatusNormalized, InteractionStatusQueued, InteractionStatusProcessing, InteractionStatusContextReady, InteractionStatusDecided, InteractionStatusGenerated, InteractionStatusCommitted, InteractionStatusDeliveryPending, InteractionStatusDelivered}
	result := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if isActiveStatus(status) {
			result = append(result, string(status))
		}
	}
	return result
}

func supersedableStatusStrings() []string { return cancellableStatusStrings() }

func cancellableStatusStrings() []string {
	return []string{string(InteractionStatusReceived), string(InteractionStatusNormalized), string(InteractionStatusQueued), string(InteractionStatusProcessing), string(InteractionStatusContextReady), string(InteractionStatusDecided), string(InteractionStatusGenerated)}
}

func terminalStatusStrings() []string {
	return []string{string(InteractionStatusCompleted), string(InteractionStatusSuperseded), string(InteractionStatusCancelled), string(InteractionStatusFailed), string(InteractionStatusInterrupted), string(InteractionStatusArchived)}
}
