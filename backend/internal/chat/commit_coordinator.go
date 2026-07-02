package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/psyche"
	"gorm.io/gorm"
)

type relationshipStateRecord struct {
	ID           string `gorm:"primaryKey;column:id"`
	CharacterID  string `gorm:"column:character_id"`
	RelationType string `gorm:"column:relation_type"`
	RelationData string `gorm:"column:relation_data"`
	CreatedAt    string `gorm:"column:created_at"`
	UpdatedAt    string `gorm:"column:updated_at"`
}

func (relationshipStateRecord) TableName() string {
	return "relationship_states"
}

type relationshipEventRecord struct {
	ID          string `gorm:"primaryKey;column:id"`
	CharacterID string `gorm:"column:character_id"`
	EventType   string `gorm:"column:event_type"`
	EventData   string `gorm:"column:event_data"`
	CreatedAt   string `gorm:"column:created_at"`
}

func (relationshipEventRecord) TableName() string {
	return "relationship_events"
}

type messageCommitPlan struct {
	Request       *ProcessMessageRequest
	Conversation  string
	Character     string
	CharacterName string
	UserMessageID string
	Reply         string
	Lines         []string
	Source        string
	Runtime       *interaction.RuntimeAssembly
}

type messageCommitResult struct {
	MessageIDs []string
	Events     []interaction.OutboxRecord
}

