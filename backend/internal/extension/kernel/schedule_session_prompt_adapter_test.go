package kernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

type schedulePromptAppender struct {
	request ConversationMessageAppendRequest
	scope   HostAPIScopeContext
	result  ConversationMessageAppendResult
	err     error
}

type schedulePromptCreator struct {
	request ConversationCreateRequest
	output  json.RawMessage
	err     error
}

func (c *schedulePromptCreator) CreateConversation(_ context.Context, request ConversationCreateRequest) (json.RawMessage, error) {
	c.request = request
	return c.output, c.err
}

func (a *schedulePromptAppender) AppendConversationMessages(ctx context.Context, request ConversationMessageAppendRequest) (ConversationMessageAppendResult, error) {
	a.request = request
	a.scope = GetHostAPIScope(ctx)
	return a.result, a.err
}

func TestScheduleSessionPromptInvokeUsesOwnedScopeAndAppender(t *testing.T) {
	owned := coordination.ExecutionScope{
		SpaceID:           "core",
		InitiatorDeviceID: "device-1",
		TargetDeviceID:    "device-1",
		CoreID:            "core",
		RoleID:            "role-1",
		RoleOwnerID:       "role-owner",
		ResourceOwnerID:   "resource-owner",
	}
	raw, err := json.Marshal(owned)
	if err != nil {
		t.Fatal(err)
	}
	appender := &schedulePromptAppender{result: ConversationMessageAppendResult{
		MessageIDs:   []string{"message-1"},
		Sequences:    []int64{3},
		LastSequence: 3,
	}}
	invoke := NewScheduleSessionPromptInvokeFunc(appender, nil)
	result, err := invoke(context.Background(), "conversation-1", "run scheduled prompt", schedule.ScheduleHandlerContext{
		ScheduleID:      "schedule-1",
		TriggerID:       "trigger-1",
		Attempt:         2,
		ScopeSnapshotID: "scope-1",
		PromptScope: &schedule.SessionPromptScope{
			SpaceID:             "core",
			CharacterID:         "role-1",
			ConversationID:      "conversation-1",
			TargetKind:          "session",
			OwnedExecutionScope: raw,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || len(result.ResultJSON) == 0 {
		t.Fatal("expected schedule result")
	}
	if appender.request.SpaceID != "core" || appender.request.CharacterID != "role-1" || appender.request.ConversationID != "conversation-1" {
		t.Fatalf("unexpected owner request: %+v", appender.request)
	}
	if appender.request.Role != "user" || appender.request.Source != "schedule:session_prompt" || appender.request.RequestID != "trigger-1" {
		t.Fatalf("unexpected delivery metadata: %+v", appender.request)
	}
	if len(appender.request.Parts) != 1 || appender.request.Parts[0].Content != "run scheduled prompt" {
		t.Fatalf("unexpected prompt parts: %+v", appender.request.Parts)
	}
	if !strings.Contains(string(result.ResultJSON), `"prompt":"run scheduled prompt"`) {
		t.Fatalf("expected immutable delivered prompt in result: %s", string(result.ResultJSON))
	}
}

func TestScheduleSessionPromptRejectsCrossSessionTarget(t *testing.T) {
	raw, _ := json.Marshal(coordination.ExecutionScope{SpaceID: "core", RoleID: "role-1"})
	invoke := NewScheduleSessionPromptInvokeFunc(&schedulePromptAppender{}, nil)
	_, err := invoke(context.Background(), "conversation-2", "prompt", schedule.ScheduleHandlerContext{
		ScheduleID:      "schedule-1",
		TriggerID:       "trigger-1",
		ScopeSnapshotID: "scope-1",
		ScheduledAt:     time.Now(),
		PromptScope: &schedule.SessionPromptScope{
			SpaceID:             "core",
			CharacterID:         "role-1",
			ConversationID:      "conversation-1",
			TargetKind:          "session",
			OwnedExecutionScope: raw,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match scope conversation") {
		t.Fatalf("expected cross-session rejection, got %v", err)
	}
}

func TestScheduleSessionPromptNewTargetCreatesConversation(t *testing.T) {
	owned := coordination.ExecutionScope{SpaceID: "core", RoleID: "role-1"}
	raw, _ := json.Marshal(owned)
	appender := &schedulePromptAppender{result: ConversationMessageAppendResult{MessageIDs: []string{"message-new"}}}
	creator := &schedulePromptCreator{output: []byte(`{"conversationId":"conversation-new"}`)}
	invoke := NewScheduleSessionPromptInvokeFunc(appender, creator)
	result, err := invoke(context.Background(), "new", "prompt", schedule.ScheduleHandlerContext{
		ScheduleID:      "schedule-new",
		TriggerID:       "trigger-new",
		ScopeSnapshotID: "scope-new",
		Title:           "Scheduled conversation",
		PromptScope: &schedule.SessionPromptScope{
			SpaceID:             "core",
			CharacterID:         "role-1",
			ProjectID:           "project-1",
			TargetKind:          "new",
			OwnedExecutionScope: raw,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if creator.request.RequestID != "trigger-new" || creator.request.Title != "Scheduled conversation" || creator.request.ProjectID != "project-1" {
		t.Fatalf("unexpected create request: %+v", creator.request)
	}
	if appender.request.ConversationID != "conversation-new" || appender.scope.ConversationID != "conversation-new" {
		t.Fatalf("expected new conversation in request and scope, got %+v and %+v", appender.request, appender.scope)
	}
	if !strings.Contains(string(result.ResultJSON), `"targetSessionId":"conversation-new"`) {
		t.Fatalf("expected target session in result: %s", string(result.ResultJSON))
	}
}

func TestScheduleSessionPromptNewOnceCreatesReservedConversationIdempotently(t *testing.T) {
	owned := coordination.ExecutionScope{SpaceID: "core", RoleID: "role-1"}
	raw, _ := json.Marshal(owned)
	appender := &schedulePromptAppender{result: ConversationMessageAppendResult{MessageIDs: []string{"message-new-once"}}}
	creator := &schedulePromptCreator{output: []byte(`{"conversationId":"dsh-session-reserved"}`)}
	invoke := NewScheduleSessionPromptInvokeFunc(appender, creator)
	result, err := invoke(context.Background(), "dsh-session-reserved", "prompt", schedule.ScheduleHandlerContext{
		ScheduleID:      "schedule-new-once",
		TriggerID:       "trigger-new-once",
		ScopeSnapshotID: "scope-new-once",
		Title:           "Reusable scheduled conversation",
		PromptScope: &schedule.SessionPromptScope{
			SpaceID:                "core",
			CharacterID:            "role-1",
			ReservedConversationID: "dsh-session-reserved",
			TargetKind:             "new_once",
			OwnedExecutionScope:    raw,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if creator.request.ConversationID != "dsh-session-reserved" {
		t.Fatalf("expected reserved conversation id, got %+v", creator.request)
	}
	if creator.request.RequestID != "schedule-new-once:schedule-new-once" {
		t.Fatalf("expected stable creation request id, got %+v", creator.request)
	}
	if appender.request.ConversationID != "dsh-session-reserved" || appender.request.RequestID != "trigger-new-once" {
		t.Fatalf("unexpected new_once delivery: %+v", appender.request)
	}
	if !strings.Contains(string(result.ResultJSON), `"targetSessionId":"dsh-session-reserved"`) {
		t.Fatalf("expected reserved target session in result: %s", string(result.ResultJSON))
	}
}

func TestScheduleSessionPromptNewOnceRejectsUnreservedConversation(t *testing.T) {
	raw, _ := json.Marshal(coordination.ExecutionScope{SpaceID: "core", RoleID: "role-1"})
	invoke := NewScheduleSessionPromptInvokeFunc(&schedulePromptAppender{}, &schedulePromptCreator{})
	_, err := invoke(context.Background(), "conversation-other", "prompt", schedule.ScheduleHandlerContext{
		ScheduleID:      "schedule-new-once",
		TriggerID:       "trigger-new-once",
		ScopeSnapshotID: "scope-new-once",
		PromptScope: &schedule.SessionPromptScope{
			SpaceID:                "core",
			CharacterID:            "role-1",
			ReservedConversationID: "dsh-session-reserved",
			TargetKind:             "new_once",
			OwnedExecutionScope:    raw,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match reserved conversation") {
		t.Fatalf("expected reserved conversation rejection, got %v", err)
	}
}
