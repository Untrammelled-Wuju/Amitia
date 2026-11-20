package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedPromptProvidesAttachmentMetadataWithoutBinaryPayloads(t *testing.T) {
	image := business.Attachment{ArtifactID: "art-image", OwnerDeviceID: "core", StorageDeviceID: "core", Kind: "image", Name: "image.png", MIMEType: "image/png", SHA256: strings.Repeat("a", 64), SizeBytes: 2048}
	role := coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{"characterId":"role","name":"Core role"}`)}
	inference := business.Inference{
		Scope:              coordination.ExecutionScope{RoleID: "role", RoleRevision: 1, ResourceOwnerID: "core", RoleOwnerID: "core", RequestID: "request"},
		Message:            "current image",
		Attachments:        []business.Attachment{image},
		Snapshot:           coordination.DataSnapshot{OwnerID: "core", Role: role},
		AttachmentErrors:   []string{"语音转写失败：provider unavailable"},
		HistoricalSnapshot: &coordination.DataSnapshot{OwnerID: "device", Role: role, Resources: []coordination.Resource{ownedMessageResource("device", "role", "previous", map[string]any{"id": "previous", "role": "user", "content": "previous image", "attachments": []business.Attachment{image}})}},
	}
	messages, err := ownedPrompt(inference)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		encoded, err := json.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		text := string(encoded)
		if strings.Contains(text, "base64") || strings.Contains(text, `"data"`) || strings.Contains(text, "image_url") {
			t.Fatalf("attachment binary leaked into model prompt: %s", text)
		}
	}
	all := ""
	for _, message := range messages {
		all += strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(strings.Join([]string{stringFromMessage(message)}, " ")), "\n", " "), "\t", " ")) + "\n"
	}
	for _, value := range []string{"art-image", "ownerDeviceId=core", "storageDeviceId=core", strings.Repeat("a", 64), "chat_attachment_extract", "provider unavailable"} {
		if !strings.Contains(all, value) {
			t.Fatalf("prompt missing %q: %s", value, all)
		}
	}
	for _, item := range inference.Attachments {
		if _, ok := any(item).(map[string]any); ok {
			t.Fatal("attachment was converted into an unbounded map")
		}
	}
}

func stringFromMessage(message map[string]interface{}) string {
	switch value := message["content"].(type) {
	case string:
		return value
	default:
		encoded, _ := json.Marshal(value)
		return string(encoded)
	}
}
