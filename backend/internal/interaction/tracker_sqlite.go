package interaction

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

type SQLiteInteractionTracker struct {
	db    *gorm.DB
	clock func() time.Time
}

func NewSQLiteInteractionTracker(db *gorm.DB) *SQLiteInteractionTracker {
	return &SQLiteInteractionTracker{db: db, clock: time.Now}
}

func (t *SQLiteInteractionTracker) InitSchema() error {
	if err := t.db.AutoMigrate(&InteractionRecordModel{}); err != nil {
		return err
	}
	if err := t.db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_interaction_request_unique ON interaction_records(user_id, request_id) WHERE request_id <> ''").Error; err != nil {
		return err
	}
	return t.db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_interaction_conv_request_unique ON interaction_records(conversation_id, request_id) WHERE request_id <> ''").Error
}

func (t *SQLiteInteractionTracker) Create(ctx context.Context, record *InteractionRecord) error {
	model := recordToModel(record)
	if model.ID == "" {
		model.ID = uuid.New().String()
	}
	now := t.now()
	if model.CreatedAt.IsZero() {
		model.CreatedAt = now
	}
	if model.UpdatedAt.IsZero() {
		model.UpdatedAt = now
	}
	result := t.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDuplicateRequest
	}
	record.ID = model.ID
	record.CreatedAt = model.CreatedAt
	record.UpdatedAt = model.UpdatedAt
	return nil
}

func (t *SQLiteInteractionTracker) Get(ctx context.Context, id string) (*InteractionRecord, bool, error) {
	var model InteractionRecordModel
	err := t.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return modelToInteractionRecord(model), true, nil
}

func (t *SQLiteInteractionTracker) GetByRequestID(ctx context.Context, userID string, requestID string) (*InteractionRecord, bool, error) {
	scope := InteractionScope{UserID: userID, RequestID: requestID}.Normalize()
	if scope.RequestID == "" {
		return nil, false, nil
	}
	var model InteractionRecordModel
	err := t.db.WithContext(ctx).
		Where("user_id = ? AND request_id = ?", scope.UserID, scope.RequestID).
		Order("created_at DESC").
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return modelToInteractionRecord(model), true, nil
}

func (t *SQLiteInteractionTracker) ListActive(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error) {
	return t.list(ctx, scope, true)
}

func (t *SQLiteInteractionTracker) ListByScope(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error) {
	return t.list(ctx, scope, false)
}

func (t *SQLiteInteractionTracker) TransitionCAS(ctx context.Context, id string, expectedVersion int64, target InteractionStatus) (*InteractionRecord, error) {
	rec, ok, err := t.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInteractionNotFound
	}
	if rec.StatusVersion != expectedVersion {
		return nil, ErrVersionConflict
	}
	if err := rec.Transition(target); err != nil {
		return nil, err
	}
	updates := transitionUpdates(rec)
	result := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ? AND status_version = ?", id, expectedVersion).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrVersionConflict
	}
	return rec, nil
}

