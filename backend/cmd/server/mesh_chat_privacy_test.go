package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/pkg/app"
)

func registerPrivacyFixture(core *threeCoreFixture) {
	consumers := &ownedMemoryConsumers{runtime: &canonicalChatMemoryRuntime{engine: core.services.OwnedBusiness, spaceID: core.core, deviceID: core.device.DeviceID.String()}}
	handler := system.NewHandler(system.NewService(app.NewAppContext(core.services.DB, nil), ""), core.services.DB, nil, nil, nil, nil, consumers)
	system.WithCanonicalMemory(core.services.OwnedBusiness, core.core)(handler)
	auth := security.AuthenticationMiddleware(security.AuthConfig{Mode: "network", SpaceID: core.core, DeviceCredentials: core.services.DeviceMesh.CredentialSvc, DeviceRegistry: core.services.KernelContainer.DeviceRegistry, Coordination: core.services.DeviceMesh.Coordination})
	api := core.router.Group("/api/privacy", auth, handler.PrivacyAuthority())
	api.POST("/scan", handler.PrivacyScan)
	api.POST("/mask", handler.PrivacyMask)
	api.GET("/scan-results", handler.PrivacyScanResultsGet)
	api.GET("/scan-results/:id", handler.PrivacyScanResultByID)
	api.DELETE("/scan-results", handler.PrivacyScanResults)
	api.POST("/deletion/preview", handler.PrivacyDeletionPreview)
	api.POST("/deletion/request", handler.PrivacyDeletionRequest)
	api.GET("/deletion/status/:id", handler.PrivacyDeletionStatus)
	api.GET("/deletion/stats", handler.PrivacyDeletionStats)
	api.POST("/deletion/cleanup", handler.PrivacyDeletionCleanup)
	api.POST("/deletion/security-tests", handler.PrivacyDeletionSecurityTests)
}

