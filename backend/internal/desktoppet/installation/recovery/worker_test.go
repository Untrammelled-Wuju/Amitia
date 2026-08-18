// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
)

type testRecoveryRepo struct {
	ops            map[string]*operation.InstallationOperation
	commitJournals map[string]*RecoveryCommitJournal
	switchJournals map[string]*RecoverySwitchJournal
}

func newTestRepo() *testRecoveryRepo {
	return &testRecoveryRepo{
		ops:            make(map[string]*operation.InstallationOperation),
		commitJournals: make(map[string]*RecoveryCommitJournal),
		switchJournals: make(map[string]*RecoverySwitchJournal),
	}
}

func (r *testRecoveryRepo) ListExpiredLeaseOperations(leaseTimeout string, limit int) ([]*operation.InstallationOperation, error) {
	var result []*operation.InstallationOperation
	for _, op := range r.ops {
		if op.Status == operation.OpStatusWaitingRuntimeACK {
			result = append(result, op)
		}
	}
	return result, nil
}

func (r *testRecoveryRepo) RenewOperationLease(operationID, executionID string) error {
	return nil
}

func (r *testRecoveryRepo) ReleaseOperationLease(operationID, executionID string) error {
	return nil
}

func (r *testRecoveryRepo) ClaimOperationLease(operationID, owner string, ttl time.Duration, expectedStatuses []string) (*operation.InstallationOperation, error) {
	op, ok := r.ops[operationID]
	if !ok {
		return nil, errors.New("not found")
	}
	return op, nil
}

func (r *testRecoveryRepo) UpdateOperationStatus(operationID, oldStatus, newStatus, executionID string) (*operation.InstallationOperation, error) {
	op, ok := r.ops[operationID]
	if !ok {
		return nil, errors.New("not found")
	}
	op.Status = newStatus
	r.ops[operationID] = op
	return op, nil
}

func (r *testRecoveryRepo) CASUpdateOperationStage(operationID, expectedStage, newStage, executionID string) (*operation.InstallationOperation, error) {
	op, ok := r.ops[operationID]
	if !ok {
		return nil, errors.New("not found")
	}
	if op.Stage != expectedStage {
		return nil, ErrCASConflict
	}
	op.Stage = newStage
	r.ops[operationID] = op
	return op, nil
}

func (r *testRecoveryRepo) CompleteOperation(operationID, expectedStage, expectedStatus, executionID string) (*operation.InstallationOperation, error) {
	op, ok := r.ops[operationID]
	if !ok {
		return nil, errors.New("not found")
	}
	if op.Stage != expectedStage || op.Status != expectedStatus {
		return nil, ErrCASConflict
	}
	op.Stage = operation.OpStageCompleted
	op.Status = operation.OpStatusCompleted
	r.ops[operationID] = op
	return op, nil
}

func (r *testRecoveryRepo) GetCommitJournal(operationID string) (*RecoveryCommitJournal, error) {
	journal, ok := r.commitJournals[operationID]
	if !ok {
		return nil, errors.New("not found")
	}
	return journal, nil
}

func (r *testRecoveryRepo) GetSwitchJournal(operationID string) (*RecoverySwitchJournal, error) {
	journal, ok := r.switchJournals[operationID]
	if !ok {
		return nil, errors.New("not found")
	}
	return journal, nil
}

func (r *testRecoveryRepo) CASUpdateCommitJournalStage(operationID, expectedStage, newStage, executionID string) (*RecoveryCommitJournal, error) {
	journal, ok := r.commitJournals[operationID]
	if !ok {
		return nil, ErrJournalNotFound
	}
	if journal.Stage != expectedStage {
		return nil, ErrCASConflict
	}
	journal.Stage = newStage
	r.commitJournals[operationID] = journal
	return journal, nil
}

func (r *testRecoveryRepo) CASUpdateSwitchJournalStage(operationID, expectedStage, newStage, executionID string) (*RecoverySwitchJournal, error) {
	journal, ok := r.switchJournals[operationID]
	if !ok {
		return nil, ErrJournalNotFound
	}
	if journal.Stage != expectedStage {
		return nil, ErrCASConflict
	}
	journal.Stage = newStage
	r.switchJournals[operationID] = journal
	return journal, nil
}

type testRuntimeRepo struct {
	forceNoTerminal bool
	queriedKeys     *[]string
}

