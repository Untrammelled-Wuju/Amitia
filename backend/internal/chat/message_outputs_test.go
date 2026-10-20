package chat

import (
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"testing"
)

func TestOwnedAppendPersistsGenericPartsAndDeduplicates(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	f.model.llmWithTools = ownedFixedReply("seed")
	if _, err := f.engine.Run(t.Context(), f.request("seed")); err != nil {
		t.Fatal(err)
	}
	request := f.request("append-req")
	input := business.AppendOwnedMessagesRequest{ConversationID: "conversation", Role: "user", Source: "extension:media", Channel: "web", Parts: []business.AppendedMessagePart{{Type: "text", Content: "你好"}, {Type: "image", ExtensionType: "media-card", URL: "/api/extension/resources/example", AltText: "图片", MIMEType: "image/png"}}}
	var first business.AppendOwnedMessagesResult
	for i := 0; i < 2; i++ {
		result, err := f.engine.AppendOwnedMessages(t.Context(), request, input)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.MessageIDs) != 2 || result.LastSequence != 4 || result.Sequences[0] != 3 || result.Channel != "web" || result.Role != "user" || result.Status != "sent" {
			t.Fatalf("bad sequence: %+v", result)
		}
		if i == 0 {
			first = result
		} else if result.MessageIDs[0] != first.MessageIDs[0] {
			t.Fatal("duplicate append")
		}
	}
	if len(f.messages(t)) != 4 {
		t.Fatal("append replay wrote extra messages")
	}
	row, err := coordination.NewOwnershipStore(f.db, "device").Get(t.Context(), "message", first.MessageIDs[1])
	if err != nil {
		t.Fatal(err)
	}
	var message map[string]any
	if err = json.Unmarshal(row.Body, &message); err != nil {
		t.Fatal(err)
	}
	if row.RoleID != "role" || message["imageUrl"] != input.Parts[1].URL || message["extensionType"] != "media-card" {
		t.Fatalf("media lost: %s", row.Body)
	}
	input.Parts[0].Content = "changed"
	if _, err = f.engine.AppendOwnedMessages(t.Context(), request, input); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("conflicting replay accepted: %v", err)
	}
}
func TestOwnedAppendRejectsSplitAssistantAndWrongQuote(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	f.model.llmWithTools = ownedFixedReply("seed")
	if _, err := f.engine.Run(t.Context(), f.request("seed")); err != nil {
		t.Fatal(err)
	}
	input := business.AppendOwnedMessagesRequest{ConversationID: "conversation", Role: "assistant", Parts: []business.AppendedMessagePart{{Type: "text", Content: "第一段"}, {Type: "text", Content: "第二段"}}}
	if _, err := f.engine.AppendOwnedMessages(t.Context(), f.request("split"), input); err == nil {
		t.Fatal("split assistant accepted")
	}
	input.Parts = input.Parts[:1]
	quote := "missing"
	input.ReplyToMessageID = &quote
	if _, err := f.engine.AppendOwnedMessages(t.Context(), f.request("quote"), input); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("unowned quote accepted: %v", err)
	}
	if len(f.messages(t)) != 2 {
		t.Fatal("rejected append persisted")
	}
}