func TestPrivacyRealTLSModesLegacyImportAndVersionConflicts(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	phone := newThreeCoreFixture(t, "privacy-phone", schemas, runtimeidentity.PlatformAndroid)
	core := newThreeCoreFixture(t, "privacy-core", schemas)
	registerPrivacyFixture(core)
	pairThreeCoreFixtures(t, phone, core)
	desktop := newThreeCoreFixture(t, "privacy-desktop", schemas)
	pairThreeCoreFixtures(t, desktop, core)
	scan := func() map[string]any {
		return decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, phone, http.MethodPost, "/api/privacy/scan", map[string]any{"scope": []string{"messages", "import_items"}}, 200))
	}
	send := func(id, text string) businessPrivacyReply {
		raw := core.request(t, phone, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": id, "characterId": "one", "message": text}, 200)
		<-core.model.roles
		return decodeThreeCoreMemoryResponse[businessPrivacyReply](t, raw)
	}
	mask := func(finding map[string]any, code int) {
		core.request(t, phone, http.MethodPost, "/api/privacy/mask", map[string]any{"items": []map[string]any{{"id": finding["id"], "sourceTable": finding["source_table"]}}, "confirmToken": "确认脱敏"}, code)
	}
	reply := send("phone-private", "password device-secret")
	if reply.Scope.ResourceOwnerID != phone.device.DeviceID.String() {
		t.Fatal("wrong initial owner")
	}
	legacy := chat.Conversation{ID: "legacy-private", SpaceID: schemas.legacySpace, Source: "import"}
	port := phone.services.DeviceMesh.LocalDeviceDataPort.(*meshLocalDataPort)
	legacy.SpaceID = port.legacySpaceID
	if err := phone.services.DB.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := phone.services.DB.Create(&chat.Message{ID: "legacy-secret", ConversationID: legacy.ID, CharacterID: "one", Role: "user", Content: "token old-secret", Revision: 1}).Error; err != nil {
		t.Fatal(err)
	}
	seedOwnedChatManagement(t, port, "conversation", "imported-private", "one", `{"source":"import"}`)
	seedOwnedChatManagement(t, port, "message", "imported-secret", "one", `{"conversationId":"imported-private","role":"user","content":"api_key imported-secret","id":"imported-secret"}`)
	result := scan()
	findings := result["findings"].([]any)
	isolated := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, desktop, http.MethodGet, "/api/privacy/scan-results", nil, 200))
	if isolated["historyTotal"] != float64(0) {
		t.Fatal("another device can read privacy history")
	}
	foreign := findings[0].(map[string]any)
	core.request(t, desktop, http.MethodPost, "/api/privacy/mask", map[string]any{"items": []map[string]any{{"id": foreign["id"], "sourceTable": foreign["source_table"]}}, "confirmToken": "确认脱敏"}, 409)

	if len(findings) != 3 || result["status"] != "completed" {
		t.Fatalf("incomplete scan: %+v", result)
	}
	for _, raw := range findings {
		mask(raw.(map[string]any), 200)
	}
	if len(scan()["findings"].([]any)) != 0 {
		t.Fatal("masked source still reported sensitive")
	}
	stored, err := coordination.NewOwnershipStore(phone.services.KernelContainer.DeviceRegistry.Database(), phone.device.DeviceID.String()).Get(t.Context(), "message", "phone-private/user")
	if err != nil || stored == nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.Unmarshal(stored.Body, &body)
	if body["content"] != "[已脱敏]" || stored.Revision != 2 {
		t.Fatal("device did not persist redaction")
	}
	assertThreeCorePrivateCopies(t, core, phone.device.DeviceID.String())
	send("stale-private", "password before")
	old := scan()["findings"].([]any)[0].(map[string]any)
	if _, err := phone.services.KernelContainer.DeviceRegistry.Database().Exec(`UPDATE kernel_device_owned_resources SET revision=revision+1 WHERE owner_id=? AND resource_id='stale-private/user'`, phone.device.DeviceID.String()); err != nil {
		t.Fatal(err)
	}
	mask(old, 409)
	policy, err := core.services.DeviceMesh.Coordination.Get(t.Context(), core.core, phone.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	core.request(t, phone, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
	mask(old, 409)
	send("cloud-private", "password cloud-secret")
	result = scan()
	findings = result["findings"].([]any)
	if len(findings) != 2 {
		t.Fatalf("coordinated and historical coverage: %+v", result)
	}
	for _, raw := range findings {
		mask(raw.(map[string]any), 200)
	}
	if len(scan()["findings"].([]any)) != 0 {
		t.Fatal("cloud or historical redaction missing")
	}
	if _, err := core.services.DB.DB(); err != nil {
		t.Fatal(err)
	}
	if err := core.services.DB.Exec("DROP TABLE messages").Error; err != nil {
		t.Fatal(err)
	}
	core.request(t, phone, http.MethodPost, "/api/privacy/scan", map[string]any{"scope": []string{"messages"}}, 503)
}

type businessPrivacyReply struct {
	Scope coordination.ExecutionScope `json:"executionScope"`
}

func TestPrivacyPagesIncludeDisabledRolesBeyondLegacyLimit(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1101; i++ {
		id := fmt.Sprintf("page-%04d", i)
		raw, _ := json.Marshal(map[string]any{"id": id, "conversationId": "private-pages", "content": "password fixture", "role": "user"})
		if _, err := tx.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,deleted,body,updated_at) VALUES(?,'message',?,'disabled-role',1,0,?,'2026-10-23T00:00:00Z')`, p.ownerID, id, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "privacy-pages"}
	seen := map[string]bool{}
	fingerprint := ""
	for page := 1; page <= 18; page++ {
		result, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "privacy/messages", ConversationOwnerID: p.ownerID, Page: page, PageSize: 64, PreviewID: fingerprint})
		if err != nil {
			t.Fatal(err)
		}
		fingerprint = result["snapshotFingerprint"].(string)
		for _, item := range result["items"].([]coordination.PrivacyMessage) {
			if seen[item.ID] {
				t.Fatal("duplicate page")
			}
			seen[item.ID] = true
			if item.RoleID != "disabled-role" {
				t.Fatal("lost original role")
			}
		}
	}
	if len(seen) != 1101 {
		t.Fatalf("scanned %d", len(seen))
	}
	if _, err := db.Exec(`UPDATE kernel_device_owned_resources SET revision=2 WHERE resource_id='page-0000'`); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "privacy/messages", ConversationOwnerID: p.ownerID, Page: 2, PageSize: 64, PreviewID: fingerprint}); err == nil {
		t.Fatal("changed scan snapshot accepted")
	}
}

func TestPrivacyStandaloneCorePersistsRedaction(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	local := newThreeCoreFixture(t, "privacy-local", schemas)
	port := local.services.DeviceMesh.LocalDeviceDataPort.(*meshLocalDataPort)
	seedOwnedChatManagement(t, port, "message", "local-secret", "one", `{"id":"local-secret","content":"password local fixture","role":"user"}`)
	request := business.Request{SpaceID: local.core, DeviceID: local.device.DeviceID.String(), CoreID: local.core}
	entries, err := local.services.OwnedBusiness.PrivacyMessages(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("local entries: %d", len(entries))
	}
	if err := local.services.OwnedBusiness.MaskPrivacyMessage(t.Context(), request, entries[0], "local-redaction"); err != nil {
		t.Fatal(err)
	}
	row, err := port.store.Get(t.Context(), "message", "local-secret")
	if err != nil || row == nil || row.Revision != 2 {
		t.Fatalf("local save: %+v %v", row, err)
	}
}
