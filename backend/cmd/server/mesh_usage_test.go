package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/pkg/app"
)

func TestUsageRealTLSOwnerModesAndReset(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	phone := newThreeCoreFixture(t, "usage-phone", schemas, runtimeidentity.PlatformAndroid)
	core := newThreeCoreFixture(t, "usage-core", schemas)
	store, err := mindruntime.NewModelUsageStore(core.services.DB, core.core)
	if err != nil {
		t.Fatal(err)
	}
	handler := system.NewHandler(system.NewService(app.NewAppContext(core.services.DB, nil), ""), core.services.DB, nil, nil, nil, nil, nil)
	system.WithModelUsage(store)(handler)
	system.WithCanonicalMemory(core.services.OwnedBusiness, core.core)(handler)
	authMW := security.AuthenticationMiddleware(security.AuthConfig{Mode: "network", SpaceID: core.core, DeviceCredentials: core.services.DeviceMesh.CredentialSvc, DeviceRegistry: core.services.KernelContainer.DeviceRegistry, Coordination: core.services.DeviceMesh.Coordination})
	api := core.router.Group("/api", authMW)
	api.GET("/usage/overview", handler.UsageOverview)
	api.GET("/usage/daily", handler.UsageDaily)
	api.DELETE("/usage/clear", handler.UsageClear)
	pairThreeCoreFixtures(t, phone, core)
	read := func(path string) map[string]any {
		t.Helper()
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(core.request(t, phone, http.MethodGet, path, nil, 200), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data
	}
	send := func(id string) {
		t.Helper()
		core.request(t, phone, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": id, "characterId": "one", "message": "你好"}, 200)
		<-core.model.roles
	}
	record := func(device string) {
		t.Helper()
		id, err := store.Begin(t.Context(), mindruntime.ModelUsageRecord{DeviceID: device, Model: "model"})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Finish(t.Context(), id, mindruntime.ModelUsageRecord{Status: "succeeded", TotalTokens: 11, UsageReported: true}); err != nil {
			t.Fatal(err)
		}
	}
	record(phone.device.DeviceID.String())
	record("other-device")
	send("phone-request")
	view := read("/api/usage/overview")
	if view["businessOwnerId"] != phone.device.DeviceID.String() || view["messageCount"] != float64(2) || view["conversationCount"] != float64(1) || view["totalRequests"] != float64(1) || view["totalTokens"] != float64(11) {
		t.Fatalf("remote Owner usage or device isolation wrong: %+v", view)
	}
	daily := read("/api/usage/daily")["daily"].([]any)
	if len(daily) != 30 || daily[0].(map[string]any)["messages"] != float64(2) {
		t.Fatalf("remote daily usage: %+v", daily)
	}
	core.request(t, phone, http.MethodDelete, "/api/usage/clear?scope=device&expectedCoreId="+core.core+"&expectedDeviceId="+phone.device.DeviceID.String(), nil, 200)
	view = read("/api/usage/overview")
	if view["totalRequests"] != float64(0) || view["messageCount"] != float64(0) {
		t.Fatalf("baseline not applied: %+v", view)
	}
	var records int64
	if err := core.services.DB.Model(&mindruntime.ModelUsageRecord{}).Count(&records).Error; err != nil || records != 2 {
		t.Fatalf("reset deleted usage records: %d %v", records, err)
	}
	var messages int
	if err := phone.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), "SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='message' AND deleted=0", phone.device.DeviceID.String()).Scan(&messages); err != nil || messages != 2 {
		t.Fatalf("reset changed messages: %d %v", messages, err)
	}
	policy, err := core.services.DeviceMesh.Coordination.Get(t.Context(), core.core, phone.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, core, phone, policy.ModeRevision)
	view = read("/api/usage/overview")
	if view["businessOwnerId"] != core.core || view["messageCount"] != float64(0) {
		t.Fatalf("coordinated Owner not isolated: %+v", view)
	}
	send("coordinated-request")
	view = read("/api/usage/overview")
	if view["businessOwnerId"] != core.core || view["messageCount"] != float64(2) {
		t.Fatalf("coordinated Owner counts missing: %+v", view)
	}
}
