package interaction

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
	if update.RecoveryDescriptor != nil {
		data, err := update.RecoveryDescriptor.NormalizeOnSerialize()
		if err == nil {
			updates["recovery_descriptor_json"] = string(data)
		}
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

func UUID() string {
	return uuid.New().String()
}

var _ InteractionTracker = (*SQLiteInteractionTracker)(nil)
