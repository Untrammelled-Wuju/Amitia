package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
)

func TestEmitOwnedToolEventIncludesMeasuredDuration(t *testing.T) {
	var event business.Event
	inference := business.Inference{Emit: func(value business.Event) error {
		event = value
		return nil
	}}
	startedAt := time.Now().Add(-1500 * time.Millisecond)
	if err := emitOwnedToolEvent(inference, "tool-completed", "call-1", "workspace.read", nil, chat.ToolResult{Status: "SUCCESS"}, nil, startedAt); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(event.Progress, &payload); err != nil {
		t.Fatal(err)
	}
	duration, _ := payload["durationMs"].(float64)
	if duration < 1400 {
		t.Fatalf("durationMs = %v, want measured elapsed duration", duration)
	}
	if payload["startedAt"] == nil || payload["startedAt"] == "" {
		t.Fatalf("startedAt missing: %#v", payload)
	}
}
