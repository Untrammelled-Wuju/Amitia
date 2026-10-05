package main

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestOwnedToolResultIsAcknowledgedInPhysicalDeviceStore(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES(?,'core','trusted','now','now')`, p.ownerID); err != nil {
		t.Fatal(err)
	}
	service := coordination.NewService(db)
	ctx, scope, finish, err := service.Begin(t.Context(), "core", p.ownerID, p.ownerID, "core", "one", "request")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	scope.RoleRevision = 3
	ctx = coordination.WithScope(ctx, scope)
	runtime := &devicemesh.Runtime{Coordination: service, LocalDeviceID: p.ownerID, LocalDeviceDataPort: p}
	p.services.DeviceMesh = runtime
	r := &meshOwnedToolRuntime{services: p.services}
	result := capability.NewToolSuccessResult("invoke", "tool")
	result.Structured = json.RawMessage(`{"text":"device-private"}`)
	reference, err := r.saveOwnedToolResult(ctx, scope, "owned/action", "fingerprint", result)
	if err != nil || reference.OwnerID != p.ownerID || !reference.OwnedToolResult {
		t.Fatalf("owner acknowledgement missing: %+v %v", reference, err)
	}
	resource, err := p.Resource(ctx, scope, "tool-result", "owned/action")
	if err != nil || resource == nil || resource.OwnerID != p.ownerID || resource.Revision != 1 {
		t.Fatalf("device result missing: %+v %v", resource, err)
	}
	var saved ownedToolResultDocument
	if err := json.Unmarshal(resource.Body, &saved); err != nil || saved.Scope != scope || saved.Fingerprint != "fingerprint" || string(saved.Result.Structured) != string(result.Structured) {
		t.Fatalf("owner document incomplete: %+v %v", saved, err)
	}
	if _, err := r.saveOwnedToolResult(ctx, scope, "owned/action", "fingerprint", result); err != nil {
		t.Fatalf("confirmed result replay failed: %v", err)
	}
	result.Structured = json.RawMessage(`{"text":"different"}`)
	if _, err := r.saveOwnedToolResult(ctx, scope, "owned/action", "fingerprint", result); err == nil {
		t.Fatal("same action overwrote confirmed output")
	}
	var otherOwnerCount int
	if err := db.QueryRow(`SELECT count(*) FROM kernel_device_owned_resources WHERE kind='tool-result' AND owner_id<>?`, p.ownerID).Scan(&otherOwnerCount); err != nil || otherOwnerCount != 0 {
		t.Fatalf("result mirrored to Core: %d %v", otherOwnerCount, err)
	}
}
