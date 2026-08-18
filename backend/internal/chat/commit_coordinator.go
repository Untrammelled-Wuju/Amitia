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
type MessagePlanningHook func(event *MessagePlanningEvent) *MessagePlanningDecision

var messageCommitHooks []MessageCommitHook
var messagePlanningHook MessagePlanningHook

func RegisterMessageCommitHook(hook MessageCommitHook) {
	if hook != nil {
		messageCommitHooks = append(messageCommitHooks, hook)
	}
}

func RegisterMessagePlanningHook(hook MessagePlanningHook) {
	messagePlanningHook = hook
}

type MessagePlanningEvent struct {
	ConversationID string
	CharacterID    string
	Channel        string
	Source         string
	UserMessage    string
	Reply          string
	Lines          []string
	UserID         string
	PeerID         string
	RequestID      string
	ForceVoice     bool
}

type PlannedEmote struct {
	EmoteID     string
	Content     string
	AltText     string
	IsAnimated  int
	Width       int
	Height      int
	Original    string
	Fallback    string
	MimeType    string
	DeliveryKey string
}

type MessagePlanningDecision struct {
	Emote       *PlannedEmote
	InsertAfter int
	SendMode    string
	Persist     func(tx *gorm.DB, message *Message) error
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
	MessagePlan    *interaction.MessagePlan
	IsInternal     bool
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
	ForceVoice      bool
}

type messageCommitResult struct {
	CommitID          string
	MessageIDs        []string
	LastSequence      int64
	Events            []newoutbox.OutboxRecord
	StateVersions     map[string]int64
	DeliveryIntentIDs []string
	MessagePlan       *interaction.MessagePlan
}

