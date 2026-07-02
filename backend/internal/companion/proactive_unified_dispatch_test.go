package companion

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

type fakeProactiveUnifiedEntry struct {
	requests []*interaction.UnifiedEntryRequest
	err      error
}

func (f *fakeProactiveUnifiedEntry) Handle(ctx context.Context, req *interaction.UnifiedEntryRequest) (*interaction.OrchestrationResult, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	return &interaction.OrchestrationResult{
		Outcome: interaction.OutcomeCompleted,
		Response: &interaction.ProcessResponse{
			Reply:          "统一入口回复",
			ConversationID: req.ConversationID,
			CharacterID:    req.CharacterID,
			RequestID:      req.RequestID,
			Events: []interaction.OutboxRecord{
				{EventType: "interaction.runtime_assembled"},
				{EventType: "interaction.completed"},
			},
		},
	}, nil
}

func TestProcessDueActiveMessageTasksUsesUnifiedEntryWithoutDirectMessageWrite(t *testing.T) {
	svc := setupCompanionScopeService(t)
	fake := &fakeProactiveUnifiedEntry{}
	svc.unifiedEntry = fake
	now := time.Now().Add(-time.Minute).Format("2006-01-02 15:04:05")
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, peer_id, updated_at) VALUES ('conv-target', 'char-1', 'wechat', 'peer-1', ?)", now).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO active_message_settings (character_id, channel) VALUES ('char-1', 'wechat')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO active_message_task (id, character_id, task_type, due_time, prompt, status) VALUES (2, 'char-1', 'morning_share', ?, '早安 prompt', 'PENDING')", now).Error; err != nil {
		t.Fatal(err)
	}

	result := svc.ProcessDueActiveMessageTasks("char-1")
	if result["sent"] != 1 {
		t.Fatalf("expected one sent task, got %#v", result)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("expected one unified entry request, got %d", len(fake.requests))
	}
	req := fake.requests[0]
	if req.ConversationID != "conv-target" || req.CharacterID != "char-1" || req.Channel != "wechat" || req.PeerID != "peer-1" {
		t.Fatalf("unexpected unified request scope: %#v", req)
	}
	if req.Message != "早安 prompt" {
		t.Fatalf("expected prompt to be submitted to unified entry, got %q", req.Message)
	}

	var messageCount int64
	if err := svc.db.Table("messages").Where("conversation_id = ?", "conv-target").Count(&messageCount).Error; err != nil {
		t.Fatal(err)
	}
	if messageCount != 0 {
		t.Fatalf("expected companion not to write messages directly, got %d", messageCount)
	}
	var proactiveStatus string
	if err := svc.db.Table("proactive_messages").Select("status").Where("conversation_id = ?", "conv-target").Limit(1).Row().Scan(&proactiveStatus); err != nil {
		t.Fatal(err)
	}
	if proactiveStatus != "queued" {
		t.Fatalf("expected queued proactive audit status, got %q", proactiveStatus)
	}
}