func (t *SQLiteInteractionTracker) UpdateMetadata(ctx context.Context, id string, update InteractionMetadataUpdate) (*InteractionRecord, error) {
	updates := map[string]interface{}{}
	if update.Priority != nil {
		updates["priority"] = *update.Priority
	}
	if update.PathType != nil {
		updates["path_type"] = *update.PathType
	}
	if update.SupersedesID != nil {
		updates["supersedes_id"] = *update.SupersedesID
	}
	if update.CommitID != nil {
		updates["commit_id"] = *update.CommitID
	}
	if update.ExecutorID != nil {
		updates["executor_id"] = *update.ExecutorID
	}
	if update.OwnerInstanceID != nil {
		updates["owner_instance_id"] = *update.OwnerInstanceID
	}
	if update.CommitToken != nil {
		updates["commit_token"] = *update.CommitToken
	}
	if update.CommitOwner != nil {
		updates["commit_owner"] = *update.CommitOwner
	}
	if update.ResultMessageIDs != nil {
		updates["result_message_ids"] = *update.ResultMessageIDs
	}
	if update.DeliveryIntentIDs != nil {
		updates["delivery_intent_ids"] = *update.DeliveryIntentIDs
	}
	if update.CorrelationID != nil {
		updates["correlation_id"] = *update.CorrelationID
	}
	if update.CausationID != nil {
		updates["causation_id"] = *update.CausationID
	}
	if update.DeadlineAt != nil {
		updates["deadline_at"] = *update.DeadlineAt
	}
	if len(updates) == 0 {
		rec, ok, err := t.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrInteractionNotFound
		}
		return rec, nil
	}
	updates["updated_at"] = t.now()
	result := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ?", id)

	casSet := false
	if update.ExpectedStatusVersion != nil {
		result = result.Where("status_version = ?", *update.ExpectedStatusVersion)
		casSet = true
	}
	if update.ExpectedOwner != nil {
		result = result.Where("commit_owner = ?", *update.ExpectedOwner)
		casSet = true
	}

	result = result.Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if casSet {
			return nil, ErrInteractionCASConflict
		}
		return nil, ErrInteractionNotFound
	}
	rec, ok, err := t.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInteractionNotFound
	}
	return rec, nil
}

func (t *SQLiteInteractionTracker) AcquireCommitToken(ctx context.Context, id string, expectedVersion int64) (*CommitToken, error) {
	token := uuid.New().String()
	owner := uuid.New().String()
	now := t.now()
	result := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ? AND status_version = ? AND status = ?",
			id, expectedVersion, string(InteractionStatusGenerated)).
		Updates(map[string]interface{}{
			"commit_token":       token,
			"commit_owner":       owner,
			"commit_acquired_at": now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrCommitTokenUnavailable
	}
	return &CommitToken{
		InteractionID: id,
		Version:       expectedVersion,
		Owner:         owner,
		Token:         token,
	}, nil
}
func (t *SQLiteInteractionTracker) RequestCancel(ctx context.Context, id string, reason string) error {
	now := t.now()
	result := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ? AND status IN ?", id, cancellableStatusStrings()).
		Updates(map[string]interface{}{
			"cancel_reason":       reason,
			"cancel_requested_at": now,
			"status_version":      gorm.Expr("status_version + 1"),
			"updated_at":          now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		rec, ok, err := t.Get(ctx, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInteractionNotFound
		}
		if rec.Status == InteractionStatusCancelled || rec.Status == InteractionStatusSuperseded || rec.IsTerminal() {
			return nil
		}
		return ErrInvalidTransition
	}
	return nil
}

func (t *SQLiteInteractionTracker) MarkSuperseded(ctx context.Context, targetID string, supersededByID string) error {
	now := t.now()
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var target InteractionRecordModel
		err := tx.Where("id = ?", targetID).First(&target).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInteractionNotFound
			}
			return err
		}
		if isTerminalStatus(InteractionStatus(target.Status)) {
			return ErrAlreadyTerminal
		}
		if !canSupersedeStatus(InteractionStatus(target.Status)) {
			return ErrInvalidTransition
		}
		if target.CommitID != "" || !target.CommittedAt.IsZero() {
			return ErrInvalidTransition
		}
		if supersededByID == "" || supersededByID == targetID {
			return ErrInvalidTransition
		}
		var superseder InteractionRecordModel
		err = tx.Where("id = ?", supersededByID).First(&superseder).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInteractionNotFound
			}
			return err
		}
		if isTerminalStatus(InteractionStatus(superseder.Status)) || !sameSupersedeScope(modelToInteractionRecord(target).Scope, modelToInteractionRecord(superseder).Scope) {
			return ErrInvalidTransition
		}
		result := tx.Model(&InteractionRecordModel{}).
			Where("id = ? AND status_version = ? AND status IN ?", targetID, target.StatusVersion, supersedableStatusStrings()).
			Updates(map[string]interface{}{
				"status":           string(InteractionStatusSuperseded),
				"status_version":   gorm.Expr("status_version + 1"),
				"superseded_by_id": supersededByID,
				"completed_at":     now,
				"updated_at":       now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrVersionConflict
		}
		return nil
	})
}

