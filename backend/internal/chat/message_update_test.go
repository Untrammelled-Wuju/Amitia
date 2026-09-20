package chat

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/pipelinecheckpoint"
)

func TestUpdateMessageForSpace(t *testing.T) {
	svc := setupScopedChatService(t)
	if err := svc.db.AutoMigrate(&pipelinecheckpoint.Record{}); err != nil {
		t.Fatal(err)
	}
	conversation := &Conversation{
		ID:       "conv-update",
		SpaceID:  "space-1",
		Title:    "修改测试",
		Channel:  "web",
		Source:   "manual",
		Revision: 1,
	}
	if err := svc.db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	userMessage := &Message{
		ID:             "msg-user",
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "旧内容",
		Sequence:       1,
		MsgType:        "text",
		Source:         "web",
		Revision:       1,
	}
	assistantMessage := &Message{
		ID:             "msg-assistant",
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "AI 内容",
		Sequence:       2,
		MsgType:        "text",
		Source:         "web",
		Revision:       1,
	}
	if err := svc.db.Create(userMessage).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Create(assistantMessage).Error; err != nil {
		t.Fatal(err)
	}

	updated, err := svc.UpdateMessageForSpace("msg-user", "space-1", "  新内容  ")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Content != "新内容" {
		t.Fatalf("expected trimmed content, got %q", updated.Content)
	}
	if updated.Revision != 2 {
		t.Fatalf("expected revision 2, got %d", updated.Revision)
	}
	if updated.UpdatedAt == "" {
		t.Fatal("expected updated_at to be populated")
	}

	if _, err := svc.UpdateMessageForSpace("msg-assistant", "space-1", "不能修改"); err == nil || !strings.Contains(err.Error(), "只能修改用户消息") {
		t.Fatalf("expected assistant message rejection, got %v", err)
	}
	if _, err := svc.UpdateMessageForSpace("msg-user", "space-1", "   "); err == nil || !strings.Contains(err.Error(), "消息内容不能为空") {
		t.Fatalf("expected empty content rejection, got %v", err)
	}
	if _, err := svc.UpdateMessageForSpace("msg-user", "space-2", "越权修改"); err == nil || !strings.Contains(err.Error(), "消息不存在") {
		t.Fatalf("expected space ownership rejection, got %v", err)
	}
}
