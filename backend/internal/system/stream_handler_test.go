package system

import (
	"reflect"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/chat"
	"gorm.io/gorm"
)

func TestLoadMessagesAfterSequenceUsesPlanOrder(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&chat.Message{}); err != nil {
		t.Fatalf("migrate messages: %v", err)
	}

	messages := []chat.Message{
		{ID: "z-text", ConversationID: "conv-order", Sequence: 1, Role: "assistant", Content: "第一条", MsgType: "text"},
		{ID: "a-emote", ConversationID: "conv-order", Sequence: 2, Role: "assistant", Content: "表情", MsgType: "emote"},
		{ID: "m-text", ConversationID: "conv-order", Sequence: 3, Role: "assistant", Content: "第二条", MsgType: "text"},
		{ID: "other", ConversationID: "conv-other", Sequence: 4, Role: "assistant", Content: "其他会话", MsgType: "text"},
	}
	for i := range messages {
		if err := db.Create(&messages[i]).Error; err != nil {
			t.Fatalf("create message: %v", err)
		}
	}

	all, err := loadMessagesAfterSequence(db, "conv-order", 0)
	if err != nil {
		t.Fatalf("load all messages: %v", err)
	}
	if got := messageIDs(all); !reflect.DeepEqual(got, []string{"z-text", "a-emote", "m-text"}) {
		t.Fatalf("unexpected message order: %v", got)
	}

	afterFirst, err := loadMessagesAfterSequence(db, "conv-order", 1)
	if err != nil {
		t.Fatalf("load messages after cursor: %v", err)
	}
	if got := messageIDs(afterFirst); !reflect.DeepEqual(got, []string{"a-emote", "m-text"}) {
		t.Fatalf("unexpected messages after cursor: %v", got)
	}
}

func messageIDs(messages []map[string]interface{}) []string {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		id, _ := message["id"].(string)
		ids = append(ids, id)
	}
	return ids
}
