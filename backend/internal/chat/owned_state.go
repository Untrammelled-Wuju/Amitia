package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/emotionstate"
	"github.com/u-ai/backend/internal/interaction"
	newoutbox "github.com/u-ai/backend/internal/outbox"
	"gorm.io/gorm"
)

type OwnedStatePlan struct {
	Scope         coordination.ExecutionScope
	SpaceID       string
	Character     string
	Conversation  string
	InteractionID string
	Channel       string
	PeerID        string
	Source        string
	IsInternal    bool
	Reply         string
	MessageID     string
	Runtime       *interaction.RuntimeAssembly
}

func (s *service) CommitOwnedState(ctx context.Context, plan OwnedStatePlan) error {
	actual, ok := coordination.FromContext(ctx)
	if !ok || actual != plan.Scope || plan.Character != actual.RoleID || actual.ResourceOwnerID != actual.RoleOwnerID || plan.SpaceID == "" || plan.Conversation == "" {
		return coordination.ErrWrongOwner
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	if plan.Runtime == nil {
		return nil
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	fingerprint := sha256.Sum256(encoded)
	identity := sha256.Sum256([]byte(actual.ResourceOwnerID + "\x00" + actual.RoleID + "\x00" + actual.RequestID))
	receiptID := "owned-state/" + hex.EncodeToString(identity[:])
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var receipt relationshipEventRecord
		err := tx.Where("id = ?", receiptID).Take(&receipt).Error
		if err == nil {
			if receipt.CharacterID != plan.Character || receipt.EventData != hex.EncodeToString(fingerprint[:]) {
				return coordination.ErrRequestConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := s.applyOwnedAppraisalResultTx(tx, plan); err != nil {
			return err
		}
		if err := s.updateOwnedPsycheStateTx(tx, plan); err != nil {
			return err
		}
		if err := s.updateOwnedRelationshipStateTx(tx, plan); err != nil {
			return err
		}
		if err := s.updateOwnedNeedStateTx(tx, plan); err != nil {
			return err
		}
		if relationshipTime := plan.Runtime.Context.Temporal.Value.RelationshipTime; relationshipTime != nil && s.relTimeCoordinator != nil {
			suppress, reason := false, ""
			if policy := relationshipTime.Policy; policy != nil {
				suppress = policy.MentionMode == "none" || policy.MaxMentionSentences == 0
				reason = policy.SuppressionReason
			}
			if err := s.relTimeCoordinator.FinalizeCommittedTx(ctx, tx, plan.SpaceID, plan.Character, plan.Scope.TurnID, relationshipTime, suppress, reason, plan.IsInternal); err != nil {
				return err
			}
		}
		if s.emotion != nil && !plan.IsInternal && plan.Source != "proactive" && plan.Runtime.Appraisal != nil {
			appraisal := plan.Runtime.Appraisal
			if _, err := emotionstate.NewService(tx).Commit(ctx, plan.SpaceID, plan.Character, inferUserAffect(appraisal), relationshipEmotionDeltaFromAppraisal(appraisal), emotionstate.Signals{}); err != nil {
				return err
			}
		}
		if err := appendOwnedStateEventsTx(tx, plan, receiptID); err != nil {
			return err
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return err
		}
		return tx.Create(&relationshipEventRecord{ID: receiptID, CharacterID: plan.Character, EventType: "owned_state_commit", EventData: hex.EncodeToString(fingerprint[:]), CreatedAt: time.Now().Format("2006-01-02 15:04:05")}).Error
	})
}

func appendOwnedStateEventsTx(tx *gorm.DB, plan OwnedStatePlan, receiptID string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	events := []struct {
		name    string
		payload map[string]any
	}{
		{"interaction.completed", map[string]any{"interactionId": plan.InteractionID, "conversationId": plan.Conversation, "characterId": plan.Character, "reply": plan.Reply, "messageIds": []string{plan.MessageID}, "delivery": plan.Runtime.Delivery}},
		{"interaction.state_changed", map[string]any{"interactionId": plan.InteractionID, "conversationId": plan.Conversation, "characterId": plan.Character, "channel": plan.Channel, "status": "committed", "timestamp": now}},
		{"interaction.runtime_assembled", map[string]any{"interactionId": plan.InteractionID, "path": plan.Runtime.Path, "safety": plan.Runtime.Safety, "delivery": plan.Runtime.Delivery, "transaction": plan.Runtime.Transaction.Name, "timestamp": now}},
	}
	for _, event := range events {
		payload, err := json.Marshal(event.payload)
		if err != nil {
			return err
		}
		record := newoutbox.OutboxRecordModel{ID: receiptID + "/" + event.name, AggregateID: plan.InteractionID, EventType: event.name, Payload: string(payload), Status: string(newoutbox.OutboxStatusPending), MaxRetries: newoutbox.DefaultMaxRetries, NextRetryAt: now, AvailableAt: now, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
	}
	return nil
}
