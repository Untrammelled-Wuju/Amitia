package chat

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/interaction"
	"gorm.io/gorm"
)

type MessageCommitHook func(event *MessageCommitEvent)

var onMessageCommitted MessageCommitHook

func RegisterMessageCommitHook(hook MessageCommitHook) {
	onMessageCommitted = hook
}

type MessageCommitEvent struct {
	ConversationID string
	CharacterID    string
	Channel        string
	Source         string
	MessageIDs     []string
	UserMessageID  string
	UserMessage    string
	Reply          string
	Lines          []string
	UserID         string
	PeerID         string
	RequestID      string
}

type messageCommitPlan struct {
	Request         *ProcessMessageRequest
	Conversation    string
	Character       string
	CharacterName   string
	UserMessageID   string
	Reply           string
	Lines           []string
	Source          string
	Runtime         *interaction.RuntimeAssembly
	CommitToken     string
	CommitOwner     string
	LeaseID         string
	LeaseOwnerToken string
	TotalTokens     int
}

type messageCommitResult struct {
	CommitID          string
	MessageIDs        []string
	LastSequence      int64
	Events            []interaction.OutboxRecord
	StateVersions     map[string]int64
	DeliveryIntentIDs []string
}

func (s *service) commitInteraction(plan messageCommitPlan) (*messageCommitResult, error) {
	result := &messageCommitResult{}

	log.Printf("[commitInteraction] enter InteractionID=%s HasRuntime=%v ExpectedVersion=%d Lines=%d", plan.Request.InteractionID, plan.Request.Runtime != nil, plan.Request.ExpectedStatusVersion, len(plan.Lines))

	if s.deliveryStore != nil && plan.Request != nil && plan.Request.InteractionID != "" {
		leaseID, ownerToken, err := s.deliveryStore.AcquireOutputLease(
			plan.Request.InteractionID, plan.Character, plan.Request.UserID, plan.Request.Channel)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire output lease: %w", err)
		}
		plan.LeaseID = leaseID
		plan.LeaseOwnerToken = ownerToken
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.acquireAndValidateCommitTokenTx(tx, &plan); err != nil {
			return err
		}
		for _, text := range plan.Lines {
			aiMsgID := uuid.New().String()
			aiMsg := &Message{ID: aiMsgID, ConversationID: plan.Conversation, Role: "assistant", Content: text, MsgType: "text", Source: plan.Source, Tokens: plan.TotalTokens, RequestID: plan.Request.RequestID}
			if err := tx.Create(aiMsg).Error; err != nil {
				return err
			}
			result.MessageIDs = append(result.MessageIDs, aiMsgID)
			result.LastSequence = aiMsg.Sequence
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		if err := tx.Model(&Message{}).Where("id = ?", plan.UserMessageID).Updates(map[string]interface{}{"status": "sent", "updated_at": now}).Error; err != nil {
			if plan.Source != "proactive" {
				return err
			}
		}
		if err := tx.Exec("UPDATE conversations SET updated_at = ?, message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?) WHERE id = ?", now, plan.Conversation, plan.Conversation).Error; err != nil {
			return err
		}
		if shouldCommitRuntime(plan.Request) {
			if !plan.Request.IsInternal {
				if plan.Runtime != nil && plan.Runtime.Appraisal != nil {
					if err := s.applyAppraisalResultTx(tx, plan); err != nil {
						return err
					}
				}
				if err := s.updatePsycheStateTx(tx, plan); err != nil {
					return err
				}
				if err := s.updateRelationshipStateTx(tx, plan); err != nil {
					return err
				}
				if err := s.updateNeedStateTx(tx, plan); err != nil {
					return err
				}
			}
			events, deliveryIntentIDs, err := s.appendInteractionOutboxTx(tx, plan, result.MessageIDs)
			if err != nil {
				return err
			}
			result.Events = events
			result.DeliveryIntentIDs = deliveryIntentIDs
		}
		if err := s.transitionInteractionCommittedTx(tx, plan, result); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if plan.LeaseID != "" {
			_ = s.deliveryStore.ReleaseOutputLease(plan.LeaseID, plan.LeaseOwnerToken)
		}
		return nil, err
	}

	if plan.LeaseID != "" {
		_ = s.deliveryStore.ReleaseOutputLease(plan.LeaseID, plan.LeaseOwnerToken)
	}
	if onMessageCommitted != nil {
		event := MessageCommitEvent{
			ConversationID: plan.Conversation,
			CharacterID:    plan.Character,
			Channel:        plan.Request.Channel,
			Source:         plan.Source,
			MessageIDs:     result.MessageIDs,
			UserMessageID:  plan.UserMessageID,
			UserMessage:    plan.Request.Message,
			Reply:          plan.Reply,
			Lines:          plan.Lines,
		}
		onMessageCommitted(&event)
	}
	return result, nil
}