func (testRuntimeRepo) SendDesiredCommand(context.Context, string, string, string, string, string, int64) error {
	return nil
}
func (testRuntimeRepo) CancelDesiredCommand(context.Context, string, string, string, string) error {
	return nil
}
func (testRuntimeRepo) QueryRuntimeAppliedState(context.Context, string, string, string) (int64, string, error) {
	return 1, "", nil
}
func (r testRuntimeRepo) QueryCommandTerminalStatusByIdempotencyKey(ctx context.Context, idempotencyKey string) (status string, found bool, err error) {
	if r.queriedKeys != nil {
		*r.queriedKeys = append(*r.queriedKeys, idempotencyKey)
	}
	if r.forceNoTerminal {
		return "", false, nil
	}
	return "completed", true, nil
}

type testSwitchRepo struct{}

func (testSwitchRepo) PublishSwitchDesired(context.Context, string, string, string, string, string, int64) error {
	return nil
}
func (testSwitchRepo) SendSwitchCommand(context.Context, string, string, string, string, string, int64) error {
	return nil
}
func (testSwitchRepo) QuerySwitchApplied(context.Context, string, string, string, int64) (bool, error) {
	return true, nil
}

func newTestWorkerWithRuntime(repo RecoveryRepo) *RecoveryWorker {
	worker := NewRecoveryWorker(repo)
	worker.runtimeRecovery = NewRuntimeRecovery(worker, repo, testRuntimeRepo{})
	return worker
}

