package chat

import (
	"context"
	"testing"
)

type fixedMessageOutputPlanner struct {
	ModelToolRuntime
	outputs []MessageOutput
	err     error
}

func (p fixedMessageOutputPlanner) PlanMessageOutputs(context.Context, SkillScope, *MessageOutputPlanningEvent) ([]MessageOutput, error) {
	return p.outputs, p.err
}

func TestCommitInteractionAppliesGenericMessageOutputs(t *testing.T) {
	db, svc, convID := setupCommitCoordinatorTest(t, false)
	svc.toolRuntime = fixedMessageOutputPlanner{outputs: []MessageOutput{
		{
			ExtensionID: "com.example/media",
			InsertAfter: 1,
			SendMode:    "between_text_messages",
			Part: MessagePart{
				Type:          "image",
				ExtensionType: "media-card",
				URL:           "/api/extension/resources/example",
				FallbackURL:   "/api/extension/resources/fallback",
				AltText:       "示例图片",
				MIMEType:      "image/png",
				Width:         320,
				Height:        240,
			},
		},
	}}
	result, err := svc.commitInteraction(t.Context(), messageCommitPlan{
		Request:       &ProcessMessageRequest{CharacterID: "char-commit", ConversationID: convID, Channel: "web", Source: "manual", RequestID: "req-output"},
		Conversation:  convID,
		Character:     "char-commit",
		UserMessageID: "user-commit",
		Reply:         "第一句\n第二句",
		Lines:         []string{"第一句", "第二句"},
		Source:        "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.MessagePlan == nil || len(result.MessagePlan.Items) != 3 {
		t.Fatalf("expected three planned messages, got %#v", result.MessagePlan)
	}
	types := []string{
		result.MessagePlan.Items[0].Type,
		result.MessagePlan.Items[1].Type,
		result.MessagePlan.Items[2].Type,
	}
	if types[0] != "text" || types[1] != "image" || types[2] != "text" {
		t.Fatalf("unexpected planned output order: %#v", types)
	}
	var image Message
	if err := db.Where("id = ?", result.MessagePlan.Items[1].MessageID).Take(&image).Error; err != nil {
		t.Fatal(err)
	}
	if image.MsgType != "image" || image.ExtensionType != "media-card" || image.ImageUrl == "" || image.OriginalAsset == "" || image.MediaWidth != 320 {
		t.Fatalf("unexpected image message: %#v", image)
	}
}

func TestAppendConversationMessagesPersistsGenericParts(t *testing.T) {
	db, svc, convID := setupCommitCoordinatorTest(t, false)
	if err := db.Model(&Conversation{}).Where("id = ?", convID).Update("user_id", "user:web").Error; err != nil {
		t.Fatal(err)
	}
	result, err := svc.AppendConversationMessages(context.Background(), &AppendConversationMessagesRequest{
		UserID:         "user:web",
		CharacterID:    "char-commit",
		ConversationID: convID,
		Channel:        "web",
		Role:           "user",
		Source:         "extension:com.example/media",
		RequestID:      "append-req",
		Parts: []MessagePart{
			{Type: "text", Content: "你好"},
			{Type: "image", ExtensionType: "media-card", URL: "/api/extension/resources/example", AltText: "图片", MIMEType: "image/png"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.MessageIDs) != 2 || len(result.Sequences) != 2 || result.LastSequence <= 0 {
		t.Fatalf("unexpected append result: %#v", result)
	}
	var messages []Message
	if err := db.Where("conversation_id = ? AND request_id = ?", convID, "append-req").Order("delivery_sequence ASC").Find(&messages).Error; err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected two appended messages, got %d", len(messages))
	}
	if messages[1].MsgType != "image" || messages[1].ExtensionType != "media-card" || messages[1].ImageUrl != "/api/extension/resources/example" {
		t.Fatalf("unexpected appended image message: %#v", messages[1])
	}
}
