package main

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"gorm.io/gorm"
)

type ownedWorkerEngineFunc func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error)

func (f ownedWorkerEngineFunc) RunWorker(ctx context.Context, scope coordination.ExecutionScope, request business.Request, workspaces []business.ProjectWorkspace, primary string) (business.Response, error) {
	return f(ctx, scope, request, workspaces, primary)
}

func ownedWorkerFixture() interaction.WorkerRunRequest {
	return interaction.WorkerRunRequest{AssignmentID: "assignment-a", CoordinationID: "coordination-a", ParentInteractionID: "parent-a", SpaceID: "space-a", CharacterID: "role-a", ConversationID: "parent-conversation", Source: "multi_agent", Objective: "检查工作树", ExpectedOutcome: "返回验证结果", WorkspaceID: "worker-mount", WorkspaceDeviceID: "device-a", WorkspaceKind: "local", WorkspaceRootURI: "amitia://workspace/@worker-mount", OwnedScope: &coordination.ExecutionScope{SpaceID: "space-a", InitiatorDeviceID: "device-a", TargetDeviceID: "device-a", CoreID: "core-a", RoleID: "role-a", ResourceOwnerID: "device-a", RoleOwnerID: "device-a"}}
}

func waitOwnedWorkerRecord(t *testing.T, tracker interaction.InteractionTracker, id string, status interaction.InteractionStatus) *interaction.InteractionRecord {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		record, found, err := tracker.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if found && record.Status == status {
			return record
		}
		time.Sleep(time.Millisecond)
	}
	record, _, _ := tracker.Get(context.Background(), id)
	t.Fatalf("worker did not reach %s: %#v", status, record)
	return nil
}

func TestOwnedWorkerDetachedExecutionAndIdempotency(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	engine := ownedWorkerEngineFunc(func(ctx context.Context, scope coordination.ExecutionScope, request business.Request, workspaces []business.ProjectWorkspace, primary string) (business.Response, error) {
		calls.Add(1)
		actual, ok := interaction.FromContext(ctx)
		if !ok || actual.Source != "multi_agent" || actual.ConversationID != "multi-agent-worker-assignment-a" || actual.RequestID != "ma-assignment-a" {
			t.Error("missing worker interaction context")
		}
		if request.RoleID != scope.RoleID || request.CoreID != scope.CoreID || request.ConversationOrigin != nil || request.ProjectID != "" || request.RequestID != actual.RequestID {
			t.Error("worker identity was not rebuilt from trusted scope")
		}
		if len(workspaces) != 1 || workspaces[0].ID != "worker-mount" || primary != "worker-mount" {
			t.Error("isolated workspace was not forwarded")
		}
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return business.Response{}, ctx.Err()
		}
		return business.Response{Saved: true, Generation: business.Generation{Text: "验证完成"}}, nil
	})
	runner := newOwnedMultiAgentWorkerRunner(engine, tracker, nil)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	requestCtx, cancel := context.WithCancel(context.Background())
	req := ownedWorkerFixture()
	id, err := runner.StartWorker(requestCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	cancel()
	if !runner.WorkerIsActive(id) {
		t.Fatal("worker ended when initiating request disconnected")
	}
	if repeated, err := runner.StartWorker(context.Background(), req); err != nil || repeated != id {
		t.Fatalf("retry: %s %v", repeated, err)
	}
	changed := req
	changed.Objective = "different action"
	if _, err := runner.StartWorker(context.Background(), changed); err == nil {
		t.Fatal("active request input collision accepted")
	}
	close(release)
	record := waitOwnedWorkerRecord(t, tracker, id, interaction.InteractionStatusCompleted)
	if record.ResultRef != "验证完成" || calls.Load() != 1 {
		t.Fatalf("result: %#v calls %d", record, calls.Load())
	}
	if _, err := runner.StartWorker(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("completed worker was dispatched twice")
	}
}

func TestOwnedWorkerCancelTerminatesExecutionAndLedger(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	started, stopped := make(chan struct{}), make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return business.Response{}, ctx.Err()
	}), tracker, nil)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	id, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err := runner.CancelWorker(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("worker context not cancelled")
	}
	waitOwnedWorkerRecord(t, tracker, id, interaction.InteractionStatusCancelled)
}

func TestOwnedWorkerFailureNeverReportsSuccess(t *testing.T) {
	for _, scenario := range []string{"permission", "partial", "unsaved", "panic"} {
		t.Run(scenario, func(t *testing.T) {
			tracker := interaction.NewInMemoryTracker()
			runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error) {
				switch scenario {
				case "permission":
					return business.Response{}, coordination.ErrScopeExpired
				case "partial":
					return business.Response{Saved: true, Generation: business.Generation{Partial: true}}, nil
				case "panic":
					panic("worker panic")
				default:
					return business.Response{}, nil
				}
			}), tracker, nil)
			t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
			id, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
			if err != nil {
				t.Fatal(err)
			}
			record := waitOwnedWorkerRecord(t, tracker, id, interaction.InteractionStatusFailed)
			if record.ErrorCode == "" {
				t.Fatal("worker failure reason missing")
			}
		})
	}
}

