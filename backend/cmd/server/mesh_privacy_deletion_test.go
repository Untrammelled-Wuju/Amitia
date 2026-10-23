package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestPrivacyDeletionRealTLSModesAndActualReadBlocking(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	phone := newThreeCoreFixture(t, "delete-phone", schemas, runtimeidentity.PlatformAndroid)
	core := newThreeCoreFixture(t, "delete-core", schemas)
	desktop := newThreeCoreFixture(t, "delete-desktop", schemas)
	registerPrivacyFixture(core)
	pairThreeCoreFixtures(t, phone, core)
	pairThreeCoreFixtures(t, desktop, core)
	const endpoint = "/api/privacy/deletion/"
	for _, coordinated := range []bool{false, true} {
		if coordinated {
			policy, err := core.services.DeviceMesh.Coordination.Get(t.Context(), core.core, phone.device.DeviceID.String())
			if err != nil {
				t.Fatal(err)
			}
			core.request(t, phone, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
		}
		id := uuid.NewString()
		reply := decodeThreeCoreMemoryResponse[business.Response](t, core.request(t, phone, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": id, "characterId": "one", "message": "需要删除的私有测试消息"}, 200))
		<-core.model.roles
		query := map[string]any{"targetId": id + "/user", "targetType": "message", "scope": "all"}
		preview := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, endpoint+"preview", query, 200))
		targets := preview["targets"].([]any)
		if len(targets) != 1 {
			t.Fatalf("targets=%v", preview)
		}
		query["target"], query["requestId"] = targets[0], uuid.NewString()
		core.request(t, desktop, http.MethodPost, endpoint+"request", query, 409)
		result := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, endpoint+"request", query, 200))
		if result["retrievalBlocked"] != true || result["status"] != "completed" {
			t.Fatalf("unconfirmed deletion: %+v", result)
		}
		storeOwner := phone
		if coordinated {
			storeOwner = core
		}
		row, err := coordination.NewOwnershipStore(storeOwner.services.KernelContainer.DeviceRegistry.Database(), reply.Scope.ResourceOwnerID).Get(t.Context(), "message", id+"/user")
		if err != nil || row == nil || !row.Deleted || string(row.Body) != "null" {
			t.Fatalf("message not erased: %+v %v", row, err)
		}
		view := decodeThreeCoreMemoryResponse[business.QueryResult](t, core.request(t, phone, http.MethodGet, "/api/device-mesh/v1/business/conversations/"+reply.ConversationID+"?characterId=one&resourceKind=message", nil, 200))
		for _, resource := range view.Snapshot.Resources {
			if resource.ID == id+"/user" {
				t.Fatal("deleted message still readable")
			}
		}
		core.request(t, phone, http.MethodPost, endpoint+"request", query, 200)
		status := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodGet, endpoint+"status/"+result["id"].(string), nil, 200))
		if status["status"] != "completed" {
			t.Fatal(status)
		}
		checks := decodeThreeCoreMemoryResponse[[]map[string]any](t, core.request(t, phone, http.MethodPost, endpoint+"security-tests", map[string]any{"targetId": id + "/user", "targetType": "message"}, 200))
		if len(checks) != 1 || checks[0]["passed"] != true {
			raw, _ := json.Marshal(checks)
			t.Fatal(string(raw))
		}
		if _, err := storeOwner.services.KernelContainer.DeviceRegistry.Database().Exec(`UPDATE kernel_device_owned_resources SET deleted=0,body='{"content":"unexpected restoration"}' WHERE owner_id=? AND kind='message' AND resource_id=?`, reply.Scope.ResourceOwnerID, id+"/user"); err != nil {
			t.Fatal(err)
		}
		changed := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodGet, endpoint+"status/"+result["id"].(string), nil, 200))
		if changed["status"] != "failed" || changed["retrievalBlocked"] != false {
			t.Fatalf("restored source falsely reported erased: %v", changed)
		}

	}
	core.request(t, phone, http.MethodPost, endpoint+"security-tests", map[string]any{"targetId": "nonexistent", "targetType": "message"}, 409)
}

