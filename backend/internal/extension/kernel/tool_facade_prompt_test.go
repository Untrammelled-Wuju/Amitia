package kernel

import (
	"encoding/json"
	"testing"
)

func TestBuildBeforePromptPayloadIncludesMessageAndSource(t *testing.T) {
	facade := &ToolFacade{}
	raw := facade.buildBeforePromptPayload(LegacyScope{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Channel:        "web",
		SessionID:      "session-1",
		Message:        "主动发送提示",
		Source:         "proactive",
		IsInternal:     true,
	})
	var payload struct {
		Context map[string]any `json:"context"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Context["message"] != "主动发送提示" {
		t.Fatalf("expected message in hook payload, got %#v", payload.Context["message"])
	}
	if payload.Context["source"] != "proactive" {
		t.Fatalf("expected proactive source, got %#v", payload.Context["source"])
	}
	if payload.Context["internal"] != true {
		t.Fatalf("expected internal flag, got %#v", payload.Context["internal"])
	}
}