func (s *service) commitInteraction(plan messageCommitPlan) (*messageCommitResult, error) {
	result := &messageCommitResult{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, text := range plan.Lines {
			aiMsgID := uuid.New().String()
			if err := tx.Create(&Message{ID: aiMsgID, ConversationID: plan.Conversation, Role: "assistant", Content: text, MsgType: "text", Source: plan.Source, RequestID: plan.Request.RequestID}).Error; err != nil {
				return err
			}
			result.MessageIDs = append(result.MessageIDs, aiMsgID)
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		if err := tx.Model(&Message{}).Where("id = ?", plan.UserMessageID).Updates(map[string]interface{}{"status": "sent", "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE conversations SET updated_at = ?, message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?) WHERE id = ?", now, plan.Conversation, plan.Conversation).Error; err != nil {
			return err
		}
		if err := s.updatePsycheStateTx(tx, plan.Character); err != nil {
			return err
		}
		if shouldCommitRuntime(plan.Request) {
			if err := s.updateRelationshipStateTx(tx, plan); err != nil {
				return err
			}
			events, err := s.appendInteractionOutboxTx(tx, plan, result.MessageIDs)
			if err != nil {
				return err
			}
			result.Events = events
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func shouldCommitRuntime(req *ProcessMessageRequest) bool {
	return req != nil && strings.TrimSpace(req.InteractionID) != "" && req.Runtime != nil
}

func (s *service) updatePsycheStateTx(tx *gorm.DB, charID string) error {
	if s.psycheStore == nil || charID == "" {
		return nil
	}
	store := s.psycheStore
	if sqliteStore, ok := s.psycheStore.(*psyche.SQLitePsycheStore); ok {
		store = sqliteStore.WithDB(tx)
	}
	return s.updatePsycheStateWithStore(store, charID)
}

func (s *service) updatePsycheStateWithStore(store psyche.PsycheStore, charID string) error {
	for attempt := 0; attempt < 3; attempt++ {
		state, err := store.LoadState(charID)
		if err != nil {
			if !errors.Is(err, psyche.ErrStateNotFound) {
				return err
			}
			initial := psyche.NewPsycheState(charID)
			if err := store.SaveState(&initial); err != nil {
				if errors.Is(err, psyche.ErrVersionConflict) {
					continue
				}
				return err
			}
			state = &initial
		}
		event := psyche.PsycheEvent{
			ID:          uuid.New().String(),
			CharacterID: charID,
			Type:        psyche.EventTypeInteraction,
			Source:      "chat.process_message",
			EnergyDelta: -0.01,
			Timestamp:   time.Now().UTC(),
		}
		newState := psyche.ApplyEvent(*state, event)
		if err := store.SaveState(&newState); err != nil {
			if errors.Is(err, psyche.ErrVersionConflict) {
				continue
			}
			return err
		}
		if err := store.AppendEvent(&event); err != nil {
			return err
		}
		snapshot := psyche.CreateSnapshot(newState)
		if err := store.SaveSnapshot(&snapshot); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("%w: character %s retry exhausted", psyche.ErrVersionConflict, charID)
}

func (s *service) updateRelationshipStateTx(tx *gorm.DB, plan messageCommitPlan) error {
	relationType := relationshipTypeForRequest(plan.Request)
	now := time.Now().Format("2006-01-02 15:04:05")
	var existing relationshipStateRecord
	err := tx.Where("character_id = ? AND relation_type = ?", plan.Character, relationType).Order("updated_at DESC").Take(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	data := map[string]float64{
		"trust":            0.5,
		"familiarity":      0,
		"security":         0.5,
		"tension":          0,
		"repairConfidence": 0.5,
		"boundary":         0.5,
	}
	if existing.RelationData != "" {
		_ = json.Unmarshal([]byte(existing.RelationData), &data)
	}
	data["familiarity"] = clampRelationshipValue(data["familiarity"] + 0.01)
	data["trust"] = clampRelationshipValue(data["trust"] + 0.002)
	data["security"] = clampRelationshipValue(data["security"] + 0.001)
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if existing.ID == "" {
		existing = relationshipStateRecord{
			ID:           uuid.New().String(),
			CharacterID:  plan.Character,
			RelationType: relationType,
			CreatedAt:    now,
		}
	}
	existing.RelationData = string(raw)
	existing.UpdatedAt = now
	if err := tx.Save(&existing).Error; err != nil {
		return err
	}
	eventData, err := json.Marshal(map[string]interface{}{
		"interactionId":  plan.Request.InteractionID,
		"conversationId": plan.Conversation,
		"requestId":      plan.Request.RequestID,
		"relationType":   relationType,
		"delta": map[string]float64{
			"familiarity": 0.01,
			"trust":       0.002,
			"security":    0.001,
		},
	})
	if err != nil {
		return err
	}
	return tx.Create(&relationshipEventRecord{
		ID:          uuid.New().String(),
		CharacterID: plan.Character,
		EventType:   "interaction",
		EventData:   string(eventData),
		CreatedAt:   now,
	}).Error
}

func relationshipTypeForRequest(req *ProcessMessageRequest) string {
	if req == nil {
		return "channel:unknown"
	}
	if peer := strings.TrimSpace(req.PeerID); peer != "" {
		return "peer:" + peer
	}
	if channel := strings.TrimSpace(req.Channel); channel != "" {
		return "channel:" + channel
	}
	return "channel:web"
}

func clampRelationshipValue(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func (s *service) appendInteractionOutboxTx(tx *gorm.DB, plan messageCommitPlan, messageIDs []string) ([]interaction.OutboxRecord, error) {
	now := time.Now()
	events := []interaction.OutboxRecord{
		newInteractionOutboxRecord(plan.Request.InteractionID, "interaction.completed", map[string]interface{}{
			"interactionId":  plan.Request.InteractionID,
			"conversationId": plan.Conversation,
			"characterId":    plan.Character,
			"reply":          plan.Reply,
			"messageIds":     messageIDs,
			"delivery":       plan.Runtime.Delivery,
		}, now),
		newInteractionOutboxRecord(plan.Request.InteractionID, "interaction.state_changed", map[string]interface{}{
			"interactionId":  plan.Request.InteractionID,
			"conversationId": plan.Conversation,
			"characterId":    plan.Character,
			"channel":        plan.Request.Channel,
			"status":         "completed",
			"timestamp":      now,
		}, now),
		newInteractionOutboxRecord(plan.Request.InteractionID, "interaction.runtime_assembled", map[string]interface{}{
			"interactionId": plan.Request.InteractionID,
			"path":          plan.Runtime.Path,
			"safety":        plan.Runtime.Safety,
			"delivery":      plan.Runtime.Delivery,
			"transaction":   plan.Runtime.Transaction.Name,
			"timestamp":     now,
		}, now),
	}
	for _, event := range events {
		model := interaction.OutboxRecordModel{
			ID:          event.ID,
			AggregateID: event.AggregateID,
			EventType:   event.EventType,
			Payload:     string(event.Payload),
			Status:      string(event.Status),
			RetryCount:  event.RetryCount,
			MaxRetries:  event.MaxRetries,
			NextRetryAt: event.NextRetryAt,
			LastError:   event.LastError,
			CreatedAt:   event.CreatedAt,
		}
		if err := tx.Create(&model).Error; err != nil {
			return nil, err
		}
	}
	return events, nil
}

func newInteractionOutboxRecord(aggregateID, eventType string, payload map[string]interface{}, now time.Time) interaction.OutboxRecord {
	raw, _ := json.Marshal(payload)
	return interaction.OutboxRecord{
		ID:          uuid.New().String(),
		AggregateID: aggregateID,
		EventType:   eventType,
		Payload:     raw,
		Status:      interaction.OutboxStatusPending,
		MaxRetries:  interaction.DefaultMaxRetries,
		CreatedAt:   now,
		NextRetryAt: now,
	}
}
