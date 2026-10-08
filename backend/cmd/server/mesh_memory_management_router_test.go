package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

func TestActualTLSAttachmentsStayAtTheirSelectedOwner(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "attachment-a", schemas), newThreeCoreFixture(t, "attachment-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	const endpoint = "/api/device-mesh/v1/business"
	for _, coordinated := range []bool{false, true} {
		if coordinated {
			policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
			if err != nil {
				t.Fatal(err)
			}
			b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
		}
		view := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodGet, endpoint+"/memories?characterId=one", nil, 200))
		for _, kind := range []string{"file", "video"} {
			data, mime := []byte("设备的文件内容"), "text/plain"
			if kind == "video" {
				data, mime = []byte{0, 0, 0, 16, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0}, "video/mp4"
			}
			digest := sha256.Sum256(data)
			item := business.Attachment{Kind: kind, Name: "附件", MIME: mime, Data: base64.StdEncoding.EncodeToString(data), Hash: hex.EncodeToString(digest[:])}
			requestID := uuid.NewString()
			payload := map[string]any{"requestId": requestID, "characterId": "one", "message": "请查看附件", "expectedExecutionScope": view.Scope, "attachments": []business.Attachment{item}}
			response := decodeThreeCoreMemoryResponse[business.Response](t, b.request(t, a, http.MethodPost, endpoint+"/messages", payload, 200))
			owner, holder, other := a.device.DeviceID.String(), a, b
			if coordinated {
				owner, holder, other = b.core, b, a
			}
			if !response.Saved || response.Scope.ResourceOwnerID != owner {
				t.Fatal("attachment response has wrong owner or unconfirmed save")
			}
			row, err := coordination.NewOwnershipStore(holder.services.KernelContainer.DeviceRegistry.Database(), owner).Get(t.Context(), "message", requestID+"/user")
			var document struct {
				Attachments []business.Attachment `json:"attachments"`
			}
			if err != nil || row == nil || json.Unmarshal(row.Body, &document) != nil || len(document.Attachments) != 1 || document.Attachments[0] != item {
				t.Fatal("original attachment binary was not saved at the acknowledged owner")
			}
			if row, err := coordination.NewOwnershipStore(other.services.KernelContainer.DeviceRegistry.Database(), owner).Get(t.Context(), "message", requestID+"/user"); err != nil || row != nil {
				t.Fatal("attachment binary mirrored at another device")
			}
			item.Hash = "tampered"
			payload["requestId"], payload["attachments"] = uuid.NewString(), []business.Attachment{item}
			b.request(t, a, http.MethodPost, endpoint+"/messages", payload, 400)
		}
	}
}

func TestActualTLSMemoryManagementPreservesOwnerAndRejectsOldIntent(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "memory-manager-a", schemas), newThreeCoreFixture(t, "memory-manager-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	const endpoint = "/api/device-mesh/v1/business"
	view := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodGet, endpoint+"/memories?characterId=one", nil, 200))
	if len(view.Resources) != 0 || view.Scope.ResourceOwnerID != a.device.DeviceID.String() {
		t.Fatal("memory manager selected another data owner")
	}
	requestID := uuid.NewString()
	payload := map[string]any{"requestId": requestID, "characterId": "one", "expectedExecutionScope": view.Scope, "action": "create", "key": "饮品", "value": "喜欢茶", "memoryType": "preference", "importance": 7}
	created := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodPost, endpoint+"/memories/manage", payload, 200))
	if !created.Saved || created.Acknowledgement.OwnerID != a.device.DeviceID.String() || created.Acknowledgement.Versions["checkpoint/memory-management/"+requestID] != 1 {
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
	if len(filtered.Resources) != 1 || filtered.Resources[0].ID != resourceID || filtered.Resources[0].OwnerID != a.device.DeviceID.String() {
		t.Fatal("memory search mixed owners or ignored filters")
	}
	b.request(t, a, http.MethodGet, endpoint+"/memories?characterId=one&limit=129", nil, 400)
	b.request(t, a, http.MethodPost, endpoint+"/memories/manage", map[string]any{"requestId": uuid.NewString(), "characterId": "one", "action": "create"}, 400)
	assertThreeCorePrivateCopies(t, b, a.device.DeviceID.String())
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
	payload["requestId"], payload["action"], payload["key"] = uuid.NewString(), "create", "新饮品"
	b.request(t, a, http.MethodPost, endpoint+"/memories/manage", payload, 409)
	current := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodGet, endpoint+"/memories?characterId=one", nil, 200))
	if current.Scope.ResourceOwnerID != b.core || len(current.Resources) != 0 {
		t.Fatal("coordination migrated device memory")
	}
	payload["expectedExecutionScope"] = current.Scope
	coreMemory := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, b.request(t, a, http.MethodPost, endpoint+"/memories/manage", payload, 200))
	if !coreMemory.Saved || coreMemory.Acknowledgement.OwnerID != b.core {
		t.Fatal("coordinated manual memory was not saved at Core")
	}
	for _, fixture := range []*threeCoreFixture{a, b} {
		owner := fixture.core
		if fixture == a {
			owner = a.device.DeviceID.String()
		}
		rows, err := coordination.NewOwnershipStore(fixture.services.KernelContainer.DeviceRegistry.Database(), owner).List(t.Context(), "memory", "one", false)
		if err != nil || len(rows) != 1 {
			t.Fatal("manual memory did not remain in its owner's database")
		}
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
	b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
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
