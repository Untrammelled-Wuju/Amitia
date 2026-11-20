package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedPromptIdentifiesScheduledAssistantMessages(t *testing.T) {
	role := coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{"characterId":"role","name":"角色"}`)}
	for _, source := range []string{"proactive", "reminder"} {
		t.Run(source, func(t *testing.T) {
			inference := business.Inference{Scope: coordination.ExecutionScope{RoleID: "role", RoleRevision: 1, ResourceOwnerID: "core", RoleOwnerID: "core"}, Message: "我看到了", Snapshot: coordination.DataSnapshot{OwnerID: "core", Role: role, Resources: []coordination.Resource{ownedMessageResource("core", "role", "scheduled", map[string]any{"role": "assistant", "source": source, "characterId": "role", "content": "记得休息"})}}}
			messages, err := ownedPrompt(inference)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for i, message := range messages {
				if message["content"] != "记得休息" {
					continue
				}
				if message["role"] != "assistant" || i == 0 || messages[i-1]["role"] != "system" || !strings.Contains(messages[i-1]["content"].(string), "发送者是你，不是用户") {
					t.Fatalf("scheduled message has incorrect provenance: %#v", messages)
				}
				found = true
			}
			if !found || messages[len(messages)-1]["role"] != "user" || messages[len(messages)-1]["content"] != "我看到了" {
				t.Fatalf("conversation messages lost: %#v", messages)
			}
		})
	}
}
