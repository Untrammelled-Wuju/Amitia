// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

var ErrRunnerNotInitialized = errors.New("migration runner: no plans registered")

type DomainLock interface {
	Acquire(owner string, ttl time.Duration) error
	Release(owner string) error
}

type BackupPort interface {
	CreateBackup(ctx context.Context) (backupID string, err error)
}

type Runner struct {
	repo      *DBRepository
	lock      DomainLock
	backup    BackupPort
	plans     map[string]DomainMigrationOperationPlan
	leaseTTL  time.Duration
	backupDir string
}

func NewRunner(repo *DBRepository) *Runner {
	return &Runner{
		repo:     repo,
		plans:    make(map[string]DomainMigrationOperationPlan),
		leaseTTL: 5 * time.Minute,
	}
}

func (r *Runner) SetLock(lock DomainLock) {
	r.lock = lock
}

func (r *Runner) SetBackupPort(backup BackupPort) {
	r.backup = backup
}

func (r *Runner) SetBackupDir(dir string) {
	r.backupDir = dir
}

func (r *Runner) RegisterPlan(plan DomainMigrationOperationPlan) {
	if r.plans == nil {
		r.plans = make(map[string]DomainMigrationOperationPlan)
	}
	r.plans[plan.ID] = plan
}

func (r *Runner) InitializeSchema(ctx context.Context) error {
	return nil
}

type DomainMigrationOperationPlan struct {
	ID                    string
	Domain                string
	SourceVersion         string
	TargetVersion         string
	PreflightChecks       []CheckFunc
	BackupRequired        bool
	SchemaSteps           []StepFunc
	BackfillSteps         []BatchedStepFunc
	VerificationChecks    []CheckFunc
	CutoverSteps          []StepFunc
	LegacyWriteBlockSteps []StepFunc
	ParityChecks          []ParityCheck
}

type ParityCheck struct {
	Name     string
	Required bool
	Check    func(ctx context.Context) (passed bool, detail string, err error)
}

const batchSize = 500

func (r *Runner) RunPlan(ctx context.Context, planID string) (string, error) {
	plan, ok := r.plans[planID]
	if !ok {
		return "", &RunnerError{Code: "PLAN_NOT_FOUND", Message: "迁移计划未注册: " + planID}
	}

	op := &MigrationOperation{
		ID:            uuid.New().String(),
		PlanID:        planID,
		SourceVersion: plan.SourceVersion,
		TargetVersion: plan.TargetVersion,
		Stage:         StagePreflight,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	if err := r.repo.CreateOperation(ctx, op); err != nil {
		return "", &RunnerError{Code: "OPERATION_CREATE_FAILED", Message: "创建迁移操作失败: " + err.Error()}
	}

	owner := fmt.Sprintf("runner-%s", op.ID)
	if r.lock != nil {
		if err := r.lock.Acquire(owner, r.leaseTTL); err != nil {
			return op.ID, &RunnerError{Code: "LOCK_ACQUIRE_FAILED", Message: "获取迁移锁失败: " + err.Error()}
		}
		defer r.lock.Release(owner)
	}

	go r.executePlan(op.ID, &plan)

	return op.ID, nil
}

func (r *Runner) executePlan(operationID string, plan *DomainMigrationOperationPlan) {
	ctx := context.Background()
	op, err := r.repo.GetOperation(ctx, operationID)
	if err != nil || op == nil {
		return
	}

	defer func() {
		if rec := recover(); rec != nil {
			r.repo.UpdateOperationStageCAS(ctx, operationID, MigrationStage(op.Stage), StageFailedTerminal, func(o *MigrationOperation) {
				o.Error = fmt.Sprintf("panic: %v", rec)
			})
		}
	}()

	step_fns := []struct {
		name MigrationStage
		fn   func(*MigrationOperation) error
	}{
		{StagePreflight, r.runPreflight},
		{StageBackup, r.runBackup},
		{StageSchema, r.runSchema},
		{StageBackfill, r.runBackfill},
		{StageVerifying, r.runVerifying},
		{StageReadCutover, r.runReadCutover},
		{StageWriteCutover, r.runWriteCutover},
		{StageLegacyWriteBlocked, r.runLegacyWriteBlock},
		{StageCompleted, nil},
	}

	for i := 0; i < len(step_fns); i++ {
		op, err = r.repo.GetOperation(ctx, operationID)
		if err != nil || op == nil {
			return
		}
		if op.Stage == StageFailedRetryable || op.Stage == StageFailedTerminal || op.Stage == StageManualReview {
			return
		}
		step := step_fns[i]
		if step.fn == nil {
			r.repo.UpdateOperationStageCAS(ctx, operationID, MigrationStage(op.Stage), StageCompleted, nil)
			return
		}
		op.Updated()
		r.repo.UpdateOperationCheckpoint(ctx, op)
		if err := step.fn(op); err != nil {
			var me *RunnerError
			if errors.As(err, &me) && me.Code == "MANUAL_REVIEW_REQUIRED" {
				r.repo.UpdateOperationStageCAS(ctx, operationID, MigrationStage(op.Stage), StageManualReview, func(o *MigrationOperation) {
					o.Error = err.Error()
				})
			} else {
				r.repo.UpdateOperationStageCAS(ctx, operationID, MigrationStage(op.Stage), StageFailedRetryable, func(o *MigrationOperation) {
					o.Error = err.Error()
				})
			}
			return
		}
	}
}

func (r *Runner) runPreflight(op *MigrationOperation) error {
	ctx := context.Background()
	if _, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StagePreflight, StageBackup, nil); err != nil {
		return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "Preflight 阶段转换失败: " + err.Error()}
	}

	plan := r.plans[op.PlanID]
	for _, check := range plan.PreflightChecks {
		passed, msg := check()
		if !passed {
			return &RunnerError{Code: "PREFLIGHT_FAILED", Message: "前置检查失败: " + msg}
		}
	}
	return nil
}