func TestPrivacyDeletionLegacyHistoricalAndFailureRetry(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	phone := newThreeCoreFixture(t, "legacy-delete-phone", schemas, runtimeidentity.PlatformAndroid)
	core := newThreeCoreFixture(t, "legacy-delete-core", schemas)
	registerPrivacyFixture(core)
	pairThreeCoreFixtures(t, phone, core)
	port := phone.services.DeviceMesh.LocalDeviceDataPort.(*meshLocalDataPort)
	for _, row := range []chat.Conversation{{ID: "legacy-delete", SpaceID: port.legacySpaceID}, {ID: "unrelated", SpaceID: "foreign-space"}} {
		if err := phone.services.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []chat.Message{{ID: "legacy-delete-one", ConversationID: "legacy-delete", CharacterID: "one", Role: "user", Content: "private legacy", ReasoningContent: "private reasoning", Revision: 1}, {ID: "legacy-delete-two", ConversationID: "legacy-delete", CharacterID: "one", Role: "assistant", Content: "sibling", Revision: 1}, {ID: "foreign-message", ConversationID: "unrelated", CharacterID: "one", Role: "user", Content: "foreign", Revision: 1}} {
		if err := phone.services.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	remove := func(id string, fail bool) map[string]any {
		t.Helper()
		input := map[string]any{"targetId": id, "targetType": "message", "scope": "all"}
		preview := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, "/api/privacy/deletion/preview", input, 200))
		targets := preview["targets"].([]any)
		if len(targets) != 1 {
			t.Fatalf("preview: %v", preview)
		}
		input["target"], input["requestId"] = targets[0], uuid.NewString()
		if fail {
			if err := phone.services.DB.Exec("ALTER TABLE message_feedback RENAME TO message_feedback_paused").Error; err != nil {
				t.Fatal(err)
			}
		}
		result := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, "/api/privacy/deletion/request", input, 200))
		if fail {
			if result["retrievalBlocked"] == true || result["status"] != "failed" {
				t.Fatalf("false success after failure: %v", result)
			}
			if err := phone.services.DB.Exec("ALTER TABLE message_feedback_paused RENAME TO message_feedback").Error; err != nil {
				t.Fatal(err)
			}
			core.request(t, phone, http.MethodPost, "/api/privacy/deletion/cleanup", nil, 200)
			result = decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodGet, "/api/privacy/deletion/status/"+result["id"].(string), nil, 200))
		}
		if result["status"] != "completed" {
			t.Fatalf("legacy not erased: %v", result)
		}
		var message chat.Message
		if err := phone.services.DB.Where("id=?", id).First(&message).Error; err != nil {
			t.Fatal(err)
		}
		if message.DeletedAt == nil || message.Content != "" || message.ReasoningContent != "" {
			t.Fatalf("legacy source retained payload: %+v", message)
		}
		return result
	}
	remove("legacy-delete-one", true)
	var kept chat.Message
	if err := phone.services.DB.Where("id='legacy-delete-two' AND deleted_at IS NULL").First(&kept).Error; err != nil {
		t.Fatal("sibling was deleted")
	}
	policy, err := core.services.DeviceMesh.Coordination.Get(t.Context(), core.core, phone.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	core.request(t, phone, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
	result := remove("legacy-delete-two", false)
	if result["ownerId"] != phone.device.DeviceID.String() {
		t.Fatal("historical deletion was routed to Core")
	}
	stats := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodGet, "/api/privacy/deletion/stats", nil, 200))
	if stats["total"] != float64(2) {
		t.Fatalf("owner stats missing: %v", stats)
	}
	foreign := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, "/api/privacy/deletion/preview", map[string]any{"targetId": "foreign-message", "targetType": "message", "scope": "all"}, 200))
	if len(foreign["targets"].([]any)) != 0 {
		t.Fatal("foreign space exposed in preview")
	}
}

func TestPrivacyDeletionConversationAndCharacterScopes(t *testing.T) {
	for _, kind := range []string{"conversation", "character"} {
		t.Run(kind, func(t *testing.T) {
			p := setupMeshLocalDataPort(t)
			scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "delete-scopes", InitiatorDeviceID: "caller"}
			seedOwnedChatManagement(t, p, "conversation", "delete-scope", "one", `{"title":"private"}`)
			seedOwnedChatManagement(t, p, "message", "delete-message", "one", `{"id":"delete-message","conversationId":"delete-scope","requestId":"turn-private","content":"private","role":"user"}`)
			seedOwnedChatManagement(t, p, "checkpoint", "turn/turn-private", "one", `{"conversationId":"delete-scope","status":"completed","response":{"text":"private"}}`)
			seedOwnedChatManagement(t, p, "message", "other-role", "two", `{"id":"other-role","conversationId":"safe","content":"safe"}`)
			input := &coordination.PrivacyDeletionRequest{TargetType: kind, TargetID: "delete-scope", Selection: "all"}
			if kind == "character" {
				input.TargetID = "one"
			}
			preview, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "privacy/deletion-preview", ConversationOwnerID: p.ownerID, PrivacyDeletion: input})
			if err != nil {
				t.Fatal(err)
			}
			targets := preview["targets"].([]coordination.PrivacyDeletionTarget)
			if len(targets) != 1 {
				t.Fatalf("targets: %v", targets)
			}
			input.Target = &targets[0]
			result, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "privacy/deletion-request", ConversationOwnerID: p.ownerID, RequestID: uuid.NewString(), PrivacyDeletion: input})
			if err != nil || result["status"] != "completed" {
				t.Fatalf("delete: %v %v", result, err)
			}
			for _, key := range []struct{ kind, id string }{{"conversation", "delete-scope"}, {"message", "delete-message"}, {"checkpoint", "turn/turn-private"}} {
				row, err := p.store.Get(t.Context(), key.kind, key.id)
				if err != nil || row == nil || !row.Deleted {
					t.Fatalf("retained: %v %v", row, err)
				}
			}
			row, err := p.store.Get(t.Context(), "message", "other-role")
			if err != nil || row == nil || row.Deleted {
				t.Fatal("unrelated role deleted")
			}
		})
	}
}

