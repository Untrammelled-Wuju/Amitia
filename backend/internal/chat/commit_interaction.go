package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/interaction"
	"gorm.io/gorm"
)

func (s *service) acquireAndValidateCommitTokenTx(tx *gorm.DB, plan *messageCommitPlan) error {
	log.Printf("[acquireCommitToken] InteractionID=%s shouldCommitRuntime=%v", plan.Request.InteractionID, shouldCommitRuntime(plan.Request))
	if !shouldCommitRuntime(plan.Request) {
		return nil
	}
	var record interaction.InteractionRecordModel
	err := tx.Where("id = ?", plan.Request.InteractionID).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return interaction.ErrInteractionNotFound
	}
	if err != nil {
		return err
	}
	if record.StatusVersion != plan.Request.ExpectedStatusVersion {
		return interaction.ErrVersionConflict
	}
	if interaction.InteractionStatus(record.Status) != interaction.InteractionStatusContextReady {
		return fmt.Errorf("%w: stale status %s", interaction.ErrInvalidTransition, record.Status)
	}
	if record.SupersededByID != "" || !record.CancelRequestedAt.IsZero() {
		return interaction.ErrVersionConflict
	}
	token := uuid.New().String()
	owner := uuid.New().String()
	now := time.Now().UTC()
	res := tx.Model(&interaction.InteractionRecordModel{}).
		Where("id = ? AND status_version = ? AND status = ? AND (superseded_by_id = '' OR superseded_by_id IS NULL)",
			plan.Request.InteractionID, record.StatusVersion, string(interaction.InteractionStatusContextReady)).
		Updates(map[string]interface{}{
			"commit_token":       token,
			"commit_owner":       owner,
			"commit_acquired_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return interaction.ErrCommitTokenUnavailable
	}
	plan.CommitToken = token
	plan.CommitOwner = owner
	return nil
}

func (s *service) transitionInteractionCommittedTx(tx *gorm.DB, plan messageCommitPlan, result *messageCommitResult) error {
	log.Printf("[transitionCommitted] InteractionID=%s shouldCommitRuntime=%v", plan.Request.InteractionID, shouldCommitRuntime(plan.Request))
	if !shouldCommitRuntime(plan.Request) {
		return nil
	}
	commitID := uuid.New().String()
	result.CommitID = commitID
	msgIDsJSON, _ := json.Marshal(result.MessageIDs)
	deliveryIDsJSON, _ := json.Marshal(result.DeliveryIntentIDs)
	now := time.Now().UTC()
	res := tx.Model(&interaction.InteractionRecordModel{}).
		Where("id = ? AND status_version = ? AND status = ? AND commit_token = ? AND commit_owner = ?",
			plan.Request.InteractionID, plan.Request.ExpectedStatusVersion, string(interaction.InteractionStatusContextReady), plan.CommitToken, plan.CommitOwner).
		Updates(map[string]interface{}{
			"status":              string(interaction.InteractionStatusCommitted),
			"status_version":      plan.Request.ExpectedStatusVersion + 1,
			"commit_id":           commitID,
			"result_message_ids":  string(msgIDsJSON),
			"delivery_intent_ids": string(deliveryIDsJSON),
			"updated_at":          now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return fmt.Errorf("%w: interaction commit transition failed", interaction.ErrVersionConflict)
	}
	log.Printf("[transitionCommitted] success, status updated to committed InteractionID=%s", plan.Request.InteractionID)
	return nil
}

func shouldCommitRuntime(req *ProcessMessageRequest) bool {
	return req != nil && strings.TrimSpace(req.InteractionID) != "" && req.Runtime != nil
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

func userIDForRequest(req *ProcessMessageRequest) string {
	if req != nil && strings.TrimSpace(req.UserID) != "" {
		return req.UserID
	}
	return "default"
}

func channelForRequest(req *ProcessMessageRequest) string {
	if req != nil && strings.TrimSpace(req.Channel) != "" {
		return req.Channel
	}
	return "web"
}
