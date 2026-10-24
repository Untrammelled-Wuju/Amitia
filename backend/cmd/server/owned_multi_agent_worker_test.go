package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	return interaction.WorkerRunRequest{AssignmentID: "assignment-a", CoordinationID: "coordination-a", ParentInteractionID: "parent-a", ParentGoalID: "goal-a", ParentGoalRevision: 1, SpaceID: "space-a", CharacterID: "role-a", ConversationID: "parent-conversation", Source: "multi_agent", Objective: "检查工作树", ExpectedOutcome: "返回验证结果", WorkspaceID: "worker-mount", WorkspaceDeviceID: "device-a", WorkspaceKind: "local", WorkspaceRootURI: "amitia://workspace/@worker-mount", OwnedScope: &coordination.ExecutionScope{SpaceID: "space-a", InitiatorDeviceID: "device-a", TargetDeviceID: "device-a", CoreID: "core-a", RoleID: "role-a", ResourceOwnerID: "device-a", RoleOwnerID: "device-a"}}
}

func registerOwnedWorkerParent(t *testing.T, tracker interaction.InteractionTracker) {
	t.Helper()
	parent := interaction.NewInteractionRecord(interaction.InteractionScope{
		SpaceID: "space-a", CharacterID: "role-a", ConversationID: "parent-conversation", Channel: "web", Source: "web", RequestID: "owned-worker-parent-test",
	})
	parent.ID = "parent-a"
	req := ownedWorkerFixture()
	parent.RecoveryDescriptor = &interaction.RecoveryDescriptor{SchemaVersion: interaction.RecoveryDescriptorSchemaVersion, MultiAgent: &interaction.MultiAgentRecoveryRef{
		CoordinationID: req.CoordinationID, ParentGoalID: req.ParentGoalID, ParentGoalRevision: req.ParentGoalRevision,
		OwnedScope: req.OwnedScope, Status: string(interaction.CoordinationRunning), WorkspaceID: req.WorkspaceID, WorkspaceDeviceID: req.WorkspaceDeviceID, WorkspaceRootURI: req.WorkspaceRootURI, PermissionMode: req.PermissionMode,
		ParentGoalSpaceID: req.SpaceID, ParentGoalCharacterID: req.CharacterID, ParentGoalConversationID: req.ConversationID,
		AssignmentRefs: []interaction.AssignmentRecoveryRef{{AssignmentID: req.AssignmentID, CharacterID: req.CharacterID, Role: req.WorkerRole, Objective: req.Objective, ExpectedOutcome: req.ExpectedOutcome, ChildInteractionID: newOwnedMultiAgentWorkerRunner(nil, tracker).WorkerInteractionID(req), Status: string(interaction.AssignmentRunning)}},
	}}
	if err := tracker.Create(context.Background(), parent); err != nil {
		t.Fatal(err)
	}
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
	registerOwnedWorkerParent(t, tracker)
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
	runner := newOwnedMultiAgentWorkerRunner(engine, tracker)
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
	registerOwnedWorkerParent(t, tracker)
	started, stopped := make(chan struct{}), make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return business.Response{}, ctx.Err()
	}), tracker)
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
	waitOwnedWorkerRecord(t, tracker, id, interaction.InteractionStatusInterrupted)
}

func TestOwnedWorkerFailureNeverReportsSuccess(t *testing.T) {
	for _, scenario := range []string{"permission", "partial", "unsaved", "panic"} {
		t.Run(scenario, func(t *testing.T) {
			tracker := interaction.NewInMemoryTracker()
			registerOwnedWorkerParent(t, tracker)
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
			}), tracker)
			t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
			id, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
			if err != nil {
				t.Fatal(err)
			}
			want := interaction.InteractionStatusFailed
			if scenario == "panic" {
				want = interaction.InteractionStatusInterrupted
			}
			record := waitOwnedWorkerRecord(t, tracker, id, want)
			if want == interaction.InteractionStatusFailed && record.ErrorCode == "" {
				t.Fatal("worker failure reason missing")
			}
		})
	}
}

func TestOwnedWorkerRequiresAuthoritativeScope(t *testing.T) {
	runner := newOwnedMultiAgentWorkerRunner(nil, interaction.NewInMemoryTracker())
	req := ownedWorkerFixture()
	if _, err := runner.StartWorker(t.Context(), req); err == nil {
		t.Fatal("missing owned engine accepted")
	}
	req.OwnedScope = nil
	if _, err := runner.StartWorker(t.Context(), req); err == nil {
		t.Fatal("unowned worker executed")
	}
	if runner.WorkerIsActive("legacy-child") {
		t.Fatal("unowned worker appeared active")
	}
	if err := runner.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestOwnedWorkerShutdownCancelsAndRejectsNewRequests(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	started := make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		close(started)
		<-ctx.Done()
		return business.Response{}, ctx.Err()
	}), tracker)
	id, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err := runner.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitOwnedWorkerRecord(t, tracker, id, interaction.InteractionStatusInterrupted)
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
	registerOwnedWorkerParent(t, tracker)
	var calls atomic.Int32
	engine := ownedWorkerEngineFunc(func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error) {
		calls.Add(1)
		return business.Response{Saved: true, Generation: business.Generation{Text: "持久化完成"}}, nil
	})
	runner := newOwnedMultiAgentWorkerRunner(engine, tracker)
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
	restarted := newOwnedMultiAgentWorkerRunner(engine, interaction.NewSQLiteInteractionTracker(db))
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