func TestRecoveryWorker_UninstallOperation_IsRecovered(t *testing.T) {
	repo := newTestRepo()

	worker := newTestWorkerWithRuntime(repo)

	op := &operation.InstallationOperation{
		ID:            "uninstall-op-1",
		OperationType: operation.TypeUninstall,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageWaitingRuntimeACK,
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecoveryWorker_EnableOperation_IsRecovered(t *testing.T) {
	repo := newTestRepo()

	worker := newTestWorkerWithRuntime(repo)

	op := &operation.InstallationOperation{
		ID:            "enable-op-1",
		OperationType: operation.TypeEnable,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageWaitingRuntimeACK,
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecoveryWorker_RecenterOperation_IsRecovered(t *testing.T) {
	repo := newTestRepo()

	worker := newTestWorkerWithRuntime(repo)

	op := &operation.InstallationOperation{
		ID:            "recenter-op-1",
		OperationType: operation.TypeRecenter,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageWaitingRuntimeACK,
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecoveryWorker_InstallOperation_UsesCommitRecovery(t *testing.T) {
	repo := newTestRepo()

	worker := newTestWorkerWithRuntime(repo)

	op := &operation.InstallationOperation{
		ID:            "install-op-1",
		OperationType: operation.TypeInstall,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageWaitingRuntimeACK,
	}
	repo.ops[op.ID] = op
	repo.commitJournals[op.ID] = &RecoveryCommitJournal{
		OperationID: op.ID,
		Stage:       operation.OpStageWaitingRuntimeACK,
	}

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecoveryWorker_SwitchOperation_UsesSwitchRecovery(t *testing.T) {
	repo := newTestRepo()

	worker := NewRecoveryWorker(repo)
	worker.switchRecovery = NewSwitchRecovery(worker, repo, testSwitchRepo{})

	op := &operation.InstallationOperation{
		ID:            "switch-op-1",
		OperationType: operation.TypeSwitch,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageWaitingRuntimeACK,
	}
	repo.ops[op.ID] = op
	repo.switchJournals[op.ID] = &RecoverySwitchJournal{
		OperationID:        op.ID,
		Stage:              SwitchStageRuntimeApplied,
		NewInstallationID:  "installation-new",
		NewDesiredRevision: 1,
	}

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecoveryWorker_InactiveOperation_IsSkipped(t *testing.T) {
	repo := newTestRepo()

	worker := NewRecoveryWorker(repo)

	op := &operation.InstallationOperation{
		ID:            "completed-op-1",
		OperationType: operation.TypeUninstall,
		Status:        operation.OpStatusCompleted,
		Stage:         operation.OpStageCompleted,
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecoveryWorker_MissingRuntimeRecoveryFailsClosed(t *testing.T) {
	repo := newTestRepo()
	worker := NewRecoveryWorker(repo)
	op := &operation.InstallationOperation{
		ID: "missing-runtime", OperationType: operation.TypeEnable,
		Status: operation.OpStatusWaitingRuntimeACK, Stage: operation.OpStageWaitingRuntimeACK,
	}
	repo.ops[op.ID] = op
	if err := worker.recoverOperation(context.Background(), op); err == nil {
		t.Fatal("expected missing runtime recovery to fail closed")
	}
}

func TestRecoveryWorker_MissingCommitJournalFailsClosed(t *testing.T) {
	repo := newTestRepo()
	worker := newTestWorkerWithRuntime(repo)
	op := &operation.InstallationOperation{
		ID: "missing-journal", OperationType: operation.TypeInstall,
		Status: operation.OpStatusWaitingRuntimeACK, Stage: operation.OpStageWaitingRuntimeACK,
	}
	repo.ops[op.ID] = op
	if err := worker.recoverOperation(context.Background(), op); err == nil {
		t.Fatal("expected missing commit journal to fail closed")
	}
}

func TestRecoveryWorker_DesiredStateCommitted_RecoversAndEnqueuesCommand(t *testing.T) {
	repo := newTestRepo()
	worker := newTestWorkerWithRuntime(repo)

	op := &operation.InstallationOperation{
		ID:            "desired-committed-op-1",
		OperationType: operation.TypeEnable,
		Status:        operation.OpStatusRunning,
		Stage:         operation.OpStageDesiredStateCommitted,
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := repo.ops[op.ID]
	if got.Stage != operation.OpStageWaitingRuntimeACK || got.Status != operation.OpStatusWaitingRuntimeACK {
		t.Fatalf("expected operation to advance to waiting_runtime_ack, got stage=%s status=%s", got.Stage, got.Status)
	}
}

func TestRecoveryWorker_RuntimeApplied_CallsFinalizer(t *testing.T) {
	repo := newTestRepo()
	worker := newTestWorkerWithRuntime(repo)

	finalizerCalled := false
	worker.desiredStateFinalizer = &mockDesiredStateFinalizer{
		onFinalize: func(ctx context.Context, op *operation.InstallationOperation) error {
			finalizerCalled = true
			if op.ID != "runtime-applied-op-1" {
				t.Errorf("unexpected operation ID: %s", op.ID)
			}
			return nil
		},
	}

	op := &operation.InstallationOperation{
		ID:            "runtime-applied-op-1",
		OperationType: operation.TypeEnable,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageRuntimeApplied,
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !finalizerCalled {
		t.Fatal("expected finalizer to be called for runtime_applied stage")
	}
	got := repo.ops[op.ID]
	if got.Stage != operation.OpStageCompleted || got.Status != operation.OpStatusCompleted {
		t.Fatalf("expected runtime_applied recovery to complete operation, got stage=%s status=%s", got.Stage, got.Status)
	}
}

func TestRecoveryWorker_RuntimeApplied_MissingFinalizerFailsClosed(t *testing.T) {
	repo := newTestRepo()
	worker := newTestWorkerWithRuntime(repo)
	worker.desiredStateFinalizer = nil

	op := &operation.InstallationOperation{
		ID:            "runtime-applied-no-finalizer",
		OperationType: operation.TypeEnable,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageRuntimeApplied,
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err == nil {
		t.Fatal("expected missing finalizer to fail closed")
	}
}

func TestRecoveryWorker_UnknownStage_FailsWithManualReview(t *testing.T) {
	repo := newTestRepo()
	worker := newTestWorkerWithRuntime(repo)

	op := &operation.InstallationOperation{
		ID:            "unknown-stage-op",
		OperationType: operation.TypeEnable,
		Status:        operation.OpStatusRunning,
		Stage:         "unknown_stage",
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err == nil {
		t.Fatal("expected unknown stage to fail with manual review error")
	}
	if !strings.Contains(err.Error(), "manual review") {
		t.Fatalf("expected manual review error, got: %v", err)
	}
}

type mockDesiredStateFinalizer struct {
	onFinalize func(ctx context.Context, op *operation.InstallationOperation) error
}

func (m *mockDesiredStateFinalizer) FinalizeDesiredStateApplied(ctx context.Context, op *operation.InstallationOperation) error {
	if m.onFinalize != nil {
		return m.onFinalize(ctx, op)
	}
	return nil
}

func TestRecoveryWorker_RecenterOperation_TerminalACKCompletes(t *testing.T) {
	repo := newTestRepo()
	worker := newTestWorkerWithRuntime(repo)

	op := &operation.InstallationOperation{
		ID:            "recenter-terminal-ack",
		OperationType: operation.TypeRecenter,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageWaitingRuntimeACK,
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := repo.ops[op.ID]
	if got.Stage != operation.OpStageCompleted || got.Status != operation.OpStatusCompleted {
		t.Fatalf("expected terminal ACK to complete recenter operation, got stage=%s status=%s", got.Stage, got.Status)
	}
}

func TestRecoveryWorker_RecenterOperation_NoTerminalACKDoesNotComplete(t *testing.T) {
	repo := newTestRepo()
	worker := NewRecoveryWorker(repo)
	worker.runtimeRecovery = NewRuntimeRecovery(worker, repo, testRuntimeRepo{forceNoTerminal: true})

	op := &operation.InstallationOperation{
		ID:            "recenter-no-ack",
		OperationType: operation.TypeRecenter,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageWaitingRuntimeACK,
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.ops[op.ID].Status == operation.OpStatusCompleted {
		t.Fatal("recenter without terminal ACK must not complete")
	}
}

func TestRecoveryWorker_TwoConsecutiveRecenters_GenerateIndependentCommands(t *testing.T) {
	repo := newTestRepo()
	queried := []string{}
	worker := NewRecoveryWorker(repo)
	worker.runtimeRecovery = NewRuntimeRecovery(worker, repo, testRuntimeRepo{queriedKeys: &queried})

	op1 := &operation.InstallationOperation{
		ID:            "recenter-1",
		OperationType: operation.TypeRecenter,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageWaitingRuntimeACK,
	}
	op2 := &operation.InstallationOperation{
		ID:            "recenter-2",
		OperationType: operation.TypeRecenter,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageWaitingRuntimeACK,
	}
	repo.ops[op1.ID] = op1
	repo.ops[op2.ID] = op2

	err := worker.recoverOperation(context.Background(), op1)
	if err != nil {
		t.Fatalf("unexpected error for op1: %v", err)
	}
	err = worker.recoverOperation(context.Background(), op2)
	if err != nil {
		t.Fatalf("unexpected error for op2: %v", err)
	}
	if len(queried) != 2 || queried[0] != "recenter:"+op1.ID || queried[1] != "recenter:"+op2.ID {
		t.Fatalf("recenter operations must query independent original command keys, got %v", queried)
	}
}

func TestRecoveryWorker_UninstallOperation_CrashAfterMoveBeforeDBCommit(t *testing.T) {
	repo := newTestRepo()
	worker := newTestWorkerWithRuntime(repo)

	finalizerCalled := false
	worker.desiredStateFinalizer = &mockDesiredStateFinalizer{
		onFinalize: func(ctx context.Context, op *operation.InstallationOperation) error {
			finalizerCalled = true
			if op.OperationType != operation.TypeUninstall {
				t.Errorf("expected uninstall operation, got: %s", op.OperationType)
			}
			return nil
		},
	}

	op := &operation.InstallationOperation{
		ID:            "uninstall-crash-after-move",
		OperationType: operation.TypeUninstall,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageRuntimeApplied,
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !finalizerCalled {
		t.Fatal("expected finalizer to be called for uninstall runtime_applied stage")
	}
}

func TestRecoveryWorker_UninstallOperation_IdempotentCompletion(t *testing.T) {
	repo := newTestRepo()
	worker := newTestWorkerWithRuntime(repo)

	callCount := 0
	worker.desiredStateFinalizer = &mockDesiredStateFinalizer{
		onFinalize: func(ctx context.Context, op *operation.InstallationOperation) error {
			callCount++
			return nil
		},
	}

	op := &operation.InstallationOperation{
		ID:            "uninstall-idempotent",
		OperationType: operation.TypeUninstall,
		Status:        operation.OpStatusWaitingRuntimeACK,
		Stage:         operation.OpStageRuntimeApplied,
	}
	repo.ops[op.ID] = op

	err := worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = worker.recoverOperation(context.Background(), op)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected idempotent finalizer to run once, got %d", callCount)
	}
}

func TestRecoveryWorker_CancelInFlightWithoutRuntimeRecoveryFailsClosed(t *testing.T) {
	repo := newTestRepo()
	worker := NewRecoveryWorker(repo)
	op := &operation.InstallationOperation{
		ID: "cancel-inflight", OperationType: operation.TypeEnable,
		Status: operation.OpStatusCancelRequested, Stage: operation.OpStageWaitingRuntimeACK,
	}
	repo.ops[op.ID] = op
	if err := worker.recoverOperation(context.Background(), op); err == nil {
		t.Fatal("expected in-flight cancel without runtime recovery to fail closed")
	}
}
