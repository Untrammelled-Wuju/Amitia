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
	old := []json.RawMessage{json.RawMessage(`{"value":"obsolete-memory-data"}`)}
	inference := business.Inference{
		Scope:              coordination.ExecutionScope{RoleID: "role", RoleRevision: 1, ResourceOwnerID: "core", RoleOwnerID: "core"},
		Message:            "你好",
		Snapshot:           coordination.DataSnapshot{OwnerID: "core", Role: role, LegacyMemories: old, LegacyProfiles: old, LegacyEpisodes: old, LegacyGraphs: old},
		HistoricalSnapshot: &coordination.DataSnapshot{OwnerID: "source", LegacyMemories: old, LegacyProfiles: old, LegacyEpisodes: old, LegacyGraphs: old, Resources: []coordination.Resource{{Kind: "memory", Body: json.RawMessage(`{"value":"current-system-memory"}`)}}},
	}
	messages, err := ownedPrompt(inference)
	if err != nil {
		t.Fatal(err)
	}
	system := messages[0]["content"].(string)
	if strings.Contains(system, "obsolete-memory-data") || !strings.Contains(system, "current-system-memory") {
		t.Fatalf("memory prompt mixed versions: %s", system)
	}
	if len(inference.HistoricalSnapshot.LegacyMemories) != 1 {
		t.Fatal("prompt mutated snapshot")
	}
}
