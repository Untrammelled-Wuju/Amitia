package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedQuoteRealTLSUsesSourceHistoryAfterCoordinationWithoutMirroringIt(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "quote-a", schemas), newThreeCoreFixture(t, "quote-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	const endpoint = "/api/device-mesh/v1/business"
	view := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodGet, endpoint+"/memories?characterId=one", nil, 200))
	requestID := uuid.NewString()
	originalText := "保存于原设备的引用消息"
	original := decodeThreeCoreMemoryResponse[business.Response](t, b.request(t, a, http.MethodPost, endpoint+"/messages", map[string]any{"requestId": requestID, "characterId": "one", "message": originalText, "expectedExecutionScope": view.Scope}, 200))
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
	view = decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodGet, endpoint+"/memories?characterId=one", nil, 200))
	digest := sha256.Sum256([]byte(originalText))
	quote := business.QuoteReference{OwnerID: a.device.DeviceID.String(), RoleID: "one", ConversationID: original.ConversationID, MessageID: requestID + "/user", ExpectedRevision: original.UserRevision, ContentHash: hex.EncodeToString(digest[:]), ExpectedScope: &view.Scope}
	quotedRequest := uuid.NewString()
	payload := map[string]any{"requestId": quotedRequest, "characterId": "one", "historicalRoleId": "one", "conversationId": original.ConversationID, "conversationOrigin": business.ConversationOrigin{OwnerID: a.device.DeviceID.String(), ID: original.ConversationID}, "message": "解释原设备消息", "expectedExecutionScope": view.Scope, "quote": quote}
	response := decodeThreeCoreMemoryResponse[business.Response](t, b.request(t, a, http.MethodPost, endpoint+"/messages", payload, 200))
	if !response.Saved || response.Scope.ResourceOwnerID != b.core {
		t.Fatal("quoted reply did not use current Core owner")
	}
	core := coordination.NewOwnershipStore(b.services.KernelContainer.DeviceRegistry.Database(), b.core)
	row, err := core.Get(t.Context(), "message", quotedRequest+"/user")
	var saved struct {
		Quote *business.ReviewedQuote `json:"quote"`
	}
	if err != nil || row == nil || json.Unmarshal(row.Body, &saved) != nil || saved.Quote == nil || saved.Quote.Content != originalText || saved.Quote.OwnerID != a.device.DeviceID.String() || saved.Quote.ConversationID != original.ConversationID {
		t.Fatal("Core input lost verified original quote", err)
	}
	if mirrored, err := core.Get(t.Context(), "message", requestID+"/user"); err != nil || mirrored != nil {
		t.Fatal("old Source message migrated to Core", err)
	}
	quote.ExpectedRevision++
	payload["requestId"], payload["quote"] = uuid.NewString(), quote
	b.request(t, a, http.MethodPost, endpoint+"/messages", payload, 409)
}