func TestPrivacyDeletionMemoryPreviewAndReplay(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	phone := newThreeCoreFixture(t, "memory-delete-phone", schemas, runtimeidentity.PlatformAndroid)
	core := newThreeCoreFixture(t, "memory-delete-core", schemas)
	registerPrivacyFixture(core)
	pairThreeCoreFixtures(t, phone, core)
	port := phone.services.DeviceMesh.LocalDeviceDataPort.(*meshLocalDataPort)
	seedOwnedChatManagement(t, port, "memory", "memory-delete-private", "one", `{"kind":"fact","key":"private","content":{"value":"private memory"},"createdAt":"2026-10-23T00:00:00Z","memoryStrength":0.7,"retentionLevel":3}`)
	input := map[string]any{"targetId": "memory-delete-private", "targetType": "memory", "scope": "all"}
	preview := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, "/api/privacy/deletion/preview", input, 200))
	targets := preview["targets"].([]any)
	if len(targets) != 1 {
		t.Fatal(preview)
	}
	input["target"], input["requestId"] = targets[0], uuid.NewString()
	result := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, "/api/privacy/deletion/request", input, 200))
	if result["status"] != "completed" || result["retrievalBlocked"] != true {
		t.Fatal(result)
	}
	replay := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, "/api/privacy/deletion/request", input, 200))
	if replay["id"] != result["id"] {
		t.Fatal("memory replay lost receipt")
	}
}

func TestPrivacyDeletionLegacyJobMigrationAndVersionFence(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	phone := newThreeCoreFixture(t, "migrate-delete-phone", schemas)
	core := newThreeCoreFixture(t, "migrate-delete-core", schemas)
	registerPrivacyFixture(core)
	pairThreeCoreFixtures(t, phone, core)
	policy, err := core.services.DeviceMesh.Coordination.Get(t.Context(), core.core, phone.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	core.request(t, phone, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
	policy, err = core.services.DeviceMesh.Coordination.Get(t.Context(), core.core, phone.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), core.core, phone.device.DeviceID.String(), policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	reply := decodeThreeCoreMemoryResponse[business.Response](t, core.request(t, phone, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": id, "characterId": "one", "message": "legacy task migration"}, 200))
	<-core.model.roles
	legacy := mindruntime.NewDataLifecycleCoordinator(core.services.DB)
	if err := legacy.InitSchema(); err != nil {
		t.Fatal(err)
	}
	tombstone, err := legacy.RequestDeletion(mindruntime.DeletionRequest{TargetID: id + "/user", TargetType: "message", Scope: mindruntime.DeletionScopeAll})
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"targetId": id + "/user", "targetType": "message", "scope": "all", "legacyDeletionId": tombstone.ID}
	preview := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, "/api/privacy/deletion/preview", input, 200))
	targets := preview["targets"].([]any)
	if len(targets) != 1 {
		t.Fatal(preview)
	}
	input["target"], input["requestId"] = targets[0], uuid.NewString()
	if _, err := core.services.KernelContainer.DeviceRegistry.Database().Exec(`UPDATE kernel_device_owned_resources SET revision=revision+1 WHERE owner_id=? AND kind='message' AND resource_id=?`, reply.Scope.ResourceOwnerID, id+"/user"); err != nil {
		t.Fatal(err)
	}
	core.request(t, phone, http.MethodPost, "/api/privacy/deletion/request", input, 409)
	preview = decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, "/api/privacy/deletion/preview", input, 200))
	input["target"], input["requestId"] = preview["targets"].([]any)[0], uuid.NewString()
	result := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, "/api/privacy/deletion/request", input, 200))
	if result["status"] != "completed" {
		t.Fatal(result)
	}
	var migrated mindruntime.DeletionTombstoneModel
	if err := core.services.DB.Where("id=?", tombstone.ID).First(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	if migrated.MigratedTo != result["id"] || migrated.Status != "migrated" {
		t.Fatal("old task not linked to real receipt")
	}
	status := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodGet, "/api/privacy/deletion/status/"+tombstone.ID, nil, 200))
	if status["status"] != "completed" {
		t.Fatal(status)
	}
}