func (t *SQLiteInteractionTracker) Complete(ctx context.Context, id string, expectedVersion int64, resultRef string) (*InteractionRecord, error) {
	return t.finish(ctx, id, expectedVersion, InteractionStatusCompleted, map[string]interface{}{"result_ref": resultRef})
}

func (t *SQLiteInteractionTracker) Fail(ctx context.Context, id string, expectedVersion int64, code string, message string) (*InteractionRecord, error) {
	return t.finish(ctx, id, expectedVersion, InteractionStatusFailed, map[string]interface{}{"error_code": code, "error_message": message})
}

func (t *SQLiteInteractionTracker) Archive(ctx context.Context, id string, expectedVersion int64) error {
	_, err := t.transitionWithExpectedVersion(ctx, id, expectedVersion, InteractionStatusArchived, nil)
	return err
}

func (t *SQLiteInteractionTracker) Range(ctx context.Context, fn func(record *InteractionRecord) bool) error {
	var models []InteractionRecordModel
	if err := t.db.WithContext(ctx).Order("created_at ASC").Find(&models).Error; err != nil {
		return err
	}
	for _, m := range models {
		if !fn(modelToInteractionRecord(m)) {
			break
		}
	}
	return nil
}

func (t *SQLiteInteractionTracker) list(ctx context.Context, scope InteractionScope, activeOnly bool) ([]*InteractionRecord, error) {
	scope = scope.Normalize()
	query := t.db.WithContext(ctx).Model(&InteractionRecordModel{})
	if scope.UserID != "" {
		query = query.Where("user_id = ?", scope.UserID)
	}
	if scope.CharacterID != "" {
		query = query.Where("character_id = ?", scope.CharacterID)
	}
	if scope.ConversationID != "" {
		query = query.Where("conversation_id = ?", scope.ConversationID)
	}
	if scope.Channel != "" {
		query = query.Where("channel = ?", scope.Channel)
	}
	if scope.PeerID != "" {
		query = query.Where("peer_id = ?", scope.PeerID)
	}
	if activeOnly {
		query = query.Where("status IN ?", activeStatusStrings())
	}
	var models []InteractionRecordModel
	if err := query.Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]*InteractionRecord, 0, len(models))
	for _, m := range models {
		result = append(result, modelToInteractionRecord(m))
	}
	return result, nil
}

func (t *SQLiteInteractionTracker) finish(ctx context.Context, id string, expectedVersion int64, status InteractionStatus, fields map[string]interface{}) (*InteractionRecord, error) {
	return t.transitionWithExpectedVersion(ctx, id, expectedVersion, status, fields)
}

func (t *SQLiteInteractionTracker) transitionWithExpectedVersion(ctx context.Context, id string, expectedVersion int64, status InteractionStatus, fields map[string]interface{}) (*InteractionRecord, error) {
	rec, ok, err := t.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInteractionNotFound
	}
	if rec.StatusVersion != expectedVersion {
		return nil, ErrVersionConflict
	}
	if rec.IsTerminal() && status != InteractionStatusArchived {
		return nil, ErrAlreadyTerminal
	}
	if err := rec.Transition(status); err != nil {
		return nil, err
	}
	now := t.now()
	if fields == nil {
		fields = map[string]interface{}{}
	}
	fields["status"] = string(status)
	fields["status_version"] = rec.StatusVersion
	fields["started_at"] = rec.StartedAt
	fields["committed_at"] = rec.CommittedAt
	fields["completed_at"] = rec.CompletedAt
	fields["updated_at"] = now
	result := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ? AND status_version = ?", id, rec.StatusVersion-1).
		Updates(fields)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrVersionConflict
	}
	return rec, nil
}

func (t *SQLiteInteractionTracker) now() time.Time {
	if t.clock != nil {
		return t.clock()
	}
	return time.Now()
}

