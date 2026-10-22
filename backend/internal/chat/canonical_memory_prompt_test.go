package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedPromptIgnoresLegacyMemoryCollections(t *testing.T) {
	role := coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{"characterId":"role","name":"角色"}`)}
	inference := business.Inference{
		Scope:              coordination.ExecutionScope{RoleID: "role", RoleRevision: 1, ResourceOwnerID: "core", RoleOwnerID: "core"},
		Message:            "你好",
		Snapshot:           coordination.DataSnapshot{OwnerID: "core", Role: role},
		HistoricalSnapshot: &coordination.DataSnapshot{OwnerID: "source", Resources: []coordination.Resource{{Kind: "memory", Body: json.RawMessage(`{"value":"current-system-memory"}`)}}},
	}
	messages, err := ownedPrompt(inference)
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, message := range messages {
		if content, ok := message["content"].(string); ok {
			text.WriteString(content)
		}
	}
	system := text.String()
	if strings.Contains(messages[0]["content"].(string), "current-system-memory") {
		t.Fatal("memory data was promoted into fixed system instructions")
	}
	if strings.Contains(system, "obsolete-memory-data") || !strings.Contains(system, "current-system-memory") {
		t.Fatalf("memory prompt mixed versions: %s", system)
	}
	if len(inference.HistoricalSnapshot.Resources) != 1 {
		t.Fatal("prompt mutated snapshot")
	}
}
