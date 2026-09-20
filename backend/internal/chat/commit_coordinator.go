package chat

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/interaction"
	newoutbox "github.com/u-ai/backend/internal/outbox"
	syncapi "github.com/u-ai/backend/internal/sync"
	"gorm.io/gorm"
)

type MessageCommitHook func(event *MessageCommitEvent)

var messageCommitHooks []MessageCommitHook

func RegisterMessageCommitHook(hook MessageCommitHook) {
	if hook != nil {
		messageCommitHooks = append(messageCommitHooks, hook)
	}
}

type MessageCommitEvent struct {
	ConversationID      string
	TurnID              string
	CharacterID         string
	Channel             string
	Source              string
	MessageIDs          []string
	Sequences           map[string]int64
	UserMessageID       string
	UserMessageSequence int64
	UserMessage         string
	Reply               string
	Reasoning           string
	ReasoningDurationMS int64
	Lines               []string
	SpaceID             string
	PeerID              string
	RequestID           string
	MessagePlan         *interaction.MessagePlan
	IsInternal          bool
}
type messageCommitPlan struct {
	Request             *ProcessMessageRequest
	TurnID              string
	Conversation        string
	Character           string
	CharacterName       string
	UserMessageID       string
	Reply               string
	Reasoning           string
	ReasoningDurationMS int64
	Lines               []string
	Source              string
	Runtime             *interaction.RuntimeAssembly
	CommitToken         string
	CommitOwner         string
	LeaseID             string
	LeaseOwnerToken     string
	TotalTokens         int
	ForceVoice          bool
}

type messageCommitResult struct {
	CommitID          string
	TurnID            string
	MessageIDs        []string
	LastSequence      int64
	Events            []newoutbox.OutboxRecord
	StateVersions     map[string]int64
	DeliveryIntentIDs []string
	MessagePlan       *interaction.MessagePlan
}