func TestOwnedWorkerRestartDoesNotReportPhantomResume(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "orphan.db")), &gorm.Config{})
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
	registerOwnedWorkerParent(t, tracker)
	req := ownedWorkerFixture()
	var calls atomic.Int32
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error) {
		calls.Add(1)
		return business.Response{Saved: true}, nil
	}), tracker)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	childID := runner.WorkerInteractionID(req)
	workerScope := interaction.InteractionScope{SpaceID: req.SpaceID, CharacterID: req.CharacterID, ConversationID: "multi-agent-worker-" + req.AssignmentID, Channel: "web", Source: "multi_agent", RequestID: "ma-" + req.AssignmentID}
	record := interaction.NewInteractionRecord(workerScope)
	record.ID = childID
	record.ExecutorID = req.OwnedScope.CoreID
	record.CausationID = req.ParentInteractionID
	record.CorrelationID = req.CoordinationID
	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := sha256.Sum256(encoded)
	record.RecoveryDescriptor = &interaction.RecoveryDescriptor{
		SchemaVersion: interaction.RecoveryDescriptorSchemaVersion,
		Requirement:   interaction.RecoveryRequired,
		State:         interaction.RecoveryDescriptorActive,
		Interaction:   interaction.RecoveryInteractionRef{InteractionID: childID, RequestID: workerScope.RequestID, Status: record.Status},
		Scope:         interaction.RecoveryScopeRef{SpaceID: req.SpaceID, CharacterID: req.CharacterID, ConversationID: workerScope.ConversationID, Channel: workerScope.Channel},
		OwnedTurn:     &interaction.OwnedTurnRecoveryRef{Scope: *req.OwnedScope, RequestID: workerScope.RequestID, InputFingerprint: hex.EncodeToString(fingerprint[:])},
	}
	persisted, _, err := interaction.RegisterInteraction(context.Background(), tracker, record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tracker.TransitionCAS(context.Background(), childID, persisted.StatusVersion, interaction.InteractionStatusProcessing); err != nil {
		t.Fatal(err)
	}
	newProcess := newOwnedMultiAgentWorkerRunner(runner.engine, interaction.NewSQLiteInteractionTracker(db))
	t.Cleanup(func() { _ = newProcess.Shutdown(context.Background()) })
	resumed, err := newProcess.StartWorker(context.Background(), req)
	if err == nil || resumed != "" || calls.Load() != 0 {
		t.Fatalf("orphan worker was falsely reported as resumed: %q %v calls=%d", resumed, err, calls.Load())
	}
	current, found, err := tracker.Get(context.Background(), childID)
	if err != nil || !found || current.Status != interaction.InteractionStatusProcessing {
		t.Fatalf("unconfirmed worker state was overwritten: %+v %v", current, err)
	}
}

func TestRound39WorkerCancelDoesNotFabricateStopBeforeEngineReturns(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	entered := make(chan struct{})
	release := make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		close(entered)
		<-ctx.Done()
		<-release
		return business.Response{}, ctx.Err()
	}), tracker)
	var released atomic.Bool
	t.Cleanup(func() {
		if released.CompareAndSwap(false, true) {
			close(release)
		}
		_ = runner.Shutdown(context.Background())
	})
	childID, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := runner.CancelWorker(context.Background(), childID); err != nil {
		t.Fatal(err)
	}
	record, found, err := tracker.Get(context.Background(), childID)
	if err != nil || !found || record.IsTerminal() || record.CancelRequestedAt.IsZero() {
		t.Fatalf("worker was marked terminal while its external engine was still running: %+v err=%v", record, err)
	}
	if released.CompareAndSwap(false, true) {
		close(release)
	}
	waitOwnedWorkerRecord(t, tracker, childID, interaction.InteractionStatusInterrupted)
}

func ownedWorkerBudgetDatabase(t *testing.T) (*interaction.SQLiteInteractionTracker, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "worker-budget.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	tracker := interaction.NewSQLiteInteractionTracker(db)
	if err := tracker.InitSchema(); err != nil {
		t.Fatal(err)
	}
	return tracker, db
}