func (s *service) commitInteraction(plan messageCommitPlan) (*messageCommitResult, error) {
	result := &messageCommitResult{}

	log.Printf("[commitInteraction] enter InteractionID=%s HasRuntime=%v ExpectedVersion=%d Lines=%d", plan.Request.InteractionID, plan.Request.Runtime != nil, plan.Request.ExpectedStatusVersion, len(plan.Lines))

	responseGroupID := plan.Request.RequestID
	if responseGroupID == "" {
		responseGroupID = plan.Request.InteractionID
	}
	if responseGroupID == "" {
		responseGroupID = uuid.New().String()
	}
	var planningDecision *MessagePlanningDecision
	if messagePlanningHook != nil {
		planningDecision = messagePlanningHook(&MessagePlanningEvent{
			ConversationID: plan.Conversation,
			CharacterID:    plan.Character,
			Channel:        plan.Request.Channel,
			Source:         plan.Source,
			UserMessage:    plan.Request.Message,
			Reply:          plan.Reply,
			Lines:          append([]string(nil), plan.Lines...),
			UserID:         plan.Request.UserID,
			PeerID:         plan.Request.PeerID,
			RequestID:      responseGroupID,
			ForceVoice:     plan.ForceVoice,
		})
	}
	emoteInsertAfter := len(plan.Lines)
	if planningDecision != nil && planningDecision.Emote != nil {
		if len(plan.Lines) == 0 {
			if planningDecision.SendMode == "emote_only" {
				emoteInsertAfter = 0
			} else {
				planningDecision = nil
			}
		} else if planningDecision.InsertAfter > 0 && planningDecision.InsertAfter < len(plan.Lines) {
			emoteInsertAfter = planningDecision.InsertAfter
		} else {
			planningDecision.InsertAfter = len(plan.Lines)
			planningDecision.SendMode = "after_all_text"
		}
	}

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
		items := make([]interaction.MessagePlanItem, 0, len(plan.Lines)+1)
		textIndex := 0
		totalItems := len(plan.Lines)
		if planningDecision != nil && planningDecision.Emote != nil {
			totalItems++
		}
		var emoteMessage *Message
		emoteInserted := false
		for sequence := 1; sequence <= totalItems; sequence++ {
			insertEmote := planningDecision != nil && planningDecision.Emote != nil && !emoteInserted && textIndex == emoteInsertAfter
			if insertEmote {
				planned := planningDecision.Emote
				status := "sent"
				if strings.ToLower(plan.Request.Channel) != "web" {
					status = "sending"
				}
				emoteMessage = &Message{ID: uuid.New().String(), ConversationID: plan.Conversation, Role: "assistant", Content: planned.Content, MsgType: "emote", Source: "ai_random", Status: status, ImageUrl: planned.Original, EmoteID: planned.EmoteID, AltText: planned.AltText, IsAnimated: planned.IsAnimated, MediaWidth: planned.Width, MediaHeight: planned.Height, OriginalAsset: planned.Original, FallbackAsset: planned.Fallback, ResponseGroupID: responseGroupID, DeliverySequence: sequence, EmoteDecisionStatus: "queued", RequestID: plan.Request.RequestID}
				if status == "sent" {
					emoteMessage.EmoteDecisionStatus = "sent"
				}
				if err := tx.Create(emoteMessage).Error; err != nil {
					return err
				}
				if err := s.recordMessageChangeTx(tx, emoteMessage, syncapi.OpCreate, 1, plan.Request.UserID); err != nil {
					return err
				}
				result.LastSequence = emoteMessage.Sequence
				items = append(items, interaction.MessagePlanItem{MessageID: emoteMessage.ID, Sequence: sequence, Type: "emote", Content: planned.Content, EmoteID: planned.EmoteID, AltText: planned.AltText, IsAnimated: planned.IsAnimated == 1, Width: planned.Width, Height: planned.Height, OriginalAssetReference: planned.Original, FallbackAssetReference: planned.Fallback})
				emoteInserted = true
				continue
			}
			text := plan.Lines[textIndex]
			aiMsgID := uuid.New().String()
			decisionStatus := "none"
			if planningDecision != nil && planningDecision.Emote != nil {
				decisionStatus = "selected"
			}
			aiMsg := &Message{ID: aiMsgID, ConversationID: plan.Conversation, Role: "assistant", Content: text, MsgType: "text", Source: plan.Source, Tokens: plan.TotalTokens, RequestID: plan.Request.RequestID, ResponseGroupID: responseGroupID, DeliverySequence: sequence, EmoteDecisionStatus: decisionStatus}
			if err := tx.Create(aiMsg).Error; err != nil {
				return err
			}
			if err := s.recordMessageChangeTx(tx, aiMsg, syncapi.OpCreate, 1, plan.Request.UserID); err != nil {
				return err
			}
			result.MessageIDs = append(result.MessageIDs, aiMsgID)
			result.LastSequence = aiMsg.Sequence
			items = append(items, interaction.MessagePlanItem{MessageID: aiMsgID, Sequence: sequence, Type: "text", Content: text})
			textIndex++
		}
		managed := strings.EqualFold(plan.Request.Channel, "web") || !plan.ForceVoice
		result.MessagePlan = &interaction.MessagePlan{ResponseGroupID: responseGroupID, Managed: managed, Items: items}
		if planningDecision != nil && planningDecision.Persist != nil {
			if err := planningDecision.Persist(tx, emoteMessage); err != nil {
				return err
			}
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
			ConversationID: plan.Conversation,
			CharacterID:    plan.Character,
			Channel:        plan.Request.Channel,
			Source:         plan.Source,
			MessageIDs:     result.MessageIDs,
			UserMessageID:  plan.UserMessageID,
			UserMessage:    plan.Request.Message,
			Reply:          plan.Reply,
			Lines:          plan.Lines,
			UserID:         plan.Request.UserID,
			PeerID:         plan.Request.PeerID,
			RequestID:      plan.Request.RequestID,
			MessagePlan:    result.MessagePlan,
			IsInternal:     plan.Request.IsInternal,
		}
		for _, hook := range messageCommitHooks {
			hook(event)
		}
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
	return s.relTimeCoordinator.FinalizeCommittedTx(context.Background(), tx, plan.Request.UserID, plan.Character, plan.Request.InteractionID, relTimeCtx, suppress, reason, plan.Request.IsInternal)
}