func (r *Runner) runBackup(op *MigrationOperation) error {
	ctx := context.Background()
	if _, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageBackup, StageSchema, nil); err != nil {
		return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "Backup 阶段转换失败: " + err.Error()}
	}

	plan := r.plans[op.PlanID]
	if plan.BackupRequired && r.backup != nil {
		backupID, err := r.backup.CreateBackup(ctx)
		if err != nil {
			return &RunnerError{Code: "BACKUP_FAILED", Message: "备份失败: " + err.Error()}
		}
		_ = backupID
	}
	return nil
}

func (r *Runner) runSchema(op *MigrationOperation) error {
	ctx := context.Background()
	if _, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageSchema, StageBackfill, nil); err != nil {
		return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "Schema 阶段转换失败: " + err.Error()}
	}

	plan := r.plans[op.PlanID]
	for _, step := range plan.SchemaSteps {
		if err := step(); err != nil {
			return &RunnerError{Code: "SCHEMA_STEP_FAILED", Message: "Schema 步骤失败: " + err.Error()}
		}
	}
	return nil
}

func (r *Runner) runBackfill(op *MigrationOperation) error {
	ctx := context.Background()
	plan := r.plans[op.PlanID]
	if len(plan.BackfillSteps) == 0 {
		if _, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageBackfill, StageVerifying, nil); err != nil {
			return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "Backfill 阶段转换失败: " + err.Error()}
		}
		return nil
	}

	_, lastPK, processedCount, conflictCount, err := r.repo.LoadCheckpoint(ctx, op.ID)
	if err != nil {
		return &RunnerError{Code: "CHECKPOINT_LOAD_FAILED", Message: "加载检查点失败: " + err.Error()}
	}
	_ = lastPK
	var processedTotal int64
	var conflictsTotal int64
	processedTotal = int64(processedCount)
	conflictsTotal = int64(conflictCount)

	for batchIdx, step := range plan.BackfillSteps {
		offset := int(processedTotal)
		for {
			processed, conflicts, err := step(offset, batchSize)
			if err != nil {
				return &RunnerError{Code: "BACKFILL_STEP_FAILED", Message: fmt.Sprintf("回填步骤 %d 失败: %v", batchIdx, err)}
			}
			processedTotal += int64(processed)
			conflictsTotal += int64(conflicts)

			stepName := fmt.Sprintf("backfill_batch_%d", batchIdx)
			inputHash := ComputeCheckpoint(input{PK: int64(offset), BatchIdx: batchIdx})
			r.repo.SaveCheckpoint(ctx, op.ID, stepName, fmt.Sprintf("%d", offset), int(processedTotal), inputHash, "", int(conflictsTotal))

			if processed < batchSize {
				break
			}
			offset += batchSize
		}
	}

	if _, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageBackfill, StageVerifying, nil); err != nil {
		return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "Backfill 阶段转换到 Verifying 失败: " + err.Error()}
	}
	return nil
}

