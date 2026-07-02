package interaction

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InteractionRecordModel struct {
	ID              string    `gorm:"primaryKey;column:id"`
	ConversationID  string    `gorm:"column:conversation_id;index"`
	CharacterID     string    `gorm:"column:character_id;index"`
	Channel         string    `gorm:"column:channel"`
	Status          string    `gorm:"column:status;index"`
	SupersedesID    string    `gorm:"column:supersedes_id"`
	SupersededByID  string    `gorm:"column:superseded_by_id"`
	CancelReason    string    `gorm:"column:cancel_reason"`
	ErrorMessage    string    `gorm:"column:error_message"`
	CreatedAt       time.Time `gorm:"column:created_at;index"`
	StartedAt       time.Time `gorm:"column:started_at"`
	CompletedAt     time.Time `gorm:"column:completed_at"`
}

func (InteractionRecordModel) TableName() string {
	return "interaction_records"
}

type SQLiteInteractionTracker struct {
	db *gorm.DB
}

func NewSQLiteInteractionTracker(db *gorm.DB) *SQLiteInteractionTracker {
	return &SQLiteInteractionTracker{db: db}
}

func (t *SQLiteInteractionTracker) InitSchema() error {
	return t.db.AutoMigrate(&InteractionRecordModel{})
}

func (t *SQLiteInteractionTracker) Track(record *InteractionRecord) {
	model := recordToModel(record)
	t.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&model)
}

func (t *SQLiteInteractionTracker) Get(id string) (*InteractionRecord, bool) {
	var model InteractionRecordModel
	err := t.db.Where("id = ?", id).First(&model).Error
	if err != nil {
		return nil, false
	}
	return modelToInteractionRecord(model), true
}

func (t *SQLiteInteractionTracker) GetActiveByScope(scope InteractionScope) []*InteractionRecord {
	var models []InteractionRecordModel
	t.db.Where("conversation_id = ? AND status IN ?",
		scope.ConversationID,
		[]string{string(InteractionStatusPending), string(InteractionStatusProcessing)},
	).Find(&models)
	result := make([]*InteractionRecord, 0, len(models))
	for _, m := range models {
		result = append(result, modelToInteractionRecord(m))
	}
	return result
}

func (t *SQLiteInteractionTracker) GetByScope(scope InteractionScope) []*InteractionRecord {
	var models []InteractionRecordModel
	t.db.Where("conversation_id = ?", scope.ConversationID).Find(&models)
	result := make([]*InteractionRecord, 0, len(models))
	for _, m := range models {
		result = append(result, modelToInteractionRecord(m))
	}
	return result
}

func (t *SQLiteInteractionTracker) Remove(id string) {
	t.db.Where("id = ?", id).Delete(&InteractionRecordModel{})
}

func (t *SQLiteInteractionTracker) Range(fn func(record *InteractionRecord) bool) {
	var models []InteractionRecordModel
	t.db.Order("created_at ASC").Find(&models)
	for _, m := range models {
		if !fn(modelToInteractionRecord(m)) {
			break
		}
	}
}

func recordToModel(r *InteractionRecord) InteractionRecordModel {
	return InteractionRecordModel{
		ID:             r.ID,
		ConversationID: r.Scope.ConversationID,
		CharacterID:    r.Scope.CharacterID,
		Channel:        r.Scope.Channel,
		Status:         string(r.Status),
		SupersedesID:   r.SupersedesID,
		SupersededByID: r.SupersededByID,
		CancelReason:   r.CancelReason,
		ErrorMessage:   r.ErrorMessage,
		CreatedAt:      r.CreatedAt,
		StartedAt:      r.StartedAt,
		CompletedAt:    r.CompletedAt,
	}
}

func modelToInteractionRecord(m InteractionRecordModel) *InteractionRecord {
	return &InteractionRecord{
		ID: m.ID,
		Scope: InteractionScope{
			ConversationID: m.ConversationID,
			CharacterID:    m.CharacterID,
			Channel:        m.Channel,
		},
		Status:        InteractionStatus(m.Status),
		SupersedesID:  m.SupersedesID,
		SupersededByID: m.SupersededByID,
		CancelReason:  m.CancelReason,
		ErrorMessage:  m.ErrorMessage,
		CreatedAt:     m.CreatedAt,
		StartedAt:     m.StartedAt,
		CompletedAt:   m.CompletedAt,
	}
}

func UUID() string {
	return uuid.New().String()
}

var _ InteractionTracker = (*SQLiteInteractionTracker)(nil)
