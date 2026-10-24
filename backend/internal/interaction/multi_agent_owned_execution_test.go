package interaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedAuthorityWorker struct {
	base      AgentWorkerRunner
	expected  coordination.ExecutionScope
	started   int
	cancelled []string
}

type cancelRetryTracker struct {
	InteractionTracker
	failed bool
}

func (t *cancelRetryTracker) RequestCancel(ctx context.Context, id string, reason string) error {
	if err := t.InteractionTracker.RequestCancel(ctx, id, reason); err != nil {
		return err
	}
	record, found, err := t.InteractionTracker.Get(ctx, id)
	if err != nil {
		return err
	}
	if found && !record.IsTerminal() {
		record.CancelRequestedAt = time.Now()
	}
	return nil
}

func (t *cancelRetryTracker) UpdateMetadata(ctx context.Context, id string, update InteractionMetadataUpdate) (*InteractionRecord, error) {
	if !t.failed && update.RecoveryDescriptor != nil && update.RecoveryDescriptor.MultiAgent != nil && update.RecoveryDescriptor.MultiAgent.Status == "cancelled" {
		t.failed = true
		return nil, errors.New("controlled transient persistence failure")
	}
	return t.InteractionTracker.UpdateMetadata(ctx, id, update)
}

type cancelRetryWorker struct{ attempts int }

func (w *cancelRetryWorker) StartWorker(context.Context, WorkerRunRequest) (string, error) {
	return "", errors.New("unexpected dispatch")
}
func (w *cancelRetryWorker) CancelWorker(context.Context, string) error {
	w.attempts++
	if w.attempts == 1 {
		return errors.New("controlled transient stop failure")
	}
	return nil
}

