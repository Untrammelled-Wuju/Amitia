package main

import (
	"context"
	"github.com/u-ai/backend/internal/continuity"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type continuityTestModel struct{}

func (continuityTestModel) GenerateOwnedReply(context.Context, business.Inference) (business.Generation, error) {
	return business.Generation{Text: "已检查", Tokens: 1}, nil
}
func (continuityTestModel) ExtractOwnedMemory(context.Context, business.Inference, business.Generation) ([]business.DerivedMemory, error) {
	return nil, nil
}

func TestMeshContinuityWorkerUsesActualGuardedLocalPortWithoutRecursion(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device-a','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	service := coordination.NewService(db)
	engine := business.NewEngine(service, p, continuityTestModel{})
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	request := business.Request{SpaceID: "core", DeviceID: "device-a", CoreID: "core", RoleID: "one", RequestID: "create"}
	document, _, err := engine.Continuity(ctx, request, business.ContinuityMutation{Action: "create", Title: "检查"})
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().Add(-time.Second)
	request.RequestID = "wait"
	document, _, err = engine.Continuity(ctx, request, business.ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: 1, Action: "add_wait", Wait: &continuity.Wait{WaitType: "time", DueAt: &due, AutoResume: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.TickContinuity(ctx); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "read"
	document, _, err = engine.Continuity(ctx, request, business.ContinuityMutation{ID: document.Thread.ID, Action: "read"})
	if err != nil || document.Lease == nil || document.Lease.State != "completed" {
		t.Fatalf("document=%+v err=%v", document, err)
	}
}
