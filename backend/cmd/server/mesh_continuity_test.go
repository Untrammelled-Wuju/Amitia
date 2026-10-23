package main

import (
	"context"
	"github.com/u-ai/backend/internal/continuity"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
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

func TestLegacyContinuityMigratesToOwnerLedgerIdempotently(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device-a','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	service := coordination.NewService(db)
	engine := business.NewEngine(service, p, continuityTestModel{})
	for _, table := range []string{"continuity_threads", "continuity_thread_bindings", "continuity_waits", "continuity_thread_events"} {
		if !p.services.DB.Migrator().HasTable(table) {
			t.Fatalf("legacy table missing: %s", table)
		}
	}
	now := time.Now().UTC()
	thread := continuity.Thread{ID: "legacy-thread", SpaceID: "core", CharacterID: "one", Title: "旧持续事项", Goal: "迁移后继续", Status: continuity.ThreadStatusWaiting, Summary: "迁移摘要", CurrentState: "等待依赖", NextAction: "依赖完成后继续", Confidence: 1, Revision: 7, CreatedAt: now, UpdatedAt: now, LastActiveAt: now}
	if err := p.services.DB.Create(&thread).Error; err != nil {
		t.Fatal(err)
	}
	binding := continuity.ThreadBinding{ID: "legacy-binding", ThreadID: thread.ID, BindingType: "conversation", BindingID: "chat-1", Role: "context", Confidence: 1, Source: "legacy", CreatedAt: now, LastActiveAt: now}
	wait := continuity.Wait{ID: "legacy-wait", ThreadID: thread.ID, WaitType: continuity.WaitTypeDependency, Status: continuity.WaitStatusWaiting, Description: "等待任务", ConditionJSON: `{"taskRunId":"task-1"}`, AutoResume: true, CreatedAt: now, UpdatedAt: now}
	event := continuity.ThreadEvent{ID: "legacy-event", ThreadID: thread.ID, EventType: "thread.created", PayloadJSON: `{"summary":"旧事件"}`, IdempotencyKey: "legacy-event", OccurredAt: now}
	for _, model := range []interface{}{&binding, &wait, &event} {
		if err := p.services.DB.Create(model).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateLegacyContinuity(t.Context(), p.services.DB, engine, "core", "device-a", "core"); err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyContinuity(t.Context(), p.services.DB, engine, "core", "device-a", "core"); err != nil {
		t.Fatal(err)
	}
	request := business.Request{SpaceID: "core", DeviceID: "device-a", CoreID: "core", RoleID: "one", RequestID: "read-migrated"}
	documents, _, err := engine.ListContinuity(t.Context(), request, true)
	if err != nil || len(documents) != 1 {
		t.Fatalf("documents=%+v err=%v", documents, err)
	}
	document := documents[0]
	if document.Thread.ID != thread.ID || document.Thread.Revision != thread.Revision || len(document.Bindings) != 1 || len(document.Waits) != 1 || len(document.Events) != 1 {
		t.Fatalf("migrated=%+v", document)
	}
}

func TestOwnedContinuityRuntimeCreatesBindsAndObservesChat(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device-a','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	engine := business.NewEngine(coordination.NewService(db), p, continuityTestModel{})
	runtime := newOwnedContinuityRuntime(engine, nil, nil, "core", "device-a", "core")
	owner := interaction.EntryOwner{SpaceID: "core", DeviceID: "device-a", TargetDeviceID: "device-a", CoreID: "core"}
	threadID, err := runtime.Resolve(t.Context(), owner, "one", "chat-1", "workspace-1", "帮我部署生产环境并持续跟进直到完成", "request-1")
	if err != nil || threadID == "" {
		t.Fatalf("threadId=%q err=%v", threadID, err)
	}
	reused, err := runtime.Resolve(t.Context(), owner, "one", "chat-1", "workspace-1", "继续", "request-2")
	if err != nil || reused != threadID {
		t.Fatalf("reused=%q want=%q err=%v", reused, threadID, err)
	}
	if err := runtime.Observe(t.Context(), owner, "one", threadID, continuity.ObservePayload{Version: "v1", SpaceID: "core", CharacterID: "one", ConversationID: "chat-1", RequestID: "request-1", ExecutionID: "execution-1", UserMessage: "帮我部署生产环境并持续跟进直到完成", AssistantReply: "部署正在进行。请确认是否继续迁移。", Source: "chat"}); err != nil {
		t.Fatal(err)
	}
	documents, _, err := engine.ListContinuity(t.Context(), business.Request{SpaceID: "core", DeviceID: "device-a", CoreID: "core", RoleID: "one", RequestID: "read"}, true)
	if err != nil || len(documents) != 1 {
		t.Fatalf("documents=%+v err=%v", documents, err)
	}
	if documents[0].Thread.ID != threadID || len(documents[0].Bindings) < 2 || documents[0].Thread.CurrentState == "" || len(documents[0].Waits) != 1 {
		t.Fatalf("document=%+v", documents[0])
	}
	approval := continuity.ApprovalLifecycleSignal{ApprovalID: "approval-1", SpaceID: "core", ConversationID: "chat-1", RequestID: "request-1", ToolCallID: "tool-1", Timestamp: time.Now().UTC()}
	if err := runtime.ApplyApprovalRequested(t.Context(), approval); err != nil {
		t.Fatal(err)
	}
	documents, _, err = engine.ListContinuity(t.Context(), business.Request{SpaceID: "core", DeviceID: "device-a", CoreID: "core", RoleID: "one", RequestID: "read-approval"}, true)
	if err != nil || len(documents) != 1 {
		t.Fatalf("approval documents=%+v err=%v", documents, err)
	}
	approvalWait := false
	for _, wait := range documents[0].Waits {
		if wait.WaitType == continuity.WaitTypeApproval && wait.Status == continuity.WaitStatusWaiting {
			approvalWait = true
		}
	}
	if !approvalWait {
		t.Fatalf("approval wait missing: %+v", documents[0].Waits)
	}
	approval.Resolved = true
	approval.Approved = true
	if err := runtime.ApplyApprovalResolved(t.Context(), approval); err != nil {
		t.Fatal(err)
	}
	documents, _, err = engine.ListContinuity(t.Context(), business.Request{SpaceID: "core", DeviceID: "device-a", CoreID: "core", RoleID: "one", RequestID: "read-approval-resolved"}, true)
	if err != nil || len(documents) != 1 {
		t.Fatalf("resolved documents=%+v err=%v", documents, err)
	}
	for _, wait := range documents[0].Waits {
		if wait.WaitType == continuity.WaitTypeApproval && wait.Status != continuity.WaitStatusResolved {
			t.Fatalf("approval wait not resolved: %+v", wait)
		}
	}
}
