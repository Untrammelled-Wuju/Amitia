package business

import (
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedImportCopiesAllMessagesAndReplaysWithoutDuplicates(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	scope := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", TargetDeviceID: "a", ResourceOwnerID: "a", RoleOwnerID: "a", RoleID: "role", RoleRevision: 1, RequestID: "owned-import"}
	request := coordination.ChatManagementRequest{ConversationOwnerID: "a", ConversationID: "legacy-import", RequestID: "import-request"}
	messages := []map[string]any{}
	for i := 0; i < 301; i++ {
		messages = append(messages, map[string]any{"id": fmt.Sprintf("legacy-%d", i), "conversationId": "legacy-import", "characterId": "old-role", "role": "user", "content": fmt.Sprint(i)})
	}
	archive := map[string]any{"sourceOwnerId": "a", "sourceRoleId": "old-role", "conversation": map[string]any{"id": "legacy-import", "source": "import", "title": "Imported"}, "messages": messages}
	first, err := engine.commitImportedChat(t.Context(), scope, request, archive)
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.commitImportedChat(t.Context(), scope, request, archive)
	if err != nil || first["conversationId"] != second["conversationId"] {
		t.Fatalf("replay=%v err=%v", second, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM kernel_device_owned_resources WHERE owner_id='a' AND kind='message' AND role_id='role' AND deleted=0`).Scan(&count); err != nil || count != 301 {
		t.Fatalf("messages=%d err=%v", count, err)
	}
	archive["sourceOwnerId"] = "b"
	if _, err := engine.commitImportedChat(t.Context(), scope, request, archive); err == nil {
		t.Fatal("foreign source accepted")
	}
}
