package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type memoryCandidateHTTPModel struct{ *threeCoreMemoryModel }

func (m *memoryCandidateHTTPModel) ExtractOwnedMemory(_ context.Context, input business.Inference, _ business.Generation) ([]business.DerivedMemory, error) {
	result := []business.DerivedMemory{}
	for _, kind := range []string{"fact", "profile", "episodic"} {
		value, err := json.Marshal(map[string]any{"value": input.Message, "memoryType": "fact", "importance": 5})
		if err != nil {
			return nil, err
		}
		result = append(result, business.DerivedMemory{Kind: kind, Key: kind + "-preference", Body: value})
	}
	return result, nil
}

func TestActualTLSMemoryManagementPreservesOwnerAndRejectsOldIntent(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "memory-manager-a", schemas), newThreeCoreFixture(t, "memory-manager-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	const endpoint = "/api/device-mesh/v1/business"
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, b, a, policy.ModeRevision)
	staleView := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodGet, endpoint+"/memories?characterId=one", nil, 200))
	policy, err = b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), b.core, a.device.DeviceID.String(), policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	view := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodGet, endpoint+"/memories?characterId=one", nil, 200))
	if len(view.Resources) != 0 || view.Scope.ResourceOwnerID != b.core {
		t.Fatal("memory manager selected another data owner")
	}
	requestID := uuid.NewString()
	payload := map[string]any{"requestId": requestID, "characterId": "one", "expectedExecutionScope": view.Scope, "action": "create", "key": "饮品", "value": "喜欢茶", "memoryType": "preference", "importance": 7}
	created := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodPost, endpoint+"/memories/manage", payload, 200))
	if !created.Saved || created.Acknowledgement.OwnerID != b.core || created.Acknowledgement.Versions["checkpoint/memory-management/"+requestID] != 1 {
		t.Fatal("manual memory was not acknowledged by Source")
	}
	resourceID := ""
	for _, resource := range created.Resources {
		if resource.Kind == "memory" {
			resourceID = resource.ID
		}
	}
	if resourceID == "" {
		t.Fatal("manual memory did not create an owned memory resource")
	}
	replayed := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodPost, endpoint+"/memories/manage", payload, 200))
	if string(replayed.Resources[0].Body) != string(created.Resources[0].Body) {
		t.Fatal("retry repeated memory derivation")
	}
	payload["requestId"], payload["value"] = uuid.NewString(), "喜欢咖啡"
	var conflict struct {
		Resources []coordination.Resource `json:"conflicts"`
	}
	if json.Unmarshal(b.request(t, a, http.MethodPost, endpoint+"/memories/manage", payload, 409), &conflict) != nil || len(conflict.Resources) != 1 || conflict.Resources[0].ID != resourceID {
		t.Fatal("conflict did not return the original owned resource")
	}
	payload["action"], payload["resolution"], payload["conflictId"] = "resolve", "merge", resourceID
	payload["expectedConflictRevision"] = conflict.Resources[0].Revision + 1
	b.request(t, a, http.MethodPost, endpoint+"/memories/manage", payload, 409)
	payload["expectedConflictRevision"] = conflict.Resources[0].Revision
	resolved := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodPost, endpoint+"/memories/manage", payload, 200))
	if resolved.Acknowledgement.Versions["memory/"+resourceID] != 2 {
		t.Fatal("memory conflict resolution did not use the original revision")
	}
	filtered := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodGet, endpoint+"/memories?characterId=one&query=咖啡&memoryType=preference&mode=keyword&limit=1", nil, 200))
	if len(filtered.Resources) != 1 || filtered.Resources[0].ID != resourceID || filtered.Resources[0].OwnerID != b.core {
		t.Fatal("memory search mixed owners or ignored filters")
	}
	b.request(t, a, http.MethodGet, endpoint+"/memories?characterId=one&limit=129", nil, 400)
	b.request(t, a, http.MethodPost, endpoint+"/memories/manage", map[string]any{"requestId": uuid.NewString(), "characterId": "one", "action": "create"}, 400)
	b.request(t, a, http.MethodPost, endpoint+"/memories/manage", map[string]any{"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": staleView.Scope, "action": "create", "key": "旧请求", "value": "不得保存", "memoryType": "fact"}, 409)
	assertThreeCorePrivateCopies(t, b, a.device.DeviceID.String())
	rows, err := coordination.NewOwnershipStore(b.services.KernelContainer.DeviceRegistry.Database(), b.core).List(t.Context(), "memory", "one", false)
	if err != nil || len(rows) != 1 {
		t.Fatal("manual memory did not remain in the Core database")
	}
}