func (s *service) commitInteraction(ctx context.Context, plan messageCommitPlan) (*messageCommitResult, error) {
	result := &messageCommitResult{}
	if plan.Request.SuppressReplyPersistence {
		plan.Lines = nil
	}

	log.Printf("[commitInteraction] enter InteractionID=%s HasRuntime=%v ExpectedVersion=%d Lines=%d", plan.Request.InteractionID, plan.Request.Runtime != nil, plan.Request.ExpectedStatusVersion, len(plan.Lines))

	responseGroupID := plan.Request.RequestID
	if responseGroupID == "" {
		responseGroupID = plan.Request.InteractionID
	}
	if responseGroupID == "" {
		responseGroupID = uuid.New().String()
	}
	var messageOutputs []MessageOutput
	if !plan.Request.SuppressReplyPersistence {
		outputs, outputErr := s.planMessageOutputs(ctx, plan)
		if outputErr != nil {
			log.Printf("[commitInteraction] message output planning failed: %v", outputErr)
		} else {
			messageOutputs = outputs
		}
	}

	if s.deliveryStore != nil && plan.Request != nil && plan.Request.InteractionID != "" {
		leaseID, ownerToken, err := s.deliveryStore.AcquireOutputLease(
			plan.Request.InteractionID, plan.Character, plan.Request.SpaceID, plan.Request.Channel)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire output lease: %w", err)
		}
		plan.LeaseID = leaseID
		plan.LeaseOwnerToken = ownerToken
	}

	messageSequences := make(map[string]int64)
	var userMessageSequence int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.acquireAndValidateCommitTokenTx(tx, &plan); err != nil {
			return err
		}
		items := make([]interaction.MessagePlanItem, 0, len(plan.Lines)+len(messageOutputs)+1)
		textIndex := 0
		totalItems := len(plan.Lines) + len(messageOutputs)
		if len(messageOutputs) > 0 {
			outputIndex := 0
			sequence := 1
			for outputIndex < len(messageOutputs) || textIndex < len(plan.Lines) {
				for outputIndex < len(messageOutputs) && messageOutputs[outputIndex].InsertAfter <= textIndex {
					output := messageOutputs[outputIndex]
					message := buildMessageFromOutput(plan, responseGroupID, sequence, output)
					if err := tx.Create(message).Error; err != nil {
						return err
					}
					if err := s.recordMessageChangeTx(tx, message, syncapi.OpCreate, 1, plan.Request.SpaceID); err != nil {
						return err
					}
					messageSequences[message.ID] = message.Sequence
					result.MessageIDs = append(result.MessageIDs, message.ID)
					result.LastSequence = message.Sequence
					items = append(items, interaction.MessagePlanItem{
						MessageID:              message.ID,
						Sequence:               sequence,
						Type:                   output.Part.Type,
						ExtensionType:          output.Part.ExtensionType,
						Content:                message.Content,
						AltText:                output.Part.AltText,
						MIMEType:               output.Part.MIMEType,
						IsAnimated:             output.Part.IsAnimated,
						Width:                  output.Part.Width,
						Height:                 output.Part.Height,
						OriginalAssetReference: output.Part.URL,
						FallbackAssetReference: output.Part.FallbackURL,
					})
					outputIndex++
					sequence++
				}
				if textIndex >= len(plan.Lines) {
					continue
				}
				text := plan.Lines[textIndex]
				aiMsgID := uuid.New().String()
				aiMsg := &Message{ID: aiMsgID, ConversationID: plan.Conversation, CharacterID: plan.Character, Role: "assistant", Content: text, MsgType: "text", Source: plan.Source, Tokens: plan.TotalTokens, RequestID: plan.Request.RequestID, ResponseGroupID: responseGroupID, DeliverySequence: sequence}
				if err := tx.Create(aiMsg).Error; err != nil {
					return err
				}
				if err := s.recordMessageChangeTx(tx, aiMsg, syncapi.OpCreate, 1, plan.Request.SpaceID); err != nil {
					return err
				}
				messageSequences[aiMsgID] = aiMsg.Sequence
				result.MessageIDs = append(result.MessageIDs, aiMsgID)
				result.LastSequence = aiMsg.Sequence
				items = append(items, interaction.MessagePlanItem{MessageID: aiMsgID, Sequence: sequence, Type: "text", Content: text})
				textIndex++
				sequence++
			}
		} else {
			for sequence := 1; sequence <= totalItems; sequence++ {
				text := plan.Lines[textIndex]
				aiMsgID := uuid.New().String()
				aiMsg := &Message{ID: aiMsgID, ConversationID: plan.Conversation, CharacterID: plan.Character, Role: "assistant", Content: text, MsgType: "text", Source: plan.Source, Tokens: plan.TotalTokens, RequestID: plan.Request.RequestID, ResponseGroupID: responseGroupID, DeliverySequence: sequence}
				if err := tx.Create(aiMsg).Error; err != nil {
					return err
				}
				if err := s.recordMessageChangeTx(tx, aiMsg, syncapi.OpCreate, 1, plan.Request.SpaceID); err != nil {
					return err
				}
				messageSequences[aiMsgID] = aiMsg.Sequence
				result.MessageIDs = append(result.MessageIDs, aiMsgID)
				result.LastSequence = aiMsg.Sequence
				items = append(items, interaction.MessagePlanItem{MessageID: aiMsgID, Sequence: sequence, Type: "text", Content: text})
				textIndex++
			}
		}
		managed := strings.EqualFold(plan.Request.Channel, "web") || !plan.ForceVoice
		result.MessagePlan = &interaction.MessagePlan{ResponseGroupID: responseGroupID, Managed: managed, Items: items}
		now := time.Now().Format("2006-01-02 15:04:05")
		if plan.UserMessageID != "" {
			if err := tx.Model(&Message{}).Where("id = ?", plan.UserMessageID).Select("sequence").Scan(&userMessageSequence).Error; err != nil {
				userMessageSequence = 0
			}
		}
		if err := tx.Model(&Message{}).Where("id = ?", plan.UserMessageID).Updates(map[string]interface{}{"status": "sent", "updated_at": now}).Error; err != nil {
			if plan.Source != "proactive" {
				return err
			}
		}
		if err := tx.Exec("UPDATE conversations SET updated_at = ?, message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?) WHERE id = ?", now, plan.Conversation, plan.Conversation).Error; err != nil {
			return err
		}
		if strings.TrimSpace(plan.Reasoning) != "" && len(result.MessageIDs) > 0 {
			var reasoningMessageID string
			if err := tx.Table("messages").
				Select("id").
				Where("id IN ? AND role = ?", result.MessageIDs, "assistant").
				Order("sequence ASC").
				Limit(1).
				Scan(&reasoningMessageID).Error; err != nil {
				return err
			}
			if reasoningMessageID != "" {
				if err := tx.Model(&Message{}).Where("id = ?", reasoningMessageID).Updates(map[string]interface{}{
					"reasoning_content":     plan.Reasoning,
					"reasoning_duration_ms": plan.ReasoningDurationMS,
				}).Error; err != nil {
					return err
				}
			}
		}
		if err := completeAssistantTurnTx(tx, plan.TurnID, responseGroupID, plan.Reply, result.MessageIDs); err != nil {
			return err
		}
		result.TurnID = plan.TurnID
		if err := s.commitAttachmentsTx(tx, plan, plan.UserMessageID); err != nil {
			return err
		}
		if shouldCommitRuntime(plan.Request) {
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
			if err := s.finalizeRelationshipTimeTx(tx, plan); err != nil {
				return err
			}
			events, deliveryIntentIDs, err := s.appendInteractionOutboxTx(tx, plan, result.MessageIDs, result.MessagePlan)
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
	if len(messageCommitHooks) > 0 {
		event := &MessageCommitEvent{
			ConversationID:      plan.Conversation,
			TurnID:              result.TurnID,
			CharacterID:         plan.Character,
			Channel:             plan.Request.Channel,
			Source:              plan.Source,
			MessageIDs:          result.MessageIDs,
			Sequences:           messageSequences,
			UserMessageID:       plan.UserMessageID,
			UserMessageSequence: userMessageSequence,
			UserMessage:         plan.Request.Message,
			Reply:               plan.Reply,
			Reasoning:           plan.Reasoning,
			ReasoningDurationMS: plan.ReasoningDurationMS,
			Lines:               plan.Lines,
			SpaceID:             plan.Request.SpaceID,
			PeerID:              plan.Request.PeerID,
			RequestID:           plan.Request.RequestID,
			MessagePlan:         result.MessagePlan,
			IsInternal:          plan.Request.IsInternal,
		}
		for _, hook := range messageCommitHooks {
			hook(event)
		}
	}
	if !plan.Request.IsInternal && plan.Source != "proactive" {
		s.trackUserAffectFromMessage(plan.Request.SpaceID, plan.Character, plan.Request.Message)
	}
	return result, nil
}

func (s *service) finalizeRelationshipTimeTx(tx *gorm.DB, plan messageCommitPlan) error {
	if s.relTimeCoordinator == nil || plan.Runtime == nil || plan.Runtime.Context.Temporal.Value.RelationshipTime == nil {
		return nil
	}
	relTimeCtx := plan.Runtime.Context.Temporal.Value.RelationshipTime
	suppress := false
	reason := ""
	if relTimeCtx.Policy != nil {
		if relTimeCtx.Policy.MentionMode == "none" || relTimeCtx.Policy.MaxMentionSentences == 0 {
			suppress = true
		}
		if relTimeCtx.Policy.SuppressionReason != "" {
			reason = relTimeCtx.Policy.SuppressionReason
		}
	}
	return s.relTimeCoordinator.FinalizeCommittedTx(context.Background(), tx, plan.Request.SpaceID, plan.Character, plan.Request.InteractionID, relTimeCtx, suppress, reason, plan.Request.IsInternal)
}
