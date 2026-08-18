// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"errors"
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

type testRuntimeRepo struct{}

func (testRuntimeRepo) SendDesiredCommand(context.Context, string, string, string, string, string, int64) error {
	return nil
}
func (testRuntimeRepo) CancelDesiredCommand(context.Context, string, string, string, string) error {
	return nil
}
func (testRuntimeRepo) QueryRuntimeAppliedState(context.Context, string, string, string) (int64, string, error) {
	return 1, "", nil
}
func (testRuntimeRepo) MarkRuntimeApplied(string, int64) error { return nil }

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