func TestMultiAgentCancellationRetriesStopAndPersistence(t *testing.T) {
	coord, tracker, _, id := newReconciliationCoordinator(t, CoordinationSequential)
	retrying := &cancelRetryTracker{InteractionTracker: tracker}
	worker := &cancelRetryWorker{}
	coord.tracker, coord.starter = retrying, worker
	if err := coord.Cancel(t.Context(), id); err == nil {
		t.Fatal("lost initial stop failure")
	}
	if tracker.records["parent-reconcile"].CancelRequestedAt.IsZero() {
		t.Fatal("cancellation intent was not durable before stopping")
	}
	if err := coord.Cancel(t.Context(), id); err != nil {
		t.Fatalf("retrying cancellation without a receipt failed: %v", err)
	}
	ac := coord.coordinations[id]
	if ac.cancelConfirmed || ac.status != CoordinationWaiting || worker.attempts != 2 {
		t.Fatal("an unconfirmed cancellation was finalized")
	}
	childID := ac.assignments[0].ChildInteractionID
	tracker.records[childID] = &InteractionRecord{ID: childID, Status: InteractionStatusCancelled}
	if err := coord.Reconcile(t.Context(), id); err == nil {
		t.Fatal("lost controlled terminal checkpoint failure")
	}
	if err := coord.Reconcile(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if !ac.cancelConfirmed || ac.status != CoordinationCancelled || tracker.records["parent-reconcile"].RecoveryDescriptor.MultiAgent.Status != "cancelled" {
		t.Fatal("cancellation did not converge after confirmed child terminal result")
	}
	attempts := worker.attempts
	if err := coord.Cancel(t.Context(), id); err != nil || worker.attempts != attempts {
		t.Fatalf("confirmed cancellation was not idempotent: %v", err)
	}
}

func (w *ownedAuthorityWorker) StartWorker(ctx context.Context, req WorkerRunRequest) (string, error) {
	actual, ok := coordination.FromContext(ctx)
	if !ok || actual != w.expected || req.OwnedScope == nil || *req.OwnedScope != w.expected {
		return "", coordination.ErrScopeExpired
	}
	w.started++
	return w.base.StartWorker(ctx, req)
}

func (w *ownedAuthorityWorker) CancelWorker(_ context.Context, id string) error {
	w.cancelled = append(w.cancelled, id)
	return nil
}

func TestMultiAgentOwnedReconciliationRestoresAuthorityOnce(t *testing.T) {
	coord, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := coord.coordinations[id]
	expected := coordination.ExecutionScope{SpaceID: "space-a", CoreID: "core-a", RoleID: "role-a", InitiatorDeviceID: "device-a", TargetDeviceID: "device-a", ResourceOwnerID: "device-a"}
	ac.ownedScope = &expected
	worker := &ownedAuthorityWorker{base: workers, expected: expected}
	coord.starter = worker
	authorizations := 0
	coord.SetOwnedExecution(func(ctx context.Context, scope coordination.ExecutionScope, run func(context.Context) error) error {
		authorizations++
		if scope != expected {
			t.Fatal("restored wrong authority")
		}
		return run(coordination.WithScope(ctx, scope))
	})
	first := ac.assignments[0].ChildInteractionID
	tracker.records[first] = &InteractionRecord{ID: first, Status: InteractionStatusCompleted, ResultRef: "first complete"}
	if err := coord.Reconcile(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if worker.started != 1 || authorizations != 1 || len(workers.started) != 2 {
		t.Fatalf("worker dispatch did not restore and reuse authority: %d %d %d", worker.started, authorizations, len(workers.started))
	}
}

func TestMultiAgentOwnedAuthorizationFailsClosed(t *testing.T) {
	coord, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := coord.coordinations[id]
	ac.ownedScope = &coordination.ExecutionScope{SpaceID: "space-a"}
	tracker.records[ac.assignments[0].ChildInteractionID] = &InteractionRecord{ID: ac.assignments[0].ChildInteractionID, Status: InteractionStatusCompleted}
	if err := coord.Reconcile(context.Background(), id); err == nil {
		t.Fatal("missing authority factory dispatched worker")
	}
	coord.SetOwnedExecution(func(context.Context, coordination.ExecutionScope, func(context.Context) error) error {
		return coordination.ErrScopeExpired
	})
	if err := coord.Reconcile(context.Background(), id); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("revoked authority accepted: %v", err)
	}
	if len(workers.started) != 1 {
		t.Fatal("unauthorized pending worker dispatched")
	}
	if ac.status != CoordinationWaiting || !ac.cancelRequested || ac.assignments[1].Status != AssignmentCancelled || ac.assignments[1].Error == "" {
		t.Fatal("revoked coordination should be waiting for external worker acknowledgement")
	}
	parent := tracker.records["parent-reconcile"]
	if parent.RecoveryDescriptor == nil || !parent.RecoveryDescriptor.MultiAgent.CancelRequested {
		t.Fatal("revocation was not durably requested")
	}
}

func TestMultiAgentParentCancellationStopsWorkersAndDispatch(t *testing.T) {
	coord, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	worker := &ownedAuthorityWorker{base: workers}
	coord.starter = worker
	parent := tracker.records["parent-reconcile"]
	parent.CancelRequestedAt = time.Now()
	if err := coord.Reconcile(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	ac := coord.coordinations[id]
	if ac.status != CoordinationWaiting || !ac.cancelRequested || ac.assignments[1].Status != AssignmentCancelled || len(worker.cancelled) != 1 || len(workers.started) != 1 {
		t.Fatalf("parent cancellation invented worker acknowledgement: %#v cancelled=%v starts=%d", ac.assignments, worker.cancelled, len(workers.started))
	}
	childID := ac.assignments[0].ChildInteractionID
	tracker.records[childID] = &InteractionRecord{ID: childID, Status: InteractionStatusCancelled}
	if err := coord.Reconcile(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if ac.status != CoordinationCancelled || len(workers.started) != 1 {
		t.Fatal("confirmed cancelled worker did not settle parent")
	}
	previous := len(worker.cancelled)
	if err := coord.Reconcile(context.Background(), id); err != nil || len(worker.cancelled) != previous {
		t.Fatal("settled cancellation sent duplicate worker stop")
	}
}

func TestMultiAgentCancelledParentCannotDispatchDirectly(t *testing.T) {
	coord, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := coord.coordinations[id]
	tracker.records["parent-reconcile"].Status = InteractionStatusCancelled
	if err := coord.startOneAssignment(context.Background(), ac, ac.assignments[1]); err == nil {
		t.Fatal("cancelled parent dispatched next assignment")
	}
	if len(workers.started) != 1 {
		t.Fatal("unexpected second worker")
	}
}