func TestActualTLSMemoryCandidatesUseHistoricalSourceAndCurrentCoreOwner(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "candidate-a", schemas), newThreeCoreFixture(t, "candidate-b", schemas)
	b.services.OwnedBusiness = business.NewEngine(b.services.DeviceMesh.Coordination, b.services.DeviceMesh, &memoryCandidateHTTPModel{threeCoreMemoryModel: &threeCoreMemoryModel{core: b.core}})
	pairThreeCoreFixtures(t, a, b)
	const endpoint = "/api/device-mesh/v1/business"
	conversation := decodeThreeCoreMemoryResponse[business.Response](t, b.request(t, a, http.MethodPost, endpoint+"/messages", map[string]any{"requestId": uuid.NewString(), "characterId": "one", "message": "喜欢散步和茶"}, 200))
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
	view := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodGet, endpoint+"/memory-candidates?characterId=one", nil, 200))
	requestID := uuid.NewString()
	payload := map[string]any{"requestId": requestID, "characterId": "one", "historicalRoleId": "one", "expectedExecutionScope": view.Scope, "action": "generate", "conversationId": conversation.ConversationID, "conversationOrigin": business.ConversationOrigin{OwnerID: a.device.DeviceID.String(), ID: conversation.ConversationID}}
	generated := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodPost, endpoint+"/memory-candidates/manage", payload, 200))
	if !generated.Saved || generated.Scope.RequestID != requestID || generated.Acknowledgement.OwnerID != b.core || generated.Acknowledgement.Versions["checkpoint/memory-candidate-operation/"+requestID] != 2 {
		t.Fatal("candidate generation did not preserve external intent and current Core acknowledgement")
	}
	replayed := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodPost, endpoint+"/memory-candidates/manage", payload, 200))
	if len(replayed.Resources) != len(generated.Resources) || replayed.Scope.RequestID != requestID {
		t.Fatal("candidate replay changed the original operation")
	}
	listed := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodGet, endpoint+"/memory-candidates?characterId=one", nil, 200))
	if len(listed.Resources) != 3 {
		t.Fatalf("historical Source conversation produced %d candidates", len(listed.Resources))
	}
	row := listed.Resources[0]
	var candidate business.OwnedMemoryCandidate
	if json.Unmarshal(row.Body, &candidate) != nil || candidate.ConversationOrigin == nil || candidate.ConversationOrigin.OwnerID != a.device.DeviceID.String() || row.OwnerID != b.core {
		t.Fatal("candidate lost its historical provenance or current owner")
	}
	accept := map[string]any{"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": listed.Scope, "action": "accept", "id": row.ID, "expectedRevision": row.Revision + 1}
	b.request(t, a, http.MethodPost, endpoint+"/memory-candidates/manage", accept, 409)
	accept["expectedRevision"] = row.Revision
	accepted := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodPost, endpoint+"/memory-candidates/manage", accept, 200))
	if !accepted.Saved || accepted.Acknowledgement.OwnerID != b.core || accepted.Acknowledgement.Versions["checkpoint/"+row.ID] != row.Revision+1 {
		t.Fatal("candidate acceptance did not use original CAS and current Core storage")
	}
	large := map[string]any{"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": listed.Scope, "action": "create", "key": "大记忆", "value": strings.Repeat("a", 70<<10), "memoryType": "fact"}
	created := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodPost, endpoint+"/memories/manage", large, 200))
	if !created.Saved || created.Acknowledgement.OwnerID != b.core {
		t.Fatal("valid memory larger than 64 KiB was rejected or saved at another owner")
	}
	large["requestId"], large["value"] = uuid.NewString(), strings.Repeat("a", 129<<10)
	b.request(t, a, http.MethodPost, endpoint+"/memories/manage", large, 409)
	large["requestId"], large["value"] = uuid.NewString(), strings.Repeat("a", 321<<10)
	b.request(t, a, http.MethodPost, endpoint+"/memories/manage", large, 400)
	assertThreeCorePrivateCopies(t, b, a.device.DeviceID.String())
}
