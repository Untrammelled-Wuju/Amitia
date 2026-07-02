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
	return t.db.AutoMigrate(&InteractionRecordModel{})
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
	result := t.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&model)
	return result.Error
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

func (t *SQLiteInteractionTracker) RequestCancel(ctx context.Context, id string, reason string) error {
	now := t.now()
	result := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ? AND status NOT IN ?", id, terminalStatusStrings()).
		Updates(map[string]interface{}{
			"cancel_reason":       reason,
			"cancel_requested_at": now,
			"updated_at":          now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		_, ok, err := t.Get(ctx, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInteractionNotFound
		}
	}
	return nil
}

func (t *SQLiteInteractionTracker) MarkSuperseded(ctx context.Context, targetID string, supersededByID string) error {
	now := t.now()
	result := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ? AND status NOT IN ?", targetID, terminalStatusStrings()).
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
		_, ok, err := t.Get(ctx, targetID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInteractionNotFound
		}
	}
	return nil
}

func (t *SQLiteInteractionTracker) Complete(ctx context.Context, id string, resultRef string) (*InteractionRecord, error) {
	return t.finish(ctx, id, InteractionStatusCompleted, map[string]interface{}{"result_ref": resultRef})
}

func (t *SQLiteInteractionTracker) Fail(ctx context.Context, id string, code string, message string) (*InteractionRecord, error) {
	return t.finish(ctx, id, InteractionStatusFailed, map[string]interface{}{"error_code": code, "error_message": message})
}

func (t *SQLiteInteractionTracker) Archive(ctx context.Context, id string) error {
	now := t.now()
	result := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":         string(InteractionStatusArchived),
			"status_version": gorm.Expr("status_version + 1"),
			"updated_at":     now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrInteractionNotFound
	}
	return nil
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

func (t *SQLiteInteractionTracker) finish(ctx context.Context, id string, status InteractionStatus, fields map[string]interface{}) (*InteractionRecord, error) {
	now := t.now()
	fields["status"] = string(status)
	fields["status_version"] = gorm.Expr("status_version + 1")
	fields["completed_at"] = now
	fields["updated_at"] = now
	result := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ? AND status NOT IN ?", id, terminalStatusStrings()).
		Updates(fields)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		_, ok, err := t.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrInteractionNotFound
		}
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
	return []string{
		string(InteractionStatusReceived),
		string(InteractionStatusNormalized),
		string(InteractionStatusQueued),
		string(InteractionStatusProcessing),
		string(InteractionStatusContextReady),
		string(InteractionStatusDecided),
		string(InteractionStatusGenerated),
		string(InteractionStatusCommitted),
		string(InteractionStatusDeliveryPending),
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
