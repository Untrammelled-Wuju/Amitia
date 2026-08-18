package migration

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRunnerTestRepo(t *testing.T) *DBRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "migration.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&operationRecord{}, &checkpointRecord{}, &conflictRecord{}, &readCutoverRecord{}, &writeCutoverRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewDBRepository(db)
}

func createRunnerOperation(t *testing.T, repo *DBRepository, planID string, stage MigrationStage) *MigrationOperation {
	t.Helper()
	op := &MigrationOperation{
		ID:            "op-" + string(stage),
		PlanID:        planID,
		SourceVersion: "legacy",
		TargetVersion: "v2",
		Stage:         stage,
		StartedAt:     "2026-08-18T00:00:00Z",
		UpdatedAt:     "2026-08-18T00:00:00Z",
	}
	if err := repo.CreateOperation(context.Background(), op); err != nil {
		t.Fatalf("create operation: %v", err)
	}
	return op
}

func TestRunnerAutomaticPlanStopsAtVerifying(t *testing.T) {
	repo := newRunnerTestRepo(t)
	runner := NewRunner(repo)
	plan := DomainMigrationOperationPlan{
		ID:            "test-plan",
		SourceVersion: "legacy",
		TargetVersion: "v2",
		PreflightChecks: []CheckFunc{
			func() (bool, string) { return true, "" },
		},
		VerificationChecks: []CheckFunc{
			func() (bool, string) { return true, "" },
		},
	}
	runner.RegisterPlan(plan)
	op := createRunnerOperation(t, repo, plan.ID, StagePreflight)

	runner.executePlan(op.ID, &plan)

	got, err := repo.GetOperation(context.Background(), op.ID)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if got.Stage != StageVerifying {
		t.Fatalf("stage=%s want=%s", got.Stage, StageVerifying)
	}
}

func TestReadCutoverExecutesAndVerifiesBeforeStageCAS(t *testing.T) {
	repo := newRunnerTestRepo(t)
	runner := NewRunner(repo)
	stepRan := false
	plan := DomainMigrationOperationPlan{
		ID: "test-plan",
		ReadCutoverSteps: []StepFunc{
			func() error { stepRan = true; return nil },
		},
		ReadCutoverChecks: []CheckFunc{
			func() (bool, string) { return stepRan, "step must run before verification" },
		},
	}
	runner.RegisterPlan(plan)
	op := createRunnerOperation(t, repo, plan.ID, StageVerifying)

	if err := runner.RequestCutover(context.Background(), op.ID, "read"); err != nil {
		t.Fatalf("read cutover: %v", err)
	}
	if !stepRan {
		t.Fatal("read cutover step did not run")
	}
	verified, err := repo.HasVerifiedReadCutover(context.Background(), op.ID)
	if err != nil || !verified {
		t.Fatalf("read verified=%v err=%v", verified, err)
	}
	got, _ := repo.GetOperation(context.Background(), op.ID)
	if got.Stage != StageReadCutover {
		t.Fatalf("stage=%s want=%s", got.Stage, StageReadCutover)
	}
}

func TestWriteCutoverRequiresVerifiedRead(t *testing.T) {
	repo := newRunnerTestRepo(t)
	runner := NewRunner(repo)
	plan := DomainMigrationOperationPlan{ID: "test-plan"}
	runner.RegisterPlan(plan)
	op := createRunnerOperation(t, repo, plan.ID, StageReadCutover)

	err := runner.RequestCutover(context.Background(), op.ID, "write")
	var runnerErr *RunnerError
	if !errors.As(err, &runnerErr) || runnerErr.Code != "READ_CUTOVER_NOT_VERIFIED" {
		t.Fatalf("err=%v want READ_CUTOVER_NOT_VERIFIED", err)
	}
}

func TestWriteCutoverRecordsBothLegacyWriteGates(t *testing.T) {
	repo := newRunnerTestRepo(t)
	runner := NewRunner(repo)
	plan := DomainMigrationOperationPlan{
		ID: "test-plan",
		WriteCutoverSteps: []StepFunc{
			func() error { return nil },
		},
		WriteCutoverChecks: []CheckFunc{
			func() (bool, string) { return true, "" },
		},
	}
	runner.RegisterPlan(plan)
	op := createRunnerOperation(t, repo, plan.ID, StageReadCutover)
	if err := repo.RecordReadCutover(context.Background(), op.ID, "v2_read_path"); err != nil {
		t.Fatalf("record read: %v", err)
	}
	if err := repo.MarkReadCutoverVerified(context.Background(), op.ID, "v2_read_path"); err != nil {
		t.Fatalf("verify read: %v", err)
	}

	if err := runner.RequestCutover(context.Background(), op.ID, "write"); err != nil {
		t.Fatalf("write cutover: %v", err)
	}
	verified, err := repo.HasVerifiedWriteCutover(context.Background(), op.ID)
	if err != nil || !verified {
		t.Fatalf("write verified=%v err=%v", verified, err)
	}
	for _, step := range []string{"installation", "editing"} {
		var count int64
		if err := repo.DB().Model(&writeCutoverRecord{}).
			Where("operation_id = ? AND step_name = ? AND verified = 1", op.ID, step).
			Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", step, err)
		}
		if count != 1 {
			t.Fatalf("step %s verified rows=%d want=1", step, count)
		}
	}
}

