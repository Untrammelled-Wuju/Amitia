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
	ProcessingStartSeq  int64  `gorm:"column:processing_start_sequence"`
	ProcessingEndSeq    int64  `gorm:"column:processing_end_sequence"`
	LeaseOwner          string `gorm:"column:lease_owner"`
	LeaseExpiresAt      string `gorm:"column:lease_expires_at"`
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

const timeLayout = "2006-01-02 15:04:05"

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
	if err := m.ensureLeaseColumns(); err != nil {
		return err
	}
	now := time.Now().UTC().Format(timeLayout)
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
				"last_message_sequence":     lastSequence,
				"idempotency_key":           idempotencyKey,
				"checkpoint_version":        1,
				"processing_start_sequence": 0,
				"processing_end_sequence":   0,
				"lease_owner":               "",
				"lease_expires_at":          "",
				"updated_at":                now,
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

func (m *Manager) AdvanceLeased(conversationID, pipelineType string, lastSequence int64, idempotencyKey string, leaseOwner string) error {
	if m.db == nil {
		return fmt.Errorf("checkpoint db is nil")
	}
	if err := m.ensureLeaseColumns(); err != nil {
		return err
	}
	now := time.Now().UTC().Format(timeLayout)
	return m.db.Transaction(func(tx *gorm.DB) error {
		var current Record
		err := tx.Where("conversation_id = ? AND pipeline_type = ?", conversationID, pipelineType).First(&current).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == nil {
			if current.LeaseOwner != "" && current.LeaseOwner != leaseOwner {
				return fmt.Errorf("checkpoint lease owner mismatch")
			}
			if current.LastMessageSequence > lastSequence {
				lastSequence = current.LastMessageSequence
			}
			return tx.Model(&Record{}).Where("conversation_id = ? AND pipeline_type = ?", conversationID, pipelineType).Updates(map[string]interface{}{
				"last_message_sequence":     lastSequence,
				"idempotency_key":           idempotencyKey,
				"checkpoint_version":        1,
				"processing_start_sequence": 0,
				"processing_end_sequence":   0,
				"lease_owner":               "",
				"lease_expires_at":          "",
				"updated_at":                now,
			}).Error
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

func (m *Manager) AcquirePendingRange(conversationID, pipelineType string, contextWindow int, leaseOwner string, leaseTTL time.Duration) ([]map[string]string, int64, bool, error) {
	if m.db == nil {
		return nil, 0, false, fmt.Errorf("checkpoint db is nil")
	}
	if err := m.ensureLeaseColumns(); err != nil {
		return nil, 0, false, err
	}
	if leaseTTL <= 0 {
		leaseTTL = 5 * time.Minute
	}
	if leaseOwner == "" {
		leaseOwner = "unknown"
	}
	now := time.Now().UTC()
	nowText := now.Format(timeLayout)
	leaseExpiresAt := now.Add(leaseTTL).Format(timeLayout)

	var messages []map[string]string
	var maxSequence int64
	acquired := false
	err := m.db.Transaction(func(tx *gorm.DB) error {
		var current Record
		err := tx.Where("conversation_id = ? AND pipeline_type = ?", conversationID, pipelineType).First(&current).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == gorm.ErrRecordNotFound {
			current = Record{
				ConversationID:      conversationID,
				PipelineType:        pipelineType,
				CheckpointVersion:   1,
				LastMessageSequence: 0,
				CreatedAt:           nowText,
			}
		}
		if current.LeaseExpiresAt != "" {
			expiresAt, parseErr := time.ParseInLocation(timeLayout, current.LeaseExpiresAt, time.UTC)
			if parseErr == nil && expiresAt.After(now) {
				maxSequence = current.LastMessageSequence
				return nil
			}
		}
		if err := tx.Table("messages").Where("conversation_id = ?", conversationID).Select("COALESCE(MAX(sequence), 0)").Scan(&maxSequence).Error; err != nil {
			return err
		}
		if maxSequence <= current.LastMessageSequence {
			return nil
		}
		startSequence := current.LastMessageSequence + 1 - int64(contextWindow)
		if startSequence < 1 {
			startSequence = 1
		}
		var rows []Message
		if err := tx.Table("messages").
			Select("sequence, role, content").
			Where("conversation_id = ? AND sequence BETWEEN ? AND ?", conversationID, startSequence, maxSequence).
			Order("sequence ASC").
			Find(&rows).Error; err != nil {
			return err
		}
		messages = make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			messages = append(messages, map[string]string{
				"role":    row.Role,
				"content": row.Content,
			})
		}
		updates := map[string]interface{}{
			"last_message_sequence":     current.LastMessageSequence,
			"checkpoint_version":        1,
			"processing_start_sequence": startSequence,
			"processing_end_sequence":   maxSequence,
			"lease_owner":               leaseOwner,
			"lease_expires_at":          leaseExpiresAt,
			"updated_at":                nowText,
		}
		if err == gorm.ErrRecordNotFound {
			current.ProcessingStartSeq = startSequence
			current.ProcessingEndSeq = maxSequence
			current.LeaseOwner = leaseOwner
			current.LeaseExpiresAt = leaseExpiresAt
			current.UpdatedAt = nowText
			if err := tx.Create(&current).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&Record{}).Where("conversation_id = ? AND pipeline_type = ?", conversationID, pipelineType).Updates(updates).Error; err != nil {
			return err
		}
		acquired = true
		return nil
	})
	if err != nil {
		return nil, 0, false, err
	}
	return messages, maxSequence, acquired, nil
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

func (m *Manager) ensureLeaseColumns() error {
	if m.db == nil {
		return fmt.Errorf("checkpoint db is nil")
	}
	migrator := m.db.Migrator()
	fields := []string{"ProcessingStartSeq", "ProcessingEndSeq", "LeaseOwner", "LeaseExpiresAt"}
	for _, field := range fields {
		if migrator.HasColumn(&Record{}, field) {
			continue
		}
		if err := migrator.AddColumn(&Record{}, field); err != nil {
			return err
		}
	}
	return nil
}
