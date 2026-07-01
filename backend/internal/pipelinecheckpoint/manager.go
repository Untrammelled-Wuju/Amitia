package pipelinecheckpoint

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Record struct {
	ConversationID      string `gorm:"column:conversation_id;primaryKey"`
	PipelineType        string `gorm:"column:pipeline_type;primaryKey"`
	LastMessageSequence int64  `gorm:"column:last_message_sequence"`
	CheckpointVersion   int    `gorm:"column:checkpoint_version"`
	IdempotencyKey      string `gorm:"column:idempotency_key"`
	CreatedAt           string `gorm:"column:created_at"`
	UpdatedAt           string `gorm:"column:updated_at"`
}

func (Record) TableName() string { return "pipeline_checkpoints" }

type Message struct {
	Sequence int64  `gorm:"column:sequence"`
	Role     string `gorm:"column:role"`
	Content  string `gorm:"column:content"`
}

type Manager struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Manager {
	return &Manager{db: db}
}

func (m *Manager) Load(conversationID, pipelineType string) (*Record, error) {
	var record Record
	err := m.db.Where("conversation_id = ? AND pipeline_type = ?", conversationID, pipelineType).First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &Record{
				ConversationID:      conversationID,
				PipelineType:        pipelineType,
				CheckpointVersion:   1,
				LastMessageSequence: 0,
			}, nil
		}
		return nil, err
	}
	return &record, nil
}

func (m *Manager) ResetConversation(conversationID string) error {
	if m.db == nil || conversationID == "" {
		return nil
	}
	return m.db.Where("conversation_id = ?", conversationID).Delete(&Record{}).Error
}

func (m *Manager) ResetAll() error {
	if m.db == nil {
		return nil
	}
	return m.db.Where("1=1").Delete(&Record{}).Error
}

func (m *Manager) Advance(conversationID, pipelineType string, lastSequence int64, idempotencyKey string) error {
	if m.db == nil {
		return fmt.Errorf("checkpoint db is nil")
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	return m.db.Transaction(func(tx *gorm.DB) error {
		var current Record
		err := tx.Where("conversation_id = ? AND pipeline_type = ?", conversationID, pipelineType).First(&current).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == nil {
			if current.LastMessageSequence > lastSequence {
				lastSequence = current.LastMessageSequence
			}
			updates := map[string]interface{}{
				"last_message_sequence": lastSequence,
				"idempotency_key":       idempotencyKey,
				"checkpoint_version":    1,
				"updated_at":            now,
			}
			return tx.Model(&Record{}).Where("conversation_id = ? AND pipeline_type = ?", conversationID, pipelineType).Updates(updates).Error
		}
		record := &Record{
			ConversationID:      conversationID,
			PipelineType:        pipelineType,
			LastMessageSequence: lastSequence,
			CheckpointVersion:   1,
			IdempotencyKey:      idempotencyKey,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		return tx.Create(record).Error
	})
}

func (m *Manager) PendingRange(conversationID, pipelineType string, contextWindow int) ([]map[string]string, int64, error) {
	if m.db == nil {
		return nil, 0, fmt.Errorf("checkpoint db is nil")
	}
	record, err := m.Load(conversationID, pipelineType)
	if err != nil {
		return nil, 0, err
	}
	var maxSequence int64
	if err := m.db.Table("messages").Where("conversation_id = ?", conversationID).Select("COALESCE(MAX(sequence), 0)").Scan(&maxSequence).Error; err != nil {
		return nil, 0, err
	}
	if maxSequence <= record.LastMessageSequence {
		return []map[string]string{}, record.LastMessageSequence, nil
	}
	startSequence := record.LastMessageSequence + 1 - int64(contextWindow)
	if startSequence < 1 {
		startSequence = 1
	}
	var rows []Message
	err = m.db.Table("messages").
		Select("sequence, role, content").
		Where("conversation_id = ? AND sequence BETWEEN ? AND ?", conversationID, startSequence, maxSequence).
		Order("sequence ASC").
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	messages := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, map[string]string{
			"role":    row.Role,
			"content": row.Content,
		})
	}
	return messages, maxSequence, nil
}
