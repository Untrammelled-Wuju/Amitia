package chat

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/interaction"
	newoutbox "github.com/u-ai/backend/internal/outbox"
	"gorm.io/gorm"
)

func (s *service) appendInteractionOutboxTx(tx *gorm.DB, plan messageCommitPlan, messageIDs []string, messagePlan *interaction.MessagePlan) ([]interaction.OutboxRecord, []string, error) {
	now := time.Now()
	deliveryIntentIDs := []string{}
	events := []interaction.OutboxRecord{
		newInteractionOutboxRecord(plan.Request.InteractionID, "interaction.completed", map[string]interface{}{
			"interactionId":  plan.Request.InteractionID,
			"conversationId": plan.Conversation,
			"characterId":    plan.Character,
			"reply":          plan.Reply,
			"messageIds":     messageIDs,
			"messagePlan":    messagePlan,
			"delivery":       plan.Runtime.Delivery,
		}, now),
		newInteractionOutboxRecord(plan.Request.InteractionID, "interaction.state_changed", map[string]interface{}{
			"interactionId":  plan.Request.InteractionID,
			"conversationId": plan.Conversation,
			"characterId":    plan.Character,
			"channel":        plan.Request.Channel,
			"status":         "committed",
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
		model := newoutbox.OutboxRecordModel{
			ID:          event.ID,
			AggregateID: event.AggregateID,
			EventType:   event.EventType,
			Payload:     string(event.Payload),
			Status:      string(newoutbox.OutboxStatusPending),
			MaxRetries:  newoutbox.DefaultMaxRetries,
			NextRetryAt: now.Format("2006-01-02 15:04:05"),
			AvailableAt: now.Format("2006-01-02 15:04:05"),
			CreatedAt:   now.Format("2006-01-02 15:04:05"),
			UpdatedAt:   now.Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(&model).Error; err != nil {
			return nil, nil, err
		}
	}
	if messagePlan != nil && messagePlan.Managed {
		var diErr error
		deliveryIntentIDs, diErr = createDeliveryIntentsInTx(tx, plan, messagePlan, now)
		if diErr != nil {
			return nil, nil, diErr
		}
	}
	return events, deliveryIntentIDs, nil
}

func createDeliveryIntentsInTx(tx *gorm.DB, plan messageCommitPlan, messagePlan *interaction.MessagePlan, now time.Time) ([]string, error) {
	ids := []string{}
	channel := plan.Request.Channel
	if plan.Runtime != nil && plan.Runtime.Delivery.Channel != "" {
		channel = plan.Runtime.Delivery.Channel
	}
	if strings.ToLower(channel) == "web" {
		return ids, nil
	}
	if strings.TrimSpace(channel) == "" {
		return ids, nil
	}
	peerID := ""
	if plan.Runtime != nil && plan.Runtime.Delivery.PeerID != "" {
		peerID = plan.Runtime.Delivery.PeerID
	} else if plan.Request.PeerID != "" {
		peerID = plan.Request.PeerID
	}
	for _, item := range messagePlan.Items {
		stableID := delivery.GenerateDeliveryID(messagePlan.ResponseGroupID, channel, peerID, item.MessageID)
		payloadData := map[string]interface{}{
			"messageId":      item.MessageID,
			"conversationId": plan.Conversation,
			"characterId":    plan.Character,
			"content":        item.Content,
		}
		maxRetries := 5
		if item.Type == "emote" {
			payloadData["emoteId"] = item.EmoteID
			payloadData["altText"] = item.AltText
			payloadData["originalPath"] = item.OriginalAssetReference
			payloadData["fallbackPath"] = item.FallbackAssetReference
			payloadData["isAnimated"] = item.IsAnimated
			maxRetries = 3
		}
		payload, _ := json.Marshal(payloadData)
		result := tx.Exec("INSERT OR IGNORE INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, response_group_id, delivery_sequence) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			stableID, plan.Request.InteractionID, channel, peerID, item.Type, string(payload), "pending", now.Format("2006-01-02 15:04:05"), maxRetries, messagePlan.ResponseGroupID, item.Sequence)
		if result.Error != nil {
			return ids, result.Error
		}
		if result.RowsAffected != 1 {
			continue
		}
		ids = append(ids, stableID)
	}
	return ids, nil
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
