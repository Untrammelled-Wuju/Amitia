package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/temporal"
)

func TestRelationshipTimeRealTLSOwnerModes(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	phone := newThreeCoreFixture(t, "relationship-phone", schemas, runtimeidentity.PlatformAndroid)
	core := newThreeCoreFixture(t, "relationship-core", schemas)
	for _, f := range []*threeCoreFixture{phone, core} {
		runner := migration.Runner{DB: f.services.DB, SkipBackup: true}
		if err := runner.Apply([]migration.Migration{migration.TemporalRelationshipTimeMigration()}); err != nil {
			t.Fatal(err)
		}
		f.services.RelTimeCoordinator = temporal.NewRelationshipTimeCoordinator(temporal.NewRelationshipTimeRepository(f.services.DB, temporal.SystemClock{}), temporal.SystemClock{})
	}
	pairThreeCoreFixtures(t, phone, core)
	path := "/api/device-mesh/v1/business/relationship-time?characterId=one"
	read := func() map[string]any {
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
	send("device-first")
	device := read()
	if device["ownerId"] != phone.device.DeviceID.String() || device["state"].(map[string]any)["interactionCount"] != float64(1) {
		t.Fatalf("device owner state: %+v", device)
	}
	settings := device["settings"].(map[string]any)
	settings["maxMentionSentences"] = float64(0)
	payload := map[string]any{"settings": settings, "expectedExecutionScope": device["executionScope"]}
	core.request(t, phone, http.MethodPut, path, payload, 200)
	core.request(t, phone, http.MethodPut, path, payload, 409)
	if read()["settings"].(map[string]any)["maxMentionSentences"] != float64(0) {
		t.Fatal("remote settings not saved")
	}
	policy, err := core.services.DeviceMesh.Coordination.Get(t.Context(), core.core, phone.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	core.request(t, phone, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
	core.request(t, phone, http.MethodPut, path, payload, 409)
	before := read()
	if before["ownerId"] != core.core || before["state"].(map[string]any)["interactionCount"] != float64(0) || before["settings"].(map[string]any)["maxMentionSentences"] != float64(1) {
		t.Fatalf("owner isolation failed: %+v", before)
	}
	send("core-first")
	after := read()
	if after["state"].(map[string]any)["interactionCount"] != float64(1) {
		t.Fatal("core completion missing")
	}
	var sourceRows int64
	if err := core.services.DB.Model(&temporal.RelationshipPresenceState{}).Where("space_id=?", temporal.RelationshipOwnerSpace(schemas.legacySpace, phone.device.DeviceID.String())).Count(&sourceRows).Error; err != nil || sourceRows != 0 {
		t.Fatalf("remote state mirrored in core: %d %v", sourceRows, err)
	}
}
