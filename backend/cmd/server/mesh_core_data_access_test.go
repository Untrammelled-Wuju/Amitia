package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestActualTLSCoreDataIsReadOnlyWithoutAdministrator(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a := newThreeCoreFixture(t, "core-data-access-a", schemas)
	b := newThreeCoreFixture(t, "core-data-access-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, b, a, policy.ModeRevision)
	readState := func() struct {
		CoreID         string              `json:"coreId"`
		CanAdminister  bool                `json:"canAdminister"`
		Policy         coordination.Policy `json:"policy"`
		CoreDataAccess struct {
			Read   bool   `json:"read"`
			Write  bool   `json:"write"`
			Reason string `json:"reason"`
		} `json:"coreDataAccess"`
	} {
		var state struct {
			CoreID         string              `json:"coreId"`
			CanAdminister  bool                `json:"canAdminister"`
			Policy         coordination.Policy `json:"policy"`
			CoreDataAccess struct {
				Read   bool   `json:"read"`
				Write  bool   `json:"write"`
				Reason string `json:"reason"`
			} `json:"coreDataAccess"`
		}
		if err := json.Unmarshal(b.request(t, a, http.MethodGet, "/api/device-mesh/v1/coordination/me", nil, http.StatusOK), &state); err != nil {
			t.Fatal(err)
		}
		return state
	}
	state := readState()
	if state.CoreID != b.core || !state.Policy.Coordinated || state.Policy.Administrator || state.CanAdminister || !state.CoreDataAccess.Read || state.CoreDataAccess.Write || state.CoreDataAccess.Reason == "" {
		t.Fatalf("unexpected non-administrator access: %+v", state)
	}
	const endpoint = "/api/device-mesh/v1/business"
	view := decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/data?kind=worldbook&characterId=one", nil, http.StatusOK))
	if view.Scope.ResourceOwnerID != b.core {
		t.Fatal("center worldbook was not readable")
	}
	b.request(t, a, http.MethodPost, endpoint+"/worldbook/manage", map[string]any{
		"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": view.Scope,
		"action": "create", "rule": map[string]any{"scope": "role", "matchType": "keyword", "matchPattern": "只读拒绝", "matchScope": "full_context", "injectContent": "不得保存", "priority": 1, "enabled": true},
	}, http.StatusForbidden)
	policy, err = b.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), b.core, a.device.DeviceID.String(), state.Policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	state = readState()
	if !state.CanAdminister || !state.Policy.Administrator || !state.CoreDataAccess.Write || state.CoreDataAccess.Reason != "" {
		t.Fatalf("administrator did not receive write access: %+v", state)
	}
	view = decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/data?kind=worldbook&characterId=one", nil, http.StatusOK))
	created := decodeThreeCoreMemoryResponse[business.WorldbookMutationResponse](t, b.request(t, a, http.MethodPost, endpoint+"/worldbook/manage", map[string]any{
		"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": view.Scope,
		"action": "create", "rule": map[string]any{"scope": "role", "matchType": "keyword", "matchPattern": "管理员写入", "matchScope": "full_context", "injectContent": "允许保存", "priority": 1, "enabled": true},
	}, http.StatusOK))
	if !created.Saved || created.Rule == nil || created.Acknowledgement.OwnerID != b.core {
		t.Fatal("administrator write was not committed by center")
	}
	policy, err = b.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), b.core, a.device.DeviceID.String(), policy.PermissionRevision, false)
	if err != nil {
		t.Fatal(err)
	}
	state = readState()
	if state.CanAdminister || state.Policy.Administrator || state.CoreDataAccess.Write || state.CoreDataAccess.Reason == "" {
		t.Fatalf("revocation did not restore read-only access: %+v", state)
	}
	b.request(t, a, http.MethodPost, endpoint+"/worldbook/manage", map[string]any{
		"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": view.Scope,
		"action": "delete", "expectedRevision": created.Rule.Revision, "rule": map[string]any{"id": created.Rule.ID, "scope": "role"},
	}, http.StatusForbidden)
	listed := decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/data?kind=worldbook&characterId=one", nil, http.StatusOK))
	if len(listed.Snapshot.Resources) != 1 {
		t.Fatalf("read-only revocation changed center data: %d", len(listed.Snapshot.Resources))
	}
}

func TestActualTLSCoordinatedDeviceKeepsConversationDataLocal(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a := newThreeCoreFixture(t, "local-conversation-a", schemas)
	b := newThreeCoreFixture(t, "local-conversation-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, b, a, policy.ModeRevision)
	policy, err = b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), b.core, a.device.DeviceID.String(), policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	const endpoint = "/api/device-mesh/v1/business"
	reply := decodeThreeCoreMemoryResponse[business.Response](t, b.request(t, a, http.MethodPost, endpoint+"/messages", map[string]any{
		"requestId": "coordinated-local-conversation", "characterId": "one", "message": "会话必须留在手机",
	}, http.StatusOK))
	if !reply.Saved || reply.ConversationID == "" || reply.Scope.ResourceOwnerID != a.device.DeviceID.String() || reply.Scope.RoleOwnerID != b.core {
		t.Fatalf("coordinated reply ownership is invalid: %+v", reply)
	}
	listed := decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/conversations?characterId=one", nil, http.StatusOK))
	if listed.Snapshot.OwnerID != a.device.DeviceID.String() || listed.Scope.ResourceOwnerID != a.device.DeviceID.String() || listed.Scope.RoleOwnerID != b.core {
		t.Fatalf("conversation list moved to Core: snapshot=%s scope=%+v", listed.Snapshot.OwnerID, listed.Scope)
	}
	foundConversation := false
	for _, resource := range listed.Snapshot.Resources {
		if resource.OwnerID != a.device.DeviceID.String() {
			t.Fatalf("%s/%s conversation resource leaked to %s", resource.Kind, resource.ID, resource.OwnerID)
		}
		if resource.Kind == "conversation" && resource.ID == reply.ConversationID {
			foundConversation = true
		}
	}
	if !foundConversation {
		t.Fatalf("local conversation list is incomplete: resources=%d", len(listed.Snapshot.Resources))
	}
	detail := decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/conversations/"+reply.ConversationID+"?characterId=one", nil, http.StatusOK))
	foundUserMessage, foundAssistantMessage := false, false
	for _, resource := range detail.Snapshot.Resources {
		if resource.OwnerID != a.device.DeviceID.String() {
			t.Fatalf("%s/%s message resource leaked to %s", resource.Kind, resource.ID, resource.OwnerID)
		}
		if resource.Kind == "message" {
			var body struct {
				Role string `json:"role"`
			}
			if err := json.Unmarshal(resource.Body, &body); err != nil {
				t.Fatal(err)
			}
			foundUserMessage = foundUserMessage || body.Role == "user"
			foundAssistantMessage = foundAssistantMessage || body.Role == "assistant"
		}
	}
	if !foundUserMessage || !foundAssistantMessage {
		t.Fatalf("local conversation messages are incomplete: user=%t assistant=%t", foundUserMessage, foundAssistantMessage)
	}
}