func recordToModel(r *InteractionRecord) InteractionRecordModel {
	scope := r.Scope.Normalize()
	return InteractionRecordModel{
		ID:                r.ID,
		UserID:            scope.UserID,
		CharacterID:       scope.CharacterID,
		ConversationID:    scope.ConversationID,
		Channel:           scope.Channel,
		PeerID:            scope.PeerID,
		SessionID:         scope.SessionID,
		Source:            scope.Source,
		RequestID:         scope.RequestID,
		Priority:          r.Priority,
		PathType:          r.PathType,
		Status:            string(r.Status),
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

func modelToInteractionRecord(m InteractionRecordModel) *InteractionRecord {
	return &InteractionRecord{
		ID: m.ID,
		Scope: InteractionScope{
			UserID:         m.UserID,
			CharacterID:    m.CharacterID,
			ConversationID: m.ConversationID,
			Channel:        m.Channel,
			PeerID:         m.PeerID,
			SessionID:      m.SessionID,
			Source:         m.Source,
			RequestID:      m.RequestID,
		}.Normalize(),
		Priority:          m.Priority,
		PathType:          m.PathType,
		Status:            InteractionStatus(m.Status),
		StatusVersion:     m.StatusVersion,
		SupersedesID:      m.SupersedesID,
		SupersededByID:    m.SupersededByID,
		CancelReason:      m.CancelReason,
		ErrorCode:         m.ErrorCode,
		ErrorMessage:      m.ErrorMessage,
		ResultRef:         m.ResultRef,
		CommitID:          m.CommitID,
		ExecutorID:        m.ExecutorID,
		OwnerInstanceID:   m.OwnerInstanceID,
		HeartbeatAt:       m.HeartbeatAt,
		CommitToken:       m.CommitToken,
		CommitOwner:       m.CommitOwner,
		CommitAcquiredAt:  m.CommitAcquiredAt,
		ResultMessageIDs:  m.ResultMessageIDs,
		DeliveryIntentIDs: m.DeliveryIntentIDs,
		CorrelationID:     m.CorrelationID,
		CausationID:       m.CausationID,
		DeadlineAt:        m.DeadlineAt,
		CancelRequestedAt: m.CancelRequestedAt,
		CreatedAt:         m.CreatedAt,
		StartedAt:         m.StartedAt,
		CommittedAt:       m.CommittedAt,
		CompletedAt:       m.CompletedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

func transitionUpdates(r *InteractionRecord) map[string]interface{} {
	return map[string]interface{}{
		"status":         string(r.Status),
		"status_version": r.StatusVersion,
		"started_at":     r.StartedAt,
		"committed_at":   r.CommittedAt,
		"completed_at":   r.CompletedAt,
		"updated_at":     r.UpdatedAt,
	}
}

func activeStatusStrings() []string {
	statuses := []InteractionStatus{
		InteractionStatusReceived,
		InteractionStatusNormalized,
		InteractionStatusQueued,
		InteractionStatusProcessing,
		InteractionStatusContextReady,
		InteractionStatusDecided,
		InteractionStatusGenerated,
		InteractionStatusCommitted,
		InteractionStatusDeliveryPending,
		InteractionStatusDelivered,
	}
	result := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if isActiveStatus(status) {
			result = append(result, string(status))
		}
	}
	return result
}

func supersedableStatusStrings() []string {
	return cancellableStatusStrings()
}

func cancellableStatusStrings() []string {
	return []string{
		string(InteractionStatusReceived),
		string(InteractionStatusNormalized),
		string(InteractionStatusQueued),
		string(InteractionStatusProcessing),
		string(InteractionStatusContextReady),
		string(InteractionStatusDecided),
		string(InteractionStatusGenerated),
	}
}

func terminalStatusStrings() []string {
	return []string{
		string(InteractionStatusCompleted),
		string(InteractionStatusSuperseded),
		string(InteractionStatusCancelled),
		string(InteractionStatusFailed),
		string(InteractionStatusInterrupted),
		string(InteractionStatusArchived),
	}
}

func UUID() string {
	return uuid.New().String()
}

var _ InteractionTracker = (*SQLiteInteractionTracker)(nil)
