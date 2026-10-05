package chat

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedPromptUsesValidatedImageAndKeepsHistoricalBinaryOutOfSystemPrompt(t *testing.T) {
	encoded := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLttAAAAABJRU5ErkJggg=="
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	image := business.Attachment{Kind: "image", Name: "image.png", MIME: "image/png", Data: encoded, Hash: hex.EncodeToString(digest[:])}
	role := coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{"characterId":"role","name":"Core role"}`)}
	raw, _ := json.Marshal(map[string]any{"role": "user", "content": "previous image", "attachments": []business.Attachment{image}})
	inference := business.Inference{Scope: coordination.ExecutionScope{RoleID: "role", RoleRevision: 1, ResourceOwnerID: "core", RoleOwnerID: "core"}, Message: "current image", Attachments: []business.Attachment{image}, Snapshot: coordination.DataSnapshot{OwnerID: "core", Role: role}, HistoricalSnapshot: &coordination.DataSnapshot{OwnerID: "device", LegacyMessages: []json.RawMessage{raw}}}
	messages, err := ownedPrompt(inference)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(messages[0]["content"].(string), encoded) {
		t.Fatal("historical image copied into system prompt")
	}
	if len(messages) != 3 {
		t.Fatalf("messages=%d", len(messages))
	}
	for _, message := range messages[1:] {
		parts, ok := message["content"].([]map[string]any)
		if !ok || len(parts) != 2 || parts[1]["type"] != "image_url" || parts[1]["image_url"].(map[string]string)["url"] != "data:image/png;base64,"+encoded {
			t.Fatalf("invalid multimodal message: %+v", message)
		}
	}
	inference.Attachments[0].Hash = "tampered"
	if _, err := ownedPrompt(inference); err == nil {
		t.Fatal("tampered image passed model adapter")
	}
}