type input struct {
	PK       int64
	BatchIdx int
}

func ComputeCheckpoint(in input) string {
	data, _ := json.Marshal(in)
	return ComputeChecksum(string(data))
}

func (r *Runner) runVerifying(op *MigrationOperation) error {
	ctx := context.Background()
	if _, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageVerifying, StageReadCutover, nil); err != nil {
		return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "Verifying 阶段转换失败: " + err.Error()}
	}

	plan := r.plans[op.PlanID]

	openConflicts, err := r.repo.CountOpenConflicts(ctx, op.ID)
	if err != nil {
		return &RunnerError{Code: "CONFLICT_COUNT_FAILED", Message: "冲突计数失败: " + err.Error()}
	}
	if openConflicts > 0 {
		return &RunnerError{Code: "OPEN_CONFLICTS", Message: fmt.Sprintf("存在 %d 个未解决冲突", openConflicts)}
	}

	for _, pCheck := range plan.ParityChecks {
		passed, detail, err := pCheck.Check(ctx)
		if err != nil {
			return &RunnerError{Code: "PARITY_CHECK_ERROR", Message: fmt.Sprintf("Parity 检查 %s 出错: %v", pCheck.Name, err)}
		}
		if !passed {
			if pCheck.Required {
				return &RunnerError{Code: "MANUAL_REVIEW_REQUIRED", Message: fmt.Sprintf("Parity 检查 %s 失败: %s", pCheck.Name, detail)}
			}
		}
	}

	for _, check := range plan.VerificationChecks {
		passed, msg := check()
		if !passed {
			return &RunnerError{Code: "VERIFICATION_FAILED", Message: "验证失败: " + msg}
		}
	}
	return nil
}

func (r *Runner) runReadCutover(op *MigrationOperation) error {
	ctx := context.Background()
	if _, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageReadCutover, StageWriteCutover, nil); err != nil {
		return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "ReadCutover 阶段转换失败: " + err.Error()}
	}

	if err := r.repo.RecordReadCutover(ctx, op.ID, op.PlanID); err != nil {
		return &RunnerError{Code: "READ_CUTOVER_RECORD_FAILED", Message: "记录读切换失败: " + err.Error()}
	}
	return nil
}

func (r *Runner) runWriteCutover(op *MigrationOperation) error {
	ctx := context.Background()

	plan := r.plans[op.PlanID]
	for _, pCheck := range plan.ParityChecks {
		if !pCheck.Required {
			continue
		}
		passed, detail, err := pCheck.Check(ctx)
		if err != nil {
			return &RunnerError{Code: "PARITY_CHECK_ERROR", Message: fmt.Sprintf("Parity 检查 %s 出错: %v", pCheck.Name, err)}
		}
		if !passed {
			return &RunnerError{Code: "MANUAL_REVIEW_REQUIRED", Message: fmt.Sprintf("Parity 检查 %s 失败: %s", pCheck.Name, detail)}
		}
	}

	if _, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageWriteCutover, StageLegacyWriteBlocked, nil); err != nil {
		return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "WriteCutover 阶段转换失败: " + err.Error()}
	}

	if err := r.repo.RecordWriteCutover(ctx, op.ID, op.PlanID); err != nil {
		return &RunnerError{Code: "WRITE_CUTOVER_RECORD_FAILED", Message: "记录写切换失败: " + err.Error()}
	}
	return nil
}

func (r *Runner) runLegacyWriteBlock(op *MigrationOperation) error {
	ctx := context.Background()
	if _, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageLegacyWriteBlocked, StageCompleted, nil); err != nil {
		return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "LegacyWriteBlock 阶段转换失败: " + err.Error()}
	}

	plan := r.plans[op.PlanID]
	for _, step := range plan.LegacyWriteBlockSteps {
		if err := step(); err != nil {
			return &RunnerError{Code: "LEGACY_BLOCK_FAILED", Message: "旧写阻断失败: " + err.Error()}
		}
	}
	return nil
}

func (r *Runner) GetOperation(ctx context.Context, operationID string) (*MigrationOperation, error) {
	if r.repo == nil {
		return nil, &RunnerError{Code: "NOT_INITIALIZED", Message: "迁移仓库未初始化"}
	}
	return r.repo.GetOperation(ctx, operationID)
}

