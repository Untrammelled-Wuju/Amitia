package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/attention"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/observability"
	ksqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCollectAttentionItemsIncludesFailedAutomations(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "workflow.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := ksqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	repo := ksqlite.NewWorkflowExecutionRepository(db)
	now := time.Now().UTC()
	run := workflow.WorkflowRun{
		ExecutionID: "workflow-run-failed",
		WorkflowID:  "workflow-one",
		Status:      workflow.RunStatusRunning,
		Context: workflow.ExecutionContext{
			SpaceID: "space-a", ConversationID: "conversation-a", CharacterID: "role-a",
			InvocationID: "invocation-a", DeviceID: "device-a",
		},
		StartedAt: now,
		UpdatedAt: now,
	}
	if _, created, err := repo.Start(context.Background(), run); err != nil || !created {
		t.Fatalf("start: created=%v err=%v", created, err)
	}
	run.Status = workflow.RunStatusFailed
	run.Error = "step exploded"
	run.FinishedAt = &now
	run.UpdatedAt = now
	if err := repo.Finish(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	services := &AppServices{KernelContainer: &kernel.Container{WorkflowExecRepo: repo}}
	items, err := collectAttentionItems(context.Background(), services, "space-a", "device-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%+v", items)
	}
	item := items[0]
	if item.Kind != "automation_failed" || !item.RequiresAction || item.WorkflowRunID != run.ExecutionID || item.WorkflowID != run.WorkflowID || item.ConversationID != "conversation-a" || !strings.Contains(item.Route, "runId=workflow-run-failed") {
		t.Fatalf("unexpected automation item: %+v", item)
	}
}

func attentionNotificationTestService(t *testing.T, shadow bool) (*attention.Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "attention.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&attention.BudgetPolicy{}, &attention.Event{}); err != nil {
		t.Fatal(err)
	}
	service := attention.NewService(db)
	policy := attention.DefaultPolicy("space")
	policy.ShadowMode = shadow
	policy.MaxUnchangedNotifications = 1
	if _, err := service.PutPolicy(context.Background(), policy); err != nil {
		t.Fatal(err)
	}
	return service, db
}

func TestAttentionNotificationRecordsUnchangedInShadowMode(t *testing.T) {
	service, db := attentionNotificationTestService(t, true)
	services := &AppServices{Attention: service}
	item := attention.Item{
		ID: "agent-approval:one", Kind: "agent_approval", Decision: "pending", RequiresAction: true,
		Title: "工具执行需要批准", SpaceID: "space", ConversationID: "conversation", ApprovalID: "one",
	}
	now := time.Now().UTC()
	allowed, err := attentionNotificationAllowed(context.Background(), services, item, now)
	if err != nil || !allowed {
		t.Fatalf("initial allowed=%v err=%v", allowed, err)
	}
	allowed, err = attentionNotificationAllowed(context.Background(), services, item, now.Add(time.Second))
	if err != nil || !allowed {
		t.Fatalf("unchanged shadow allowed=%v err=%v", allowed, err)
	}
	var event attention.Event
	if err := db.Where("space_id = ? AND event_kind = ?", "space", "unchanged").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.SourceID != item.ID || event.IdempotencyKey == "" {
		t.Fatalf("unexpected unchanged event: %+v", event)
	}
}

func TestAttentionNotificationSuppressesRepeatedUnchangedOutsideShadowMode(t *testing.T) {
	service, _ := attentionNotificationTestService(t, false)
	services := &AppServices{Attention: service}
	item := attention.Item{
		ID: "task-approval:one", Kind: "task_approval", Decision: "pending", RequiresAction: true,
		Title: "设备任务需要批准", SpaceID: "space", ConversationID: "conversation", ApprovalID: "one",
	}
	now := time.Now().UTC()
	for index := 0; index < 2; index++ {
		allowed, err := attentionNotificationAllowed(context.Background(), services, item, now.Add(time.Duration(index)*time.Second))
		if err != nil || !allowed {
			t.Fatalf("call=%d allowed=%v err=%v", index, allowed, err)
		}
	}
	allowed, err := attentionNotificationAllowed(context.Background(), services, item, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("repeated unchanged notification should be suppressed")
	}
}

func TestCollectAttentionTimelineProjectsHumanApprover(t *testing.T) {
	store := observability.NewMemoryStore()
	if err := store.SaveAuditEvent(context.Background(), observability.AuditEvent{
		AuditID: "audit-approval-one", ActorType: observability.ActorUser, ActorID: "device-a",
		SubjectType: observability.SubjectTool, SubjectID: "terminal.write", Action: "approval.decide",
		Decision: "approved", Result: "resolved", ApprovalID: "approval-one", CreatedAt: time.Now().UTC(),
		Metadata: map[string]any{"requestId": "request-one", "principalType": "local_ui", "authMethod": "desktop_session", "sessionId": "session-a"},
	}); err != nil {
		t.Fatal(err)
	}
	services := &AppServices{KernelContainer: &kernel.Container{ObservabilityStore: store}}
	events, err := collectAttentionTimeline(context.Background(), services, "space", "", "", "", "approval-one", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Type == "approval.decision" {
			if event.Details["authorizedBy"] != "device-a" || event.Details["authorizedByType"] != "user" || event.Details["authMethod"] != "desktop_session" {
				t.Fatalf("unexpected approval decision timeline: %+v", event)
			}
			return
		}
	}
	t.Fatalf("approval decision missing from timeline: %+v", events)
}
