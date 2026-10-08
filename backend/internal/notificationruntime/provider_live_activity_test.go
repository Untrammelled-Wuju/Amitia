package notificationruntime

import (
	"testing"
	"time"
)

func TestLiveActivityAPNSPayloadCarriesPresentationState(t *testing.T) {
	now := time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)
	updated := now.Add(-45 * time.Second)
	started := now.Add(-5 * time.Minute)
	payload := liveActivityAPNSPayload(
		DeviceEndpoint{Locale: "zh-TW", Appearance: "dark"},
		ExecutionState{
			RunID:          "run-a",
			ConversationID: "conversation-a",
			CharacterID:    "character-a",
			AgentID:        "codex",
			Title:          "任务",
			Summary:        "正在处理",
			Phase:          "running",
			CurrentStep:    2,
			TotalSteps:     5,
			Progress:       0.4,
			TotalTokens:    1234,
			Revision:       7,
			StartedAt:      started,
			UpdatedAt:      updated,
		},
		"start",
		now,
	)
	aps := payload["aps"].(map[string]any)
	if got := aps["stale-date"]; got != now.Add(5*time.Minute).Unix() {
		t.Fatalf("stale-date = %#v", got)
	}
	if got := aps["relevance-score"]; got != float64(updated.Unix()) {
		t.Fatalf("relevance-score must use business update time, got %#v", got)
	}
	content := aps["content-state"].(map[string]any)
	if content["locale"] != "zh-TW" || content["appearance"] != "dark" || content["agentName"] != "codex" {
		t.Fatalf("presentation state mismatch: %#v", content)
	}
	if content["totalTokens"] != 1234 {
		t.Fatalf("token usage missing: %#v", content)
	}
	attributes := aps["attributes"].(map[string]any)
	if attributes["runId"] != "run-a" || attributes["conversationId"] != "conversation-a" {
		t.Fatalf("activity identity mismatch: %#v", attributes)
	}
}

func TestLiveActivityAPNSHeartbeatDoesNotStealPriority(t *testing.T) {
	now := time.Date(2026, 10, 8, 4, 0, 0, 0, time.UTC)
	businessAt := now.Add(-20 * time.Minute)
	payload := liveActivityAPNSPayload(
		DeviceEndpoint{},
		ExecutionState{RunID: "run-a", Phase: "running", UpdatedAt: businessAt},
		"update",
		now,
	)
	aps := payload["aps"].(map[string]any)
	if aps["relevance-score"] != float64(businessAt.Unix()) {
		t.Fatalf("heartbeat changed business priority: %#v", aps["relevance-score"])
	}
}

func TestLiveActivityAPNSPayloadFallsBackFromZeroTimes(t *testing.T) {
	now := time.Date(2026, 10, 8, 4, 30, 0, 0, time.UTC)
	payload := liveActivityAPNSPayload(
		DeviceEndpoint{},
		ExecutionState{RunID: "run-zero", Phase: "running"},
		"start",
		now,
	)
	aps := payload["aps"].(map[string]any)
	content := aps["content-state"].(map[string]any)
	if content["updatedAt"] != now.Unix() {
		t.Fatalf("zero UpdatedAt was not repaired: %#v", content["updatedAt"])
	}
	attributes := aps["attributes"].(map[string]any)
	if attributes["startedAt"] != now.Unix() {
		t.Fatalf("zero StartedAt was not repaired: %#v", attributes["startedAt"])
	}
}

func TestLiveActivityAPNSTerminalDismissal(t *testing.T) {
	now := time.Date(2026, 10, 8, 5, 0, 0, 0, time.UTC)
	completed := liveActivityAPNSPayload(
		DeviceEndpoint{},
		ExecutionState{RunID: "run-a", Phase: "completed", UpdatedAt: now},
		"end",
		now,
	)["aps"].(map[string]any)
	if completed["relevance-score"] != 0 {
		t.Fatalf("terminal relevance must be zero: %#v", completed)
	}
	if completed["dismissal-date"] != now.Add(60*time.Second).Unix() {
		t.Fatalf("completed dismissal mismatch: %#v", completed["dismissal-date"])
	}
	if _, ok := completed["stale-date"]; ok {
		t.Fatal("terminal payload must not carry stale-date")
	}

	cancelled := liveActivityAPNSPayload(
		DeviceEndpoint{},
		ExecutionState{RunID: "run-b", Phase: "cancelled", UpdatedAt: now},
		"end",
		now,
	)["aps"].(map[string]any)
	if cancelled["dismissal-date"] != now.Unix() {
		t.Fatalf("cancelled activity must dismiss immediately: %#v", cancelled["dismissal-date"])
	}
}