func (r *Runner) RequestCutover(ctx context.Context, operationID, direction string) error {
	if operationID == "" {
		return &RunnerError{Code: "OPERATION_REQUIRED", Message: "operationId 不能为空"}
	}
	if direction != "read" && direction != "write" {
		return &RunnerError{Code: "INVALID_DIRECTION", Message: "切转方向无效: " + direction}
	}

	op, err := r.repo.GetOperation(ctx, operationID)
	if err != nil {
		return &RunnerError{Code: "OPERATION_GET_FAILED", Message: "获取迁移操作失败: " + err.Error()}
	}
	if op == nil {
		return &RunnerError{Code: "OPERATION_NOT_FOUND", Message: "迁移操作不存在"}
	}

	openConflicts, err := r.repo.CountOpenConflicts(ctx, operationID)
	if err != nil {
		return &RunnerError{Code: "CONFLICT_COUNT_FAILED", Message: "冲突计数失败: " + err.Error()}
	}
	if openConflicts > 0 {
		return &RunnerError{Code: "OPEN_CONFLICTS", Message: fmt.Sprintf("存在 %d 个未解决冲突，不允许切转", openConflicts)}
	}

	plan, ok := r.plans[op.PlanID]
	if !ok {
		return &RunnerError{Code: "PLAN_NOT_FOUND", Message: "迁移计划未注册: " + op.PlanID}
	}

	backupExists := true
	if plan.BackupRequired && r.backupDir != "" {
		entries, err := os.ReadDir(r.backupDir)
		if err != nil || len(entries) == 0 {
			backupExists = false
		}
	}
	if !backupExists {
		return &RunnerError{Code: "BACKUP_MISSING", Message: "备份缺失，不允许切转"}
	}

	switch direction {
	case "read":
		if op.Stage != StageVerifying && op.Stage != StageReadCutover {
			return &RunnerError{Code: "INVALID_STAGE", Message: fmt.Sprintf("当前阶段 %s 不允许读切转", op.Stage)}
		}
		for _, pCheck := range plan.ParityChecks {
			if !pCheck.Required {
				continue
			}
			passed, detail, err := pCheck.Check(ctx)
			if err != nil {
				return &RunnerError{Code: "PARITY_CHECK_ERROR", Message: fmt.Sprintf("Parity 检查 %s 出错: %v", pCheck.Name, err)}
			}
			if !passed {
				return &RunnerError{Code: "PARITY_FAILED", Message: fmt.Sprintf("Parity 检查 %s 失败: %s", pCheck.Name, detail)}
			}
		}
		_, _ = r.repo.UpdateOperationStageCAS(ctx, operationID, MigrationStage(op.Stage), StageReadCutover, nil)
		r.repo.RecordReadCutover(ctx, operationID, op.PlanID)
	case "write":
		if op.Stage != StageReadCutover && op.Stage != StageWriteCutover {
			return &RunnerError{Code: "INVALID_STAGE", Message: fmt.Sprintf("当前阶段 %s 不允许写切转", op.Stage)}
		}
		for _, pCheck := range plan.ParityChecks {
			if !pCheck.Required {
				continue
			}
			passed, detail, err := pCheck.Check(ctx)
			if err != nil {
				return &RunnerError{Code: "PARITY_CHECK_ERROR", Message: fmt.Sprintf("Parity 检查 %s 出错: %v", pCheck.Name, err)}
			}
			if !passed {
				return &RunnerError{Code: "PARITY_FAILED", Message: fmt.Sprintf("Parity 检查 %s 失败: %s", pCheck.Name, detail)}
			}
		}
		_, _ = r.repo.UpdateOperationStageCAS(ctx, operationID, MigrationStage(op.Stage), StageWriteCutover, nil)
		r.repo.RecordWriteCutover(ctx, operationID, op.PlanID)
	}
	return nil
}

func (r *Runner) ensureBackupDir() error {
	if r.backupDir == "" {
		return nil
	}
	return os.MkdirAll(r.backupDir, 0o700)
}

func (r *Runner) SetLeaseTTL(ttl time.Duration) {
	r.leaseTTL = ttl
}

type RunnerError struct {
	Code    string
	Message string
	Err     error
}

func (e *RunnerError) Error() string {
	return e.Message
}

func (op *MigrationOperation) Updated() {
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}
