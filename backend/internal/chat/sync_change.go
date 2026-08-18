package chat

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/requestidentity"
	syncapi "github.com/u-ai/backend/internal/sync"
	"gorm.io/gorm"
)

func normalizeChangeUserID(userID string) string {
	return requestidentity.NormalizeUserID(userID)
}

func newBusinessMutationID(entityType syncapi.EntityType, entityID string, op syncapi.OperationType) syncapi.MutationID {
	return syncapi.MutationID("business_" + string(entityType) + "_" + entityID + "_" + string(op) + "_" + uuid.NewString())
}

func (s *service) recordConversationChangeTx(tx *gorm.DB, c *Conversation, op syncapi.OperationType, revision int64, userID string) error {
	if s.changeRecorder == nil || c == nil {
		return nil
	}
	payload, err := json.Marshal(map[string]interface{}{
		"id":          c.ID,
		"characterId": c.CharacterID,
		"title":       c.Title,
		"channel":     c.Channel,
		"source":      c.Source,
		"peerId":      c.PeerID,
	})
	if err != nil {
		return err
	}
	_, err = s.changeRecorder.RecordChange(tx, syncapi.EntityTypeConversation, syncapi.EntityID(c.ID), op, revision, newBusinessMutationID(syncapi.EntityTypeConversation, c.ID, op), normalizeChangeUserID(userID), syncapi.ScopeDevice, payload)
	return err
}

func (s *service) recordMessageChangeTx(tx *gorm.DB, m *Message, op syncapi.OperationType, revision int64, userID string) error {
	if s.changeRecorder == nil || m == nil {
		return nil
	}
	payload, err := json.Marshal(map[string]interface{}{
		"id":             m.ID,
		"conversationId": m.ConversationID,
		"role":           m.Role,
		"content":        m.Content,
		"sequence":       m.Sequence,
		"msgType":        m.MsgType,
		"source":         m.Source,
	})
	if err != nil {
		return err
	}
	_, err = s.changeRecorder.RecordChange(tx, syncapi.EntityTypeMessage, syncapi.EntityID(m.ID), op, revision, newBusinessMutationID(syncapi.EntityTypeMessage, m.ID, op), normalizeChangeUserID(userID), syncapi.ScopeDevice, payload)
	return err
}

func (s *service) persistConversationWithChange(c *Conversation, userID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(c).Error; err != nil {
			return err
		}
		return s.recordConversationChangeTx(tx, c, syncapi.OpCreate, 1, userID)
	})
}

func (s *service) persistMessageWithChange(m *Message, userID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		return s.recordMessageChangeTx(tx, m, syncapi.OpCreate, 1, userID)
	})
}
