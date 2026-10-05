package business

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type continuitySignalPort struct{ testDataPort }

func (p continuitySignalPort) Snapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.DataSnapshot, error) {
	if query.ResourceKind != "continuity" {
		return p.testDataPort.Snapshot(ctx, scope, query)
	}
	rows, next, err := coordination.NewOwnershipStore(p.db, scope.ResourceOwnerID).ListPage(ctx, "continuity", scope.RoleID, query)
	return coordination.DataSnapshot{OwnerID: scope.ResourceOwnerID, Role: p.role, Resources: rows, NextCursors: map[string]string{"continuity": next}}, err
}

func TestContinuitySignalMatchesExactConditionsAndNeverReplaysWake(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	engine.data = continuitySignalPort{engine.data.(testDataPort)}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "create"}
	document, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "等待任务完成"})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "wait"
	document, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: 1, Action: "add_wait", Wait: &continuity.Wait{WaitType: continuity.WaitTypeDependency, ConditionJSON: `{"taskRunId":"task-1"}`, AutoResume: true}})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "event-1"
	signal := continuity.Signal{WaitType: continuity.WaitTypeDependency, OccurredAt: time.Now().UTC(), Attributes: map[string]any{"taskRunId": "other-task"}}
	if resolved, err := engine.SignalContinuity(t.Context(), request, signal); err != nil || len(resolved) != 0 {
		t.Fatalf("unrelated signal accepted: %v %v", resolved, err)
	}
	signal.Attributes["taskRunId"] = "task-1"
	resolved, err := engine.SignalContinuity(t.Context(), request, signal)
	if err != nil || len(resolved) != 1 || resolved[0] != document.Waits[0].ID || model.calls.Load() != 0 {
		t.Fatalf("signal=%v %v", resolved, err)
	}
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	if model.calls.Load() != 1 {
		t.Fatal("resolved dependency did not continue through Core")
	}
	if _, err := engine.SignalContinuity(t.Context(), request, signal); err != nil {
		t.Fatal(err)
	}
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	if model.calls.Load() != 1 {
		t.Fatal("duplicate event replayed completed action")
	}
}
