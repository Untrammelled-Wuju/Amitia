// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	migrationcore "github.com/u-ai/backend/internal/migration"
)

var ErrRunnerNotInitialized = errors.New("migration runner: no plans registered")

type BackupPort interface {
	CreateBackup(ctx context.Context) (backupID string, err error)
	BackupExists(ctx context.Context, backupID string) (bool, error)
}

type Runner struct {
	repo      *DBRepository
	lock      *migrationcore.PersistentLock
	lockName  string
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
		lockName: "desktop_pet_migration",
	}
}

func (r *Runner) SetLock(lock *migrationcore.PersistentLock) {
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

	go r.executePlan(op.ID, &plan)

	return op.ID, nil
}

func (r *Runner) executePlan(operationID string, plan *DomainMigrationOperationPlan) {
	ctx := context.Background()
	op, err := r.repo.GetOperation(ctx, operationID)
	if err != nil || op == nil {
		return
	}

	if r.lock != nil {
		lockCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := r.lock.Acquire(lockCtx, r.lockName, r.leaseTTL); err != nil {
			_, _ = r.repo.UpdateOperationStageCAS(ctx, operationID, StagePreflight, StageFailedTerminal, func(o *MigrationOperation) {
				o.Error = "获取迁移锁失败: " + err.Error()
			})
			cancel()
			return
		}
		cancel()
		_ = r.lock.StartHeartbeat(r.lockName, 30*time.Second, r.leaseTTL)
		defer r.lock.Release(r.lockName)
	}

	defer func() {
		if rec := recover(); rec != nil {
			r.repo.UpdateOperationStageCAS(ctx, operationID, MigrationStage(op.Stage), StageFailedTerminal, func(o *MigrationOperation) {
				o.Error = fmt.Sprintf("panic: %v", rec)
			})
		}
	}()

	stages := []struct {
		from  MigrationStage
		to    MigrationStage
		stage MigrationStage
		fn    func(*MigrationOperation) error
	}{
		{StagePreflight, StageBackup, StagePreflight, r.runPreflight},
		{StageBackup, StageSchema, StageBackup, r.runBackup},
		{StageSchema, StageBackfill, StageSchema, r.runSchema},
		{StageBackfill, StageVerifying, StageBackfill, r.runBackfill},
		{StageVerifying, StageVerifying, StageVerifying, r.runVerifying},
	}

	for i := 0; i < len(stages); i++ {
		op, err = r.repo.GetOperation(ctx, operationID)
		if err != nil || op == nil {
			return
		}
		if op.Stage == StageFailedRetryable || op.Stage == StageFailedTerminal || op.Stage == StageManualReview {
			return
		}
		expected := stages[i].from
		if op.Stage != expected {
			for j := i + 1; j < len(stages); j++ {
				if op.Stage == stages[j].from {
					i = j - 1
					break
				}
			}
			op.Updated()
			r.repo.UpdateOperationCheckpoint(ctx, op)
			continue
		}
		op.Updated()
		r.repo.UpdateOperationCheckpoint(ctx, op)
		stepErr := stages[i].fn(op)
		if stepErr == nil {
			casOk, casErr := r.repo.UpdateOperationStageCAS(ctx, operationID, expected, stages[i].to, nil)
			if casErr != nil || !casOk {
				stepErr = &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: fmt.Sprintf("阶段 %s 转换到 %s 失败", expected, stages[i].to)}
				if casErr != nil {
					stepErr = &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: casErr.Error()}
				}
			}
		}
		if stepErr != nil {
			var me *RunnerError
			if errors.As(stepErr, &me) && me.Code == "MANUAL_REVIEW_REQUIRED" {
				r.repo.UpdateOperationStageCAS(ctx, operationID, expected, StageManualReview, func(o *MigrationOperation) {
					o.Error = stepErr.Error()
				})
			} else {
				r.repo.UpdateOperationStageCAS(ctx, operationID, expected, StageFailedRetryable, func(o *MigrationOperation) {
					o.Error = stepErr.Error()
				})
			}
			return
		}
	}
}