type ownedWorkerFallback struct {
	calls     int
	stopped   bool
	cancelled bool
}

func (f *ownedWorkerFallback) StartWorker(context.Context, interaction.WorkerRunRequest) (string, error) {
	f.calls++
	return "legacy-child", nil
}
func (f *ownedWorkerFallback) WorkerInteractionID(interaction.WorkerRunRequest) string {
	return "legacy-child"
}
func (f *ownedWorkerFallback) WorkerIsActive(id string) bool  { return id == "legacy-child" }
func (f *ownedWorkerFallback) Shutdown(context.Context) error { f.stopped = true; return nil }
func (f *ownedWorkerFallback) CancelWorker(context.Context, string) error {
	f.cancelled = true
	return nil
}

func TestOwnedWorkerLegacyDelegationAndScopeValidation(t *testing.T) {
	fallback := &ownedWorkerFallback{}
	runner := newOwnedMultiAgentWorkerRunner(nil, interaction.NewInMemoryTracker(), fallback)
	req := ownedWorkerFixture()
	if _, err := runner.StartWorker(context.Background(), req); err == nil {
		t.Fatal("missing owned engine accepted")
	}
	if fallback.calls != 0 {
		t.Fatal("owned request fell back to legacy")
	}
	req.OwnedScope = nil
	if id, err := runner.StartWorker(context.Background(), req); err != nil || id != "legacy-child" {
		t.Fatalf("fallback %s %v", id, err)
	}
	if runner.WorkerInteractionID(req) != "legacy-child" || !runner.WorkerIsActive("legacy-child") {
		t.Fatal("fallback lifecycle not forwarded")
	}
	if err := runner.CancelWorker(context.Background(), "legacy-child"); err != nil {
		t.Fatal(err)
	}
	if err := runner.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !fallback.stopped || !fallback.cancelled {
		t.Fatal("fallback lifecycle not completed")
	}
}

func TestOwnedWorkerShutdownCancelsAndRejectsNewRequests(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	started := make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		close(started)
		<-ctx.Done()
		return business.Response{}, ctx.Err()
	}), tracker, nil)
	id, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err := runner.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitOwnedWorkerRecord(t, tracker, id, interaction.InteractionStatusCancelled)
	if runner.WorkerIsActive(id) {
		t.Fatal("closed worker is active")
	}
	if _, err := runner.StartWorker(context.Background(), ownedWorkerFixture()); err == nil {
		t.Fatal("closed runner accepted worker")
	}
	if _, err := (&ownedWorkerEnginePort{}).RunWorker(context.Background(), coordination.ExecutionScope{}, business.Request{}, nil, ""); err == nil || errors.Is(err, context.Canceled) {
		t.Fatal("uninitialized engine accepted")
	}
}

func TestOwnedWorkerSQLiteReservationAndRestartReplay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "worker.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	tracker := interaction.NewSQLiteInteractionTracker(db)
	if err := tracker.InitSchema(); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	engine := ownedWorkerEngineFunc(func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error) {
		calls.Add(1)
		return business.Response{Saved: true, Generation: business.Generation{Text: "持久化完成"}}, nil
	})
	runner := newOwnedMultiAgentWorkerRunner(engine, tracker, nil)
	req := ownedWorkerFixture()
	id, err := runner.StartWorker(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	record := waitOwnedWorkerRecord(t, tracker, id, interaction.InteractionStatusCompleted)
	if record.RecoveryDescriptor == nil || record.RecoveryDescriptor.OwnedTurn == nil || record.RecoveryDescriptor.OwnedTurn.InputFingerprint == "" {
		t.Fatal("worker reservation fingerprint not persisted")
	}
	if err := runner.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	restarted := newOwnedMultiAgentWorkerRunner(engine, interaction.NewSQLiteInteractionTracker(db), nil)
	t.Cleanup(func() { _ = restarted.Shutdown(context.Background()) })
	if repeated, err := restarted.StartWorker(context.Background(), req); err != nil || repeated != id {
		t.Fatalf("restart replay: %s %v", repeated, err)
	}
	if calls.Load() != 1 {
		t.Fatal("restart dispatched completed worker again")
	}
	req.Objective = "changed after restart"
	if _, err := restarted.StartWorker(context.Background(), req); err == nil {
		t.Fatal("persisted input conflict not rejected")
	}
}
