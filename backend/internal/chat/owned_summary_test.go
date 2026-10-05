package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedMemorySummaryKeepsAuthorizedHistoryAndExcludesExpiredOrOtherOwnerData(t *testing.T) {
	inference := business.Inference{ConversationID: "conversation", Context: &business.ForwardedContext{Summary: "forwarded"}, HistoricalSnapshot: &coordination.DataSnapshot{OwnerID: "device-a", LegacySummary: json.RawMessage(`{"summary":"legacy"}`)}, Snapshot: coordination.DataSnapshot{OwnerID: "core-c", Resources: []coordination.Resource{
		{OwnerID: "core-c", Kind: "summary", Body: json.RawMessage(`{"conversationId":"conversation","content":{"summary":"current"}}`)},
		{OwnerID: "core-c", Kind: "summary", Body: json.RawMessage(`{"conversationId":"other-conversation","content":{"summary":"other conversation"}}`)},
		{OwnerID: "other-owner", Kind: "summary", Body: json.RawMessage(`{"conversationId":"conversation","content":{"summary":"other owner"}}`)},
		{OwnerID: "core-c", Kind: "summary", Deleted: true, Body: json.RawMessage(`{"conversationId":"conversation","content":{"summary":"deleted"}}`)},
		{OwnerID: "core-c", Kind: "summary", Body: json.RawMessage(`{"conversationId":"conversation","expiresAt":"2020-01-01T00:00:00Z","content":{"summary":"expired"}}`)},
	}}}
	encoded, err := json.Marshal(ownedSummaryContext(inference))
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, required := range []string{"forwarded", "legacy", "current"} {
		if !strings.Contains(text, required) {
			t.Fatalf("lost summary: %s", text)
		}
	}
	for _, forbidden := range []string{"other conversation", "other owner", "deleted", "expired"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("unauthorized summary used: %s", text)
		}
	}
}
