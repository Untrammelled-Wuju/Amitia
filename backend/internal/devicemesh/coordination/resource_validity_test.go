package coordination

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/memorypolicy"
)

func TestResourceRecallableRestoresAutomaticDormancyOnly(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	document := map[string]json.RawMessage{
		"kind":    json.RawMessage(`"fact"`),
		"content": json.RawMessage(`{"value":"长期事实","memoryType":"fact","importance":1}`),
	}
	memorypolicy.WriteLifecycle(document, memorypolicy.NewLifecycleWithPolicy(memorypolicy.DecayPolicyDurable, 5, false, 1, now.Add(-365*24*time.Hour)))
	encoded, _ := json.Marshal(document)
	if ResourceUsable(encoded, now) {
		t.Fatal("dormant memory leaked into passive recall")
	}
	if !ResourceRecallable(encoded, now) {
		t.Fatal("dormant memory was not available to explicit retrieval")
	}
	document["archivedAt"] = json.RawMessage(`"2026-10-01T00:00:00Z"`)
	encoded, _ = json.Marshal(document)
	if ResourceRecallable(encoded, now) {
		t.Fatal("manually archived memory was exposed to explicit retrieval")
	}
}

func TestResourceRecallableRestoresAutomaticallyArchivedEpisode(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	document := map[string]json.RawMessage{
		"kind":    json.RawMessage(`"episodic"`),
		"content": json.RawMessage(`{"value":"旧事件","memoryType":"episodic","importance":1}`),
	}
	memorypolicy.WriteLifecycle(document, memorypolicy.NewLifecycleWithPolicy(memorypolicy.DecayPolicyEpisodic, 5, false, 1, now.Add(-365*24*time.Hour)))
	encoded, _ := json.Marshal(document)
	if ResourceUsable(encoded, now) {
		t.Fatal("archived episode leaked into passive recall")
	}
	if !ResourceRecallable(encoded, now) {
		t.Fatal("archived episode was not available to explicit retrieval")
	}
}

func TestResourceAvailabilityEnforcesTemporalValidity(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for _, testCase := range []struct {
		name string
		body string
		want bool
	}{
		{"currently-valid", `{"content":{"value":"当前事实","validFromUtc":"2026-10-01T00:00:00Z","validToUtc":"2026-11-01T00:00:00Z"}}`, true},
		{"not-yet-valid", `{"content":{"value":"未来事实","validFromUtc":"2026-11-01T00:00:00Z"}}`, false},
		{"expired", `{"content":{"value":"旧事实","validToUtc":"2026-10-12T00:00:00Z"}}`, false},
		{"date-only-end", `{"content":{"value":"当天有效","validTo":"2026-10-13"}}`, false},
	} {
		if got := ResourceRecallable(json.RawMessage(testCase.body), now); got != testCase.want {
			t.Fatalf("%s got=%v want=%v", testCase.name, got, testCase.want)
		}
	}
}
