package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func NewScheduleSessionPromptInvokeFunc(appender ConversationMessageAppender, creator ConversationCreator) schedule.SessionPromptInvokeFunc {
	return func(ctx context.Context, sessionID string, prompt string, scheduleContext schedule.ScheduleHandlerContext) (*schedule.RuntimeHandlerResult, error) {
		if appender == nil {
			return nil, fmt.Errorf("schedule session prompt appender is not configured")
		}
		snapshot := scheduleContext.PromptScope
		if snapshot == nil {
			return nil, fmt.Errorf("schedule session prompt requires frozen owner scope")
		}
		targetKind := strings.TrimSpace(snapshot.TargetKind)
		if targetKind == "" {
			targetKind = "session"
		}
		if targetKind == "session" && (snapshot.ConversationID == "" || snapshot.ConversationID != sessionID) {
			return nil, fmt.Errorf("schedule session prompt target %s does not match scope conversation %s", sessionID, snapshot.ConversationID)
		}
		if targetKind == "new_once" && (snapshot.ReservedConversationID == "" || snapshot.ReservedConversationID != sessionID) {
			return nil, fmt.Errorf("schedule new_once prompt target %s does not match reserved conversation %s", sessionID, snapshot.ReservedConversationID)
		}
		if targetKind != "session" && targetKind != "new_once" && targetKind != "new" {
			return nil, fmt.Errorf("schedule session prompt has unsupported target kind %s", targetKind)
		}
		if len(snapshot.OwnedExecutionScope) == 0 || strings.EqualFold(strings.TrimSpace(string(snapshot.OwnedExecutionScope)), "null") {
			return nil, fmt.Errorf("schedule session prompt has no owned execution scope")
		}
		var owned coordination.ExecutionScope
		if err := json.Unmarshal(snapshot.OwnedExecutionScope, &owned); err != nil {
			return nil, fmt.Errorf("decode schedule owned execution scope: %w", err)
		}
		if owned.SpaceID != snapshot.SpaceID || owned.RoleID != snapshot.CharacterID {
			return nil, fmt.Errorf("schedule owned execution scope does not match snapshot owner")
		}
		ctx = coordination.WithScope(ctx, owned)
		hostScope := HostAPIScopeContext{
			SpaceID:        snapshot.SpaceID,
			CharacterID:    snapshot.CharacterID,
			ConversationID: snapshot.ConversationID,
		}
		ctx = WithHostAPIScope(ctx, hostScope)
		requestID := strings.TrimSpace(scheduleContext.TriggerID)
		if requestID == "" {
			requestID = fmt.Sprintf("%s:%d", scheduleContext.ScheduleID, scheduleContext.Attempt)
		}
		createConversation := func(createRequestID string, conversationID string) (string, error) {
			if creator == nil {
				return "", fmt.Errorf("schedule new-session prompt creator is not configured")
			}
			raw, err := creator.CreateConversation(ctx, ConversationCreateRequest{
				SpaceID:        snapshot.SpaceID,
				CharacterID:    snapshot.CharacterID,
				ConversationID: conversationID,
				RequestID:      createRequestID,
				Title:          scheduleContext.Title,
				ProjectID:      snapshot.ProjectID,
				Channel:        "extension",
			})
			if err != nil {
				return "", fmt.Errorf("create scheduled session: %w", err)
			}
			var created struct {
				ConversationID string `json:"conversationId"`
				ID             string `json:"id"`
			}
			if err := json.Unmarshal(raw, &created); err != nil {
				return "", fmt.Errorf("decode created scheduled session: %w", err)
			}
			createdSessionID := strings.TrimSpace(created.ConversationID)
			if createdSessionID == "" {
				createdSessionID = strings.TrimSpace(created.ID)
			}
			if createdSessionID == "" {
				return "", fmt.Errorf("create scheduled session returned no conversation id: %s", string(raw))
			}
			if conversationID != "" && createdSessionID != conversationID {
				return "", fmt.Errorf("create scheduled session returned %s instead of reserved conversation %s: %s", createdSessionID, conversationID, string(raw))
			}
			return createdSessionID, nil
		}
		if targetKind == "new" {
			createdSessionID, err := createConversation(requestID, "")
			if err != nil {
				return nil, err
			}
			sessionID = createdSessionID
		}
		if targetKind == "new_once" {
			createdSessionID, err := createConversation(fmt.Sprintf("schedule-new-once:%s", scheduleContext.ScheduleID), snapshot.ReservedConversationID)
			if err != nil {
				return nil, err
			}
			sessionID = createdSessionID
		}
		hostScope.ConversationID = sessionID
		ctx = WithHostAPIScope(ctx, hostScope)
		result, err := appender.AppendConversationMessages(ctx, ConversationMessageAppendRequest{
			SpaceID:        snapshot.SpaceID,
			CharacterID:    snapshot.CharacterID,
			ConversationID: sessionID,
			Channel:        "extension",
			Role:           "user",
			Source:         "schedule:session_prompt",
			RequestID:      requestID,
			Parts: []ConversationMessagePart{
				{Type: "text", Content: prompt},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("append scheduled session prompt: %w", err)
		}
		raw, err := json.Marshal(map[string]any{
			"messageIds":      result.MessageIDs,
			"sequences":       result.Sequences,
			"deliveryGroupId": result.DeliveryGroupID,
			"lastSequence":    result.LastSequence,
			"prompt":          prompt,
			"targetSessionId": sessionID,
		})
		if err != nil {
			return nil, fmt.Errorf("encode scheduled session prompt result: %w", err)
		}
		return &schedule.RuntimeHandlerResult{ResultJSON: raw}, nil
	}
}
