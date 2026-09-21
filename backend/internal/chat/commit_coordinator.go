package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/interaction"
	newoutbox "github.com/u-ai/backend/internal/outbox"
	syncapi "github.com/u-ai/backend/internal/sync"
	"gorm.io/gorm"
)

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
	TextMessageID     string
	LastSequence      int64
	Events            []newoutbox.OutboxRecord
	StateVersions     map[string]int64
	DeliveryIntentIDs []string
	MessagePlan       *interaction.MessagePlan
	TurnItems         []AssistantTurnItem
}

func (s *service) commitInteraction(ctx context.Context, plan messageCommitPlan) (*messageCommitResult, error) {
	result := &messageCommitResult{}
	suppressReply := plan.Request.SuppressReplyPersistence
	log.Printf("[commitInteraction] enter InteractionID=%s HasRuntime=%v ExpectedVersion=%d ReplyBytes=%d", plan.Request.InteractionID, plan.Request.Runtime != nil, plan.Request.ExpectedStatusVersion, len(plan.Reply))

	deliveryGroupID := strings.TrimSpace(plan.Request.RequestID)
	if deliveryGroupID == "" {
		deliveryGroupID = strings.TrimSpace(plan.Request.InteractionID)
	}
	if deliveryGroupID == "" {
		deliveryGroupID = uuid.NewString()
	}

	var messageOutputs []MessageOutput
	if !suppressReply {
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

		items := make([]interaction.MessagePlanItem, 0, len(messageOutputs)+1)
		deliverySequence := 1
		persistOutput := func(output MessageOutput) error {
			message := buildMessageFromOutput(plan, deliveryGroupID, deliverySequence, output)
			if err := tx.Create(message).Error; err != nil {
				return err
			}
			if err := s.recordMessageChangeTx(tx, message, syncapi.OpCreate, 1, plan.Request.SpaceID); err != nil {
				return err
			}
			messageSequences[message.ID] = message.Sequence
			result.MessageIDs = append(result.MessageIDs, message.ID)
			result.LastSequence = message.Sequence
			if strings.TrimSpace(plan.TurnID) != "" {
				encoded, err := json.Marshal(output.Part)
				if err != nil {
					return err
				}
				now := nowString()
				turnItem := AssistantTurnItem{
					ID: uuid.NewString(), TurnID: plan.TurnID, ConversationID: plan.Conversation, ItemType: output.Part.Type,
					Status: assistantTurnStatusCompleted, Revision: 2, Content: message.Content, ResultJSON: string(encoded),
					MessageID: message.ID, CreatedAt: now, UpdatedAt: now,
				}
				if err := appendAssistantTurnItemTx(tx, &turnItem); err != nil {
					return err
				}
				result.TurnItems = append(result.TurnItems, turnItem)
			}
			items = append(items, interaction.MessagePlanItem{
				MessageID: message.ID, Sequence: deliverySequence, Type: output.Part.Type,
				ExtensionType: output.Part.ExtensionType, Content: message.Content, AltText: output.Part.AltText,
				MIMEType: output.Part.MIMEType, IsAnimated: output.Part.IsAnimated, Width: output.Part.Width, Height: output.Part.Height,
				OriginalAssetReference: output.Part.URL, FallbackAssetReference: output.Part.FallbackURL,
			})
			deliverySequence++
			return nil
		}

		outputIndex := 0
		for outputIndex < len(messageOutputs) && messageOutputs[outputIndex].Placement == "before_text" {
			if err := persistOutput(messageOutputs[outputIndex]); err != nil {
				return err
			}
			outputIndex++
		}

		if !suppressReply && strings.TrimSpace(plan.Reply) != "" {
			aiMsg := &Message{
				ID: uuid.NewString(), ConversationID: plan.Conversation, CharacterID: plan.Character,
				Role: "assistant", Content: plan.Reply, MsgType: "text", Source: plan.Source,
				Tokens: plan.TotalTokens, RequestID: plan.Request.RequestID,
				DeliveryGroupID: deliveryGroupID, DeliverySequence: deliverySequence,
			}
			if err := tx.Create(aiMsg).Error; err != nil {
				return err
			}
			if err := s.recordMessageChangeTx(tx, aiMsg, syncapi.OpCreate, 1, plan.Request.SpaceID); err != nil {
				return err
			}
			result.TextMessageID = aiMsg.ID
			messageSequences[aiMsg.ID] = aiMsg.Sequence
			result.MessageIDs = append(result.MessageIDs, aiMsg.ID)
			result.LastSequence = aiMsg.Sequence
			items = append(items, interaction.MessagePlanItem{MessageID: aiMsg.ID, Sequence: deliverySequence, Type: "text", Content: plan.Reply})
			deliverySequence++
		}

		for outputIndex < len(messageOutputs) {
			if err := persistOutput(messageOutputs[outputIndex]); err != nil {
				return err
			}
			outputIndex++
		}

		managed := strings.EqualFold(plan.Request.Channel, "web") || !plan.ForceVoice
		result.MessagePlan = &interaction.MessagePlan{DeliveryGroupID: deliveryGroupID, Managed: managed, Items: items}
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
		if strings.TrimSpace(plan.Reasoning) != "" && result.TextMessageID != "" {
			if err := tx.Model(&Message{}).Where("id = ?", result.TextMessageID).Updates(map[string]interface{}{
				"reasoning_content": plan.Reasoning, "reasoning_duration_ms": plan.ReasoningDurationMS,
			}).Error; err != nil {
				return err
			}
		}
		if err := completeAssistantTurnTx(tx, plan.TurnID, plan.Reply, result.TextMessageID); err != nil {
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

	if result.TurnID != "" {
		var turn AssistantTurn
		if err := s.db.Where("id = ?", result.TurnID).First(&turn).Error; err == nil {
			recorder := &assistantTurnRecorder{
				db: s.db, TurnID: turn.ID, ConversationID: turn.ConversationID, CharacterID: turn.CharacterID,
				UserMessageID: turn.UserMessageID, RequestID: turn.RequestID, ExecutionID: turn.ExecutionID,
				TurnSequence: turn.Sequence, enabled: true,
			}
			for _, item := range result.TurnItems {
				if publishErr := recorder.publishItemEvents(context.Background(), item); publishErr != nil {
					log.Printf("[commitInteraction] publish rich block event failed: %v", publishErr)
				}
			}
			var finalBlock AssistantTurnItem
			if err := s.db.Where("turn_id = ? AND item_type = ? AND is_final = 1", result.TurnID, assistantTurnItemText).Order("sequence DESC").First(&finalBlock).Error; err == nil {
				messageID := finalBlock.MessageID
				if _, publishErr := conversationstream.DefaultManager().Publish(context.Background(), conversationstream.AgentUIEvent{
					ConversationID: plan.Conversation, RequestID: plan.Request.RequestID, ExecutionID: turn.ExecutionID,
					TurnID: turn.ID, TurnSequence: turn.Sequence, BlockID: finalBlock.ID, BlockSequence: finalBlock.Sequence,
					MessageID: messageID, MessageSequence: messageSequences[messageID], Revision: finalBlock.Revision,
					Type: "text.completed", Status: assistantTurnStatusCompleted,
					Payload: map[string]any{"blockType": "text", "content": finalBlock.Content, "recoveryCheckpoint": true},
				}, true); publishErr != nil {
					log.Printf("[commitInteraction] publish final block event failed: %v", publishErr)
				}
			}
			terminal := conversationstream.AgentUIEvent{
				ConversationID: plan.Conversation, RequestID: plan.Request.RequestID, ExecutionID: turn.ExecutionID,
				TurnID: turn.ID, TurnSequence: turn.Sequence, Type: "turn.completed", Status: assistantTurnStatusCompleted,
				Payload: map[string]any{"messageIds": result.MessageIDs, "recoveryCheckpoint": true},
			}
			if result.TextMessageID != "" {
				terminal.MessageID = result.TextMessageID
				terminal.MessageSequence = messageSequences[result.TextMessageID]
			}
			if _, publishErr := conversationstream.DefaultManager().Publish(context.Background(), terminal, true); publishErr != nil {
				log.Printf("[commitInteraction] publish terminal turn event failed: %v", publishErr)
			}
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