func (r *Runner) runPreflight(op *MigrationOperation) error {
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
	plan := r.plans[op.PlanID]
	if !plan.BackupRequired || r.backup == nil {
		return nil
	}
	backupID, err := r.backup.CreateBackup(context.Background())
	if err != nil {
		return &RunnerError{Code: "BACKUP_FAILED", Message: "备份失败: " + err.Error()}
	}
	op.BackupID = backupID
	op.Updated()
	if err := r.repo.UpdateOperationCheckpoint(context.Background(), op); err != nil {
		return &RunnerError{Code: "BACKUP_ID_PERSIST_FAILED", Message: "备份 ID 持久化失败: " + err.Error()}
	}
	return nil
}

func (r *Runner) runSchema(op *MigrationOperation) error {
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
	if err := r.repo.RecordReadCutover(ctx, op.ID, op.PlanID); err != nil {
		return &RunnerError{Code: "READ_CUTOVER_RECORD_FAILED", Message: "记录读切换失败: " + err.Error()}
	}
	if err := r.repo.MarkReadCutoverVerified(ctx, op.ID); err != nil {
		return &RunnerError{Code: "READ_CUTOVER_VERIFY_FAILED", Message: "标记读切换已验证失败: " + err.Error()}
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

	if err := r.repo.RecordWriteCutover(ctx, op.ID, op.PlanID); err != nil {
		return &RunnerError{Code: "WRITE_CUTOVER_RECORD_FAILED", Message: "记录写切换失败: " + err.Error()}
	}
	if err := r.repo.MarkWriteCutoverVerified(ctx, op.ID); err != nil {
		return &RunnerError{Code: "WRITE_CUTOVER_VERIFY_FAILED", Message: "标记写切换已验证失败: " + err.Error()}
	}
	return nil
}

func (r *Runner) runLegacyWriteBlock(op *MigrationOperation) error {
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

	if plan.BackupRequired && op.BackupID == "" {
		return &RunnerError{Code: "BACKUP_MISSING", Message: "备份缺失，不允许切转"}
	}
	if plan.BackupRequired && r.backup != nil {
		exists, err := r.backup.BackupExists(ctx, op.BackupID)
		if err != nil {
			return &RunnerError{Code: "BACKUP_VERIFY_FAILED", Message: "备份校验失败: " + err.Error()}
		}
		if !exists {
			return &RunnerError{Code: "BACKUP_NOT_FOUND", Message: "备份记录不存在: " + op.BackupID}
		}
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
		if _, err := r.repo.UpdateOperationStageCAS(ctx, operationID, MigrationStage(op.Stage), StageReadCutover, nil); err != nil {
			return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "读切转阶段转换失败: " + err.Error()}
		}
		if err := r.repo.RecordReadCutover(ctx, operationID, op.PlanID); err != nil {
			return &RunnerError{Code: "READ_CUTOVER_RECORD_FAILED", Message: "记录读切转失败: " + err.Error()}
		}
		for _, step := range plan.CutoverSteps {
			if err := step(); err != nil {
				return &RunnerError{Code: "READ_CUTOVER_STEP_FAILED", Message: "读切转步骤执行失败: " + err.Error()}
			}
		}
		if err := r.repo.MarkReadCutoverVerified(ctx, operationID); err != nil {
			return &RunnerError{Code: "READ_CUTOVER_VERIFY_FAILED", Message: "标记读切转已验证失败: " + err.Error()}
		}
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
		if _, err := r.repo.UpdateOperationStageCAS(ctx, operationID, MigrationStage(op.Stage), StageWriteCutover, nil); err != nil {
			return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "写切转阶段转换失败: " + err.Error()}
		}
		if err := r.repo.RecordWriteCutover(ctx, operationID, op.PlanID); err != nil {
			return &RunnerError{Code: "WRITE_CUTOVER_RECORD_FAILED", Message: "记录写切转失败: " + err.Error()}
		}
		for _, step := range plan.LegacyWriteBlockSteps {
			if err := step(); err != nil {
				return &RunnerError{Code: "LEGACY_BLOCK_FAILED", Message: "旧写阻断失败: " + err.Error()}
			}
		}
		if err := r.repo.MarkWriteCutoverVerified(ctx, operationID); err != nil {
			return &RunnerError{Code: "WRITE_CUTOVER_VERIFY_FAILED", Message: "标记写切转已验证失败: " + err.Error()}
		}
		if _, err := r.repo.UpdateOperationStageCAS(ctx, operationID, StageWriteCutover, StageCompleted, nil); err != nil {
			return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "完成阶段转换失败: " + err.Error()}
		}
	}
	return nil
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
