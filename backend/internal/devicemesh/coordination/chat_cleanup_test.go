package coordination_test

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestConversationCleanupKeepsMemoryOnlyWhenExplicitlyRequested(t *testing.T) {
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{false: "privacy-default", true: "keep-memory"}[keep], func(t *testing.T) {
			db, _ := setup(t)
			store := coordination.NewOwnershipStore(db, "a")
			commit := ownedCommit("create-chat")
			commit.Mutations = []coordination.Mutation{{Kind: "conversation", ID: "chat", RoleID: "role", Body: json.RawMessage(`{"title":"Chat"}`)}, {Kind: "message", ID: "message", RoleID: "role", Body: json.RawMessage(`{"conversationId":"chat","content":"hello"}`)}, {Kind: "memory", ID: "memory", RoleID: "role", Body: json.RawMessage(`{"conversationId":"chat","content":"remember"}`)}}
			if _, err := store.Apply(t.Context(), commit); err != nil {
				t.Fatal(err)
			}
			commit.Scope.RequestID = "delete-chat"
			commit.Mutations = []coordination.Mutation{{Kind: "conversation", ID: "chat", RoleID: "role", ExpectedRevision: 1, Deleted: true, KeepConversationMemories: keep, Body: json.RawMessage(`null`)}}
			if _, err := store.Apply(t.Context(), commit); err != nil {
				t.Fatal(err)
			}
			memory, err := store.Get(t.Context(), "memory", "memory")
			if err != nil || memory == nil || memory.Deleted == keep {
				t.Fatalf("memory=%+v err=%v keep=%v", memory, err, keep)
			}
			message, err := store.Get(t.Context(), "message", "message")
			if err != nil || message == nil || !message.Deleted {
				t.Fatalf("message=%+v err=%v", message, err)
			}
		})
	}
}
