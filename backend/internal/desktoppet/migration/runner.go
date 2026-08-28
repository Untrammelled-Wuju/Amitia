// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	repo               *DBRepository
	lock               *migrationcore.PersistentLock
	lockName           string
	backup             BackupPort
	plans              map[string]DomainMigrationOperationPlan
	leaseTTL           time.Duration
	backupDir          string
	leaseLost          chan struct{}
	legacyWriteRefresh func() error
}

func NewRunner(repo *DBRepository) *Runner {
	return &Runner{
		repo:      repo,
		plans:     make(map[string]DomainMigrationOperationPlan),
		leaseTTL:  5 * time.Minute,
		lockName:  "desktop_pet_migration",
		leaseLost: make(chan struct{}),
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

func (r *Runner) SetLegacyWriteRefresh(refresh func() error) {
	r.legacyWriteRefresh = refresh
}

func (r *Runner) heartbeatInterval() time.Duration {
	interval := r.leaseTTL / 3
	if interval <= 0 {
		interval = time.Second
	}
	if r.leaseTTL >= 15*time.Second && interval < 5*time.Second {
		interval = 5 * time.Second
	}
	return interval
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
	ReadCutoverSteps      []StepFunc
	ReadCutoverChecks     []CheckFunc
	WriteCutoverSteps     []StepFunc
	WriteCutoverChecks    []CheckFunc
	LegacyWriteBlockSteps []StepFunc
	ParityChecks          []ParityCheck
}

type ParityCheck struct {
	Name     string
	Required bool
	Check    func(ctx context.Context) (passed bool, detail string, err error)
}

const batchSize = 500

func (r *Runner) checkLeaseLost(ctx context.Context, operationID string, op *MigrationOperation) bool {
	select {
	case <-r.leaseLost:
		ok, err := r.repo.UpdateOperationStageCAS(ctx, operationID, MigrationStage(op.Stage), StageFailedTerminal, func(o *MigrationOperation) {
			o.Error = "lease ownership lost, migration terminated"
		})
		if err != nil || !ok {
			log.Printf("desktop pet migration: persist lease-lost terminal state for %s failed: ok=%v err=%v", operationID, ok, err)
		}
		return true
	default:
		return false
	}
}

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
		err := r.lock.Acquire(lockCtx, r.lockName, r.leaseTTL)
		cancel()
		if err != nil {
			ok, persistErr := r.repo.UpdateOperationStageCAS(ctx, operationID, StagePreflight, StageFailedTerminal, func(o *MigrationOperation) {
				o.Error = "获取迁移锁失败: " + err.Error()
			})
			if persistErr != nil || !ok {
				log.Printf("desktop pet migration: persist lock acquisition failure for %s failed: ok=%v err=%v", operationID, ok, persistErr)
			}
			return
		}
		leaseLost := make(chan struct{})
		r.leaseLost = leaseLost
		r.lock.SetLeaseLostHandler(func(string) {
			select {
			case <-leaseLost:
			default:
				close(leaseLost)
			}
		})
		if err := r.lock.StartHeartbeat(r.lockName, r.heartbeatInterval(), r.leaseTTL); err != nil {
			ok, persistErr := r.repo.UpdateOperationStageCAS(ctx, operationID, StagePreflight, StageFailedTerminal, func(o *MigrationOperation) {
				o.Error = "启动心跳失败: " + err.Error()
			})
			if persistErr != nil || !ok {
				log.Printf("desktop pet migration: persist heartbeat startup failure for %s failed: ok=%v err=%v", operationID, ok, persistErr)
			}
			if releaseErr := r.lock.Release(r.lockName); releaseErr != nil {
				log.Printf("desktop pet migration: release lock after heartbeat startup failure: %v", releaseErr)
			}
			return
		}
		defer func() {
			if releaseErr := r.lock.Release(r.lockName); releaseErr != nil {
				log.Printf("desktop pet migration: release plan lock %s failed: %v", operationID, releaseErr)
			}
		}()
	} else {
		r.leaseLost = make(chan struct{})
	}

	defer func() {
		if rec := recover(); rec != nil {
			ok, persistErr := r.repo.UpdateOperationStageCAS(ctx, operationID, MigrationStage(op.Stage), StageFailedTerminal, func(o *MigrationOperation) {
				o.Error = fmt.Sprintf("panic: %v", rec)
			})
			if persistErr != nil || !ok {
				log.Printf("desktop pet migration: persist panic terminal state for %s failed: ok=%v err=%v", operationID, ok, persistErr)
			}
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
		if r.checkLeaseLost(ctx, operationID, op) {
			return
		}
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
			if checkpointErr := r.repo.UpdateOperationCheckpoint(ctx, op); checkpointErr != nil {
				log.Printf("desktop pet migration: update checkpoint for %s failed: %v", operationID, checkpointErr)
				return
			}
			continue
		}
		op.Updated()
		if checkpointErr := r.repo.UpdateOperationCheckpoint(ctx, op); checkpointErr != nil {
			stepErr := &RunnerError{Code: "CHECKPOINT_UPDATE_FAILED", Message: "更新迁移操作检查点失败: " + checkpointErr.Error()}
			ok, persistErr := r.repo.UpdateOperationStageCAS(ctx, operationID, expected, StageFailedRetryable, func(o *MigrationOperation) { o.Error = stepErr.Error() })
			if persistErr != nil || !ok {
				log.Printf("desktop pet migration: persist checkpoint failure for %s failed: ok=%v err=%v", operationID, ok, persistErr)
			}
			return
		}
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
			next := StageFailedRetryable
			var me *RunnerError
			if errors.As(stepErr, &me) && me.Code == "MANUAL_REVIEW_REQUIRED" {
				next = StageManualReview
			}
			ok, persistErr := r.repo.UpdateOperationStageCAS(ctx, operationID, expected, next, func(o *MigrationOperation) {
				o.Error = stepErr.Error()
			})
			if persistErr != nil || !ok {
				log.Printf("desktop pet migration: persist stage failure for %s failed: ok=%v err=%v original=%v", operationID, ok, persistErr, stepErr)
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
	if !plan.BackupRequired {
		return nil
	}
	if r.backup == nil {
		return &RunnerError{Code: "BACKUP_NOT_CONFIGURED", Message: "迁移计划要求备份，但 BackupPort 未配置"}
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
			if err := r.repo.SaveCheckpoint(ctx, op.ID, stepName, fmt.Sprintf("%d", offset), int(processedTotal), inputHash, "", int(conflictsTotal)); err != nil {
				return &RunnerError{Code: "CHECKPOINT_SAVE_FAILED", Message: "保存回填检查点失败: " + err.Error()}
			}

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
	return ComputeChecksum(fmt.Sprintf("pk=%d;batch=%d", in.PK, in.BatchIdx))
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

func (r *Runner) GetOperation(ctx context.Context, operationID string) (*MigrationOperation, error) {
	if r.repo == nil {
		return nil, &RunnerError{Code: "NOT_INITIALIZED", Message: "迁移仓库未初始化"}
	}
	return r.repo.GetOperation(ctx, operationID)
}

func (r *Runner) RequestCutover(ctx context.Context, operationID, direction string) (retErr error) {
	if operationID == "" {
		return &RunnerError{Code: "OPERATION_REQUIRED", Message: "operationId 不能为空"}
	}
	if direction != "read" && direction != "write" {
		return &RunnerError{Code: "INVALID_DIRECTION", Message: "切转方向无效: " + direction}
	}

	releaseLock, err := r.acquireCutoverLock(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if releaseErr := releaseLock(); releaseErr != nil {
			if retErr == nil {
				retErr = &RunnerError{Code: "MIGRATION_LOCK_RELEASE_FAILED", Message: "释放切转迁移锁失败: " + releaseErr.Error()}
			} else {
				retErr = errors.Join(retErr, fmt.Errorf("release cutover lock: %w", releaseErr))
			}
		}
	}()

	retErr = r.requestCutoverLocked(ctx, operationID, direction)
	return retErr
}

func (r *Runner) acquireCutoverLock(ctx context.Context) (func() error, error) {
	if r.lock == nil {
		return func() error { return nil }, nil
	}
	lockCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := r.lock.Acquire(lockCtx, r.lockName, r.leaseTTL); err != nil {
		return nil, &RunnerError{Code: "MIGRATION_LOCK_FAILED", Message: "获取切转迁移锁失败: " + err.Error()}
	}

	leaseLost := make(chan struct{})
	r.leaseLost = leaseLost
	r.lock.SetLeaseLostHandler(func(string) {
		select {
		case <-leaseLost:
		default:
			close(leaseLost)
		}
	})
	if err := r.lock.StartHeartbeat(r.lockName, r.heartbeatInterval(), r.leaseTTL); err != nil {
		releaseErr := r.lock.Release(r.lockName)
		if releaseErr != nil {
			return nil, &RunnerError{Code: "LOCK_HEARTBEAT_FAILED", Message: "启动切转锁心跳失败: " + errors.Join(err, releaseErr).Error()}
		}
		return nil, &RunnerError{Code: "LOCK_HEARTBEAT_FAILED", Message: "启动切转锁心跳失败: " + err.Error()}
	}
	return func() error { return r.lock.Release(r.lockName) }, nil
}

func (r *Runner) requestCutoverLocked(ctx context.Context, operationID, direction string) error {
	op, err := r.repo.GetOperation(ctx, operationID)
	if err != nil {
		return &RunnerError{Code: "OPERATION_GET_FAILED", Message: "获取迁移操作失败: " + err.Error()}
	}
	if op == nil {
		return &RunnerError{Code: "OPERATION_NOT_FOUND", Message: "迁移操作不存在"}
	}
	if r.checkLeaseLost(ctx, operationID, op) {
		return &RunnerError{Code: "LEASE_LOST", Message: "迁移锁所有权已丢失"}
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
	if plan.BackupRequired {
		if op.BackupID == "" || r.backup == nil {
			return &RunnerError{Code: "BACKUP_MISSING", Message: "备份缺失，不允许切转"}
		}
		exists, verifyErr := r.backup.BackupExists(ctx, op.BackupID)
		if verifyErr != nil {
			return &RunnerError{Code: "BACKUP_VERIFY_FAILED", Message: "备份校验失败: " + verifyErr.Error()}
		}
		if !exists {
			return &RunnerError{Code: "BACKUP_NOT_FOUND", Message: "备份记录不存在: " + op.BackupID}
		}
	}

	if direction == "read" {
		return r.requestReadCutover(ctx, op, plan)
	}
	return r.requestWriteCutover(ctx, op, plan)
}

func (r *Runner) runRequiredParity(ctx context.Context, plan DomainMigrationOperationPlan) error {
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
	return nil
}

func (r *Runner) requestReadCutover(ctx context.Context, op *MigrationOperation, plan DomainMigrationOperationPlan) error {
	if op.Stage == StageReadCutover {
		verified, err := r.repo.HasVerifiedReadCutover(ctx, op.ID)
		if err != nil {
			return &RunnerError{Code: "READ_CUTOVER_CHECK_FAILED", Message: err.Error()}
		}
		if verified {
			return nil
		}
	}
	if op.Stage != StageVerifying {
		return &RunnerError{Code: "INVALID_STAGE", Message: fmt.Sprintf("当前阶段 %s 不允许读切转", op.Stage)}
	}
	if err := r.runRequiredParity(ctx, plan); err != nil {
		return err
	}
	for _, step := range plan.ReadCutoverSteps {
		if r.checkLeaseLost(ctx, op.ID, op) {
			return &RunnerError{Code: "LEASE_LOST", Message: "lease lost during read cutover"}
		}
		if err := step(); err != nil {
			return &RunnerError{Code: "READ_CUTOVER_STEP_FAILED", Message: "读切转步骤执行失败: " + err.Error()}
		}
	}
	for _, check := range plan.ReadCutoverChecks {
		passed, detail := check()
		if !passed {
			return &RunnerError{Code: "READ_CUTOVER_VERIFY_FAILED", Message: "读切转验证失败: " + detail}
		}
	}
	const readStep = "v2_read_path"
	if err := r.repo.RecordReadCutover(ctx, op.ID, readStep); err != nil {
		return &RunnerError{Code: "READ_CUTOVER_RECORD_FAILED", Message: "记录读切转失败: " + err.Error()}
	}
	if err := r.repo.MarkReadCutoverVerified(ctx, op.ID, readStep); err != nil {
		return &RunnerError{Code: "READ_CUTOVER_VERIFY_FAILED", Message: "标记读切转已验证失败: " + err.Error()}
	}
	ok, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageVerifying, StageReadCutover, nil)
	if err != nil || !ok {
		if err == nil {
			err = errors.New("stage CAS returned false")
		}
		return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "读切转阶段转换失败: " + err.Error()}
	}
	return nil
}

func (r *Runner) requestWriteCutover(ctx context.Context, op *MigrationOperation, plan DomainMigrationOperationPlan) error {
	if op.Stage == StageCompleted {
		verified, err := r.repo.HasVerifiedWriteCutover(ctx, op.ID)
		if err == nil && verified {
			return nil
		}
	}
	if op.Stage != StageReadCutover && op.Stage != StageWriteCutover && op.Stage != StageLegacyWriteBlocked {
		return &RunnerError{Code: "INVALID_STAGE", Message: fmt.Sprintf("当前阶段 %s 不允许写切转", op.Stage)}
	}
	readVerified, err := r.repo.HasVerifiedReadCutover(ctx, op.ID)
	if err != nil {
		return &RunnerError{Code: "READ_CUTOVER_CHECK_FAILED", Message: "检查读切转验证状态失败: " + err.Error()}
	}
	if !readVerified {
		return &RunnerError{Code: "READ_CUTOVER_NOT_VERIFIED", Message: "读切转未验证，不允许写切转"}
	}
	if err := r.runRequiredParity(ctx, plan); err != nil {
		return err
	}
	if op.Stage == StageReadCutover {
		for _, step := range plan.WriteCutoverSteps {
			if r.checkLeaseLost(ctx, op.ID, op) {
				return &RunnerError{Code: "LEASE_LOST", Message: "lease lost during write cutover"}
			}
			if err := step(); err != nil {
				return &RunnerError{Code: "WRITE_CUTOVER_STEP_FAILED", Message: "写切转步骤执行失败: " + err.Error()}
			}
		}
		for _, check := range plan.WriteCutoverChecks {
			passed, detail := check()
			if !passed {
				return &RunnerError{Code: "WRITE_CUTOVER_VERIFY_FAILED", Message: "写切转验证失败: " + detail}
			}
		}
		for _, stepName := range []string{"installation", "editing"} {
			if err := r.repo.RecordWriteCutover(ctx, op.ID, stepName); err != nil {
				return &RunnerError{Code: "WRITE_CUTOVER_RECORD_FAILED", Message: "记录写切转失败: " + err.Error()}
			}
			if err := r.repo.MarkWriteCutoverVerified(ctx, op.ID, stepName); err != nil {
				return &RunnerError{Code: "WRITE_CUTOVER_VERIFY_FAILED", Message: "标记写切转已验证失败: " + err.Error()}
			}
		}
		ok, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageReadCutover, StageWriteCutover, nil)
		if err != nil || !ok {
			if err == nil {
				err = errors.New("stage CAS returned false")
			}
			return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "写切转阶段转换失败: " + err.Error()}
		}
		op.Stage = StageWriteCutover
	}

	if op.Stage == StageWriteCutover {
		writeVerified, err := r.repo.HasVerifiedWriteCutover(ctx, op.ID)
		if err != nil {
			return &RunnerError{Code: "WRITE_CUTOVER_CHECK_FAILED", Message: "检查写切转验证状态失败: " + err.Error()}
		}
		if !writeVerified {
			return &RunnerError{Code: "WRITE_CUTOVER_NOT_VERIFIED", Message: "installation/editing 写切转未全部验证，禁止阻断旧写"}
		}
		for _, step := range plan.LegacyWriteBlockSteps {
			if r.checkLeaseLost(ctx, op.ID, op) {
				return &RunnerError{Code: "LEASE_LOST", Message: "lease lost during legacy write block"}
			}
			if err := step(); err != nil {
				return &RunnerError{Code: "LEGACY_BLOCK_FAILED", Message: "旧写阻断失败: " + err.Error()}
			}
		}
		ok, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageWriteCutover, StageLegacyWriteBlocked, nil)
		if err != nil || !ok {
			if err == nil {
				err = errors.New("stage CAS returned false")
			}
			return &RunnerError{Code: "STAGE_TRANSITION_FAILED", Message: "旧写阻断阶段转换失败: " + err.Error()}
		}
		op.Stage = StageLegacyWriteBlocked
	}
	if op.Stage == StageLegacyWriteBlocked {
		if r.legacyWriteRefresh == nil {
			return &RunnerError{Code: "LEGACY_BLOCK_FAILED", Message: "旧写阻断状态刷新器未配置"}
		}
		if err := r.legacyWriteRefresh(); err != nil {
			return &RunnerError{Code: "LEGACY_BLOCK_FAILED", Message: "刷新旧写阻断状态失败: " + err.Error()}
		}
		ok, err := r.repo.UpdateOperationStageCAS(ctx, op.ID, StageLegacyWriteBlocked, StageCompleted, nil)
		if err != nil || !ok {
			if err == nil {
				err = errors.New("stage CAS returned false")
			}
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