type unavailableBackup struct{}

func (unavailableBackup) CreateBackup(context.Context) (string, error)       { return "backup-1", nil }
func (unavailableBackup) BackupExists(context.Context, string) (bool, error) { return false, nil }

func TestCutoverRequiresExactOperationBackup(t *testing.T) {
	repo := newRunnerTestRepo(t)
	runner := NewRunner(repo)
	runner.SetBackupPort(unavailableBackup{})
	plan := DomainMigrationOperationPlan{ID: "test-plan", BackupRequired: true}
	runner.RegisterPlan(plan)
	op := createRunnerOperation(t, repo, plan.ID, StageVerifying)
	op.BackupID = "backup-1"
	if err := repo.UpdateOperationCheckpoint(context.Background(), op); err != nil {
		t.Fatalf("persist backup id: %v", err)
	}

	err := runner.RequestCutover(context.Background(), op.ID, "read")
	var runnerErr *RunnerError
	if !errors.As(err, &runnerErr) || runnerErr.Code != "BACKUP_NOT_FOUND" {
		t.Fatalf("err=%v want BACKUP_NOT_FOUND", err)
	}
}

func TestReadCutoverFailureDoesNotPersistVerifiedRecordOrAdvanceStage(t *testing.T) {
	repo := newRunnerTestRepo(t)
	runner := NewRunner(repo)
	plan := DomainMigrationOperationPlan{
		ID: "read-failure-plan",
		ReadCutoverSteps: []StepFunc{
			func() error { return errors.New("read step failed") },
		},
	}
	runner.RegisterPlan(plan)
	op := createRunnerOperation(t, repo, plan.ID, StageVerifying)

	if err := runner.RequestCutover(context.Background(), op.ID, "read"); err == nil {
		t.Fatal("expected read cutover failure")
	}
	got, err := repo.GetOperation(context.Background(), op.ID)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if got.Stage != StageVerifying {
		t.Fatalf("stage=%s want=%s", got.Stage, StageVerifying)
	}
	var count int64
	if err := repo.DB().Model(&readCutoverRecord{}).Where("operation_id = ?", op.ID).Count(&count).Error; err != nil {
		t.Fatalf("count read cutovers: %v", err)
	}
	if count != 0 {
		t.Fatalf("read cutover rows=%d want=0", count)
	}
}

func TestWriteCutoverFailureDoesNotPersistVerifiedRecordOrAdvanceStage(t *testing.T) {
	repo := newRunnerTestRepo(t)
	runner := NewRunner(repo)
	plan := DomainMigrationOperationPlan{
		ID: "write-failure-plan",
		WriteCutoverSteps: []StepFunc{
			func() error { return errors.New("write step failed") },
		},
	}
	runner.RegisterPlan(plan)
	op := createRunnerOperation(t, repo, plan.ID, StageReadCutover)
	if err := repo.RecordReadCutover(context.Background(), op.ID, "v2_read_path"); err != nil {
		t.Fatalf("record read: %v", err)
	}
	if err := repo.MarkReadCutoverVerified(context.Background(), op.ID, "v2_read_path"); err != nil {
		t.Fatalf("verify read: %v", err)
	}

	if err := runner.RequestCutover(context.Background(), op.ID, "write"); err == nil {
		t.Fatal("expected write cutover failure")
	}
	got, err := repo.GetOperation(context.Background(), op.ID)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if got.Stage != StageReadCutover {
		t.Fatalf("stage=%s want=%s", got.Stage, StageReadCutover)
	}
	var count int64
	if err := repo.DB().Model(&writeCutoverRecord{}).Where("operation_id = ?", op.ID).Count(&count).Error; err != nil {
		t.Fatalf("count write cutovers: %v", err)
	}
	if count != 0 {
		t.Fatalf("write cutover rows=%d want=0", count)
	}
}
