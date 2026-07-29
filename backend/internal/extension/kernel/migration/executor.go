package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type MigrationHandler func(ctx context.Context, step MigrationPathStep, def *MigrationDefinition, checkpoint *MigrationCheckpoint) (json.RawMessage, error)

type MigrationExecutor struct {
	mu         sync.Mutex
	planner    *MigrationPlanner
	snapshot   *SnapshotManager
	checkpoint *CheckpointManager
	validator  *PreconditionValidator
	repo       *MigrationRepository
	resolver   *MigrationGraphResolver
	inProgress map[string]*MigrationOperation
}

func NewMigrationExecutor(
	planner *MigrationPlanner,
	snapshot *SnapshotManager,
	checkpoint *CheckpointManager,
	validator *PreconditionValidator,
	repo *MigrationRepository,
	resolver *MigrationGraphResolver,
) *MigrationExecutor {
	return &MigrationExecutor{
		planner:    planner,
		snapshot:   snapshot,
		checkpoint: checkpoint,
		validator:  validator,
		repo:       repo,
		resolver:   resolver,
		inProgress: make(map[string]*MigrationOperation),
	}
}

func (e *MigrationExecutor) PlanAndExecute(ctx context.Context, input MigrationPlanInput, handler MigrationHandler) (*MigrationOperation, error) {
	if e == nil {
		return nil, fmt.Errorf("migration: executor is nil")
	}
	if input.FromVersion == "" || input.ToVersion == "" {
		return nil, fmt.Errorf("migration: from_version and to_version are required")
	}
	if input.FromVersion == input.ToVersion {
		return nil, fmt.Errorf("migration: from_version and to_version are the same")
	}
	if handler == nil {
		return nil, fmt.Errorf("migration: handler is required")
	}

	planOutput, err := e.planner.PlanMigration(input)
	if err != nil {
		return nil, fmt.Errorf("migration: plan failed: %w", err)
	}

	if planOutput.RequiresUserConfirm {
		return nil, fmt.Errorf("migration: requires user confirmation (risk=%s, irreversible=%v)", planOutput.EstimatedRisk, planOutput.HasIrreversible)
	}

	operationID := fmt.Sprintf("mop-%s", uuid.New().String())
	op := &MigrationOperation{
		OperationID:         operationID,
		ExtensionID:         input.ExtensionID,
		FromVersion:         input.FromVersion,
		ToVersion:           input.ToVersion,
		FromDefinitionHash:  input.FromDefinitionHash,
		ToDefinitionHash:    input.ToDefinitionHash,
		MigrationPath:       planOutput.Path,
		Status:              OperationStatusCreated,
		CurrentStep:         0,
		StartedAt:           time.Now().UTC(),
		Reversibility:       planOutput.Reversibility,
		RequiresUserConfirm: planOutput.RequiresUserConfirm,
		UserConfirmed:       !planOutput.RequiresUserConfirm,
	}

	if e.repo != nil {
		if err := e.repo.SaveMigrationOperation(ctx, op); err != nil {
			return nil, fmt.Errorf("migration: save operation: %w", err)
		}
	}

	e.mu.Lock()
	e.inProgress[operationID] = op
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.inProgress, operationID)
		e.mu.Unlock()
	}()

	if err := e.executeSteps(ctx, op, planOutput, input, handler); err != nil {
		e.failOperation(ctx, op, err)
		return op, err
	}

	finished := time.Now().UTC()
	op.FinishedAt = &finished
	op.Status = OperationStatusCompleted
	if e.repo != nil {
		e.repo.SaveMigrationOperation(ctx, op)
		e.repo.UpdateOperationStatus(ctx, operationID, OperationStatusCompleted, "", "")
	}
	return op, nil
}

func (e *MigrationExecutor) executeSteps(ctx context.Context, op *MigrationOperation, plan *MigrationPlanOutput, input MigrationPlanInput, handler MigrationHandler) error {
	defMap := make(map[string]MigrationDefinition, len(input.AvailableMigrations))
	for _, m := range input.AvailableMigrations {
		defMap[m.MigrationID] = m
	}

	if plan.HasIrreversible || plan.Reversibility == ReversibilitySnapshotReversible || plan.Reversibility == ReversibilityReverseScriptRequired {
		op.Status = OperationStatusSnapshotting
		if e.repo != nil {
			e.repo.UpdateOperationStatus(ctx, op.OperationID, OperationStatusSnapshotting, "", "")
		}
		if err := e.createSnapshotForMigration(ctx, op, input); err != nil {
			return fmt.Errorf("snapshot phase failed: %w", err)
		}
	}

	op.Status = OperationStatusMigrating
	if e.repo != nil {
		e.repo.UpdateOperationStatus(ctx, op.OperationID, OperationStatusMigrating, "", "")
	}

	for i, step := range op.MigrationPath.Steps {
		select {
		case <-ctx.Done():
			return fmt.Errorf("migration: cancelled at step %d: %w", i+1, ctx.Err())
		default:
		}

		op.CurrentStep = i
		if e.repo != nil {
			e.repo.SaveMigrationOperation(ctx, op)
		}

		def, ok := defMap[step.MigrationID]
		if !ok {
			return fmt.Errorf("migration: definition %s not found for step %d", step.MigrationID, i+1)
		}

		validResult, err := e.validator.ValidateMigrationDefinition(&def)
		if err != nil {
			return fmt.Errorf("migration: validate definition %s: %w", step.MigrationID, err)
		}
		if !validResult.Passed {
			return fmt.Errorf("migration: definition %s validation failed: %v", step.MigrationID, validResult.Errors)
		}

		if len(def.Precondition) > 0 {
			op.Status = OperationStatusValidating
			if e.repo != nil {
				e.repo.UpdateOperationStatus(ctx, op.OperationID, OperationStatusValidating, "", "")
			}
			preResult, err := e.validator.ValidatePreconditions(ctx, def.Precondition)
			if err != nil {
				return fmt.Errorf("migration: validate preconditions for %s: %w", step.MigrationID, err)
			}
			if !preResult.Passed {
				return fmt.Errorf("migration: preconditions failed for %s: %v", step.MigrationID, preResult.Errors)
			}
		}

		var checkpoint *MigrationCheckpoint
		if e.checkpoint != nil {
			if def.Idempotency == IdempotencyCheckpointIdempotent || def.Idempotency == IdempotencyNonIdempotent {
				existing, _ := e.checkpoint.GetLatestCheckpoint(ctx, op.OperationID, i+1)
				if existing != nil {
					checkpoint = existing
				} else {
					initialCursor, _ := json.Marshal(map[string]any{"offset": 0, "batch": 0})
					checkpoint, err = e.checkpoint.CreateCheckpoint(
						ctx, op.OperationID, i+1, step.MigrationID, "start",
						initialCursor, 0, 0,
						input.FromDefinitionHash, def.DefinitionHash, op.SnapshotID,
					)
					if err != nil {
						return fmt.Errorf("migration: create checkpoint for %s: %w", step.MigrationID, err)
					}
					op.CheckpointID = checkpoint.CheckpointID
					if e.repo != nil {
						e.repo.SaveMigrationOperation(ctx, op)
					}
				}
			}
		}

		stepRecord := MigrationStepRecord{
			StepID:      i + 1,
			OperationID: op.OperationID,
			MigrationID: step.MigrationID,
			Status:      "running",
			InputHash:   input.FromDefinitionHash,
			StartedAt:   time.Now().UTC(),
		}
		if e.repo != nil {
			e.repo.SaveMigrationStep(ctx, &stepRecord)
		}

		op.Status = OperationStatusMigrating
		if e.repo != nil {
			e.repo.UpdateOperationStatus(ctx, op.OperationID, OperationStatusMigrating, "", "")
		}

		output, err := handler(ctx, step, &def, checkpoint)
		finished := time.Now().UTC()
		stepRecord.FinishedAt = &finished

		if err != nil {
			stepRecord.Status = "failed"
			stepRecord.ErrorCode = "handler_error"
			stepRecord.ErrorMessage = err.Error()
			if e.repo != nil {
				e.repo.SaveMigrationStep(ctx, &stepRecord)
			}
			return fmt.Errorf("migration: step %d (%s) handler failed: %w", i+1, step.MigrationID, err)
		}

		if len(output) > 0 {
			stepRecord.OutputHash = fmt.Sprintf("sha256:%x", output[:min(len(output), 32)])
		}
		stepRecord.Status = "succeeded"
		if checkpoint != nil {
			stepRecord.CheckpointID = checkpoint.CheckpointID
		}
		if e.repo != nil {
			e.repo.SaveMigrationStep(ctx, &stepRecord)
		}

		if len(def.Postcondition) > 0 {
			postResult, err := e.validator.ValidatePostconditions(ctx, def.Postcondition)
			if err != nil {
				return fmt.Errorf("migration: validate postconditions for %s: %w", step.MigrationID, err)
			}
			if !postResult.Passed {
				return fmt.Errorf("migration: postconditions failed for %s: %v", step.MigrationID, postResult.Errors)
			}
		}
	}

	return nil
}

func (e *MigrationExecutor) createSnapshotForMigration(ctx context.Context, op *MigrationOperation, input MigrationPlanInput) error {
	if e.snapshot == nil {
		return nil
	}
	var entries []SnapshotEntry
	for _, m := range input.AvailableMigrations {
		for _, dd := range m.DataDomains {
			if dd.Storage == "" {
				continue
			}
			entryType := SnapshotEntryFile
			if dd.Domain == "sqlite" || dd.Domain == "schema" {
				entryType = SnapshotEntrySQLite
			} else if dd.Domain == "data" || dd.Domain == "resource_index" {
				entryType = SnapshotEntryDirectory
			}
			entries = append(entries, SnapshotEntry{
				Type:       entryType,
				SourcePath: dd.Storage,
			})
		}
	}
	if len(entries) == 0 {
		return nil
	}
	manifest, err := e.snapshot.CreateSnapshot(ctx, op.ExtensionID, op.OperationID, 0, entries)
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	op.SnapshotID = manifest.SnapshotID
	if e.repo != nil {
		e.repo.SaveMigrationOperation(ctx, op)
	}
	return nil
}

func (e *MigrationExecutor) failOperation(ctx context.Context, op *MigrationOperation, cause error) {
	finished := time.Now().UTC()
	op.FinishedAt = &finished
	op.Status = OperationStatusFailed
	op.ErrorCode = "migration_failed"
	op.ErrorMessage = cause.Error()
	if e.repo != nil {
		e.repo.SaveMigrationOperation(ctx, op)
		e.repo.UpdateOperationStatus(ctx, op.OperationID, OperationStatusFailed, op.ErrorCode, op.ErrorMessage)
	}
}

func (e *MigrationExecutor) CancelMigration(ctx context.Context, operationID string) error {
	e.mu.Lock()
	op, ok := e.inProgress[operationID]
	e.mu.Unlock()

	if !ok {
		if e.repo == nil {
			return fmt.Errorf("migration: operation %s not found", operationID)
		}
		var err error
		op, err = e.repo.GetMigrationOperation(ctx, operationID)
		if err != nil {
			return fmt.Errorf("migration: get operation: %w", err)
		}
	}

	if op.Status == OperationStatusCompleted || op.Status == OperationStatusCancelled {
		return fmt.Errorf("migration: operation %s already %s", operationID, op.Status)
	}

	finished := time.Now().UTC()
	op.FinishedAt = &finished
	op.Status = OperationStatusCancelled
	op.ErrorCode = "cancelled"
	op.ErrorMessage = "migration cancelled by user"

	if e.repo != nil {
		e.repo.SaveMigrationOperation(ctx, op)
		e.repo.UpdateOperationStatus(ctx, operationID, OperationStatusCancelled, "cancelled", "migration cancelled by user")
	}
	return nil
}

func (e *MigrationExecutor) ResumeMigration(ctx context.Context, operationID string, handler MigrationHandler) (*MigrationOperation, error) {
	if e.repo == nil {
		return nil, fmt.Errorf("migration: repository not available for resume")
	}
	op, err := e.repo.GetMigrationOperation(ctx, operationID)
	if err != nil {
		return nil, fmt.Errorf("migration: get operation: %w", err)
	}
	if op.Status != OperationStatusFailed && op.Status != OperationStatusRecoveryRequired {
		return nil, fmt.Errorf("migration: operation %s status is %s, cannot resume", operationID, op.Status)
	}

	if e.checkpoint != nil {
		checkpoints, _ := e.checkpoint.ListCheckpoints(ctx, operationID)
		if len(checkpoints) > 0 {
			lastCp := checkpoints[len(checkpoints)-1]
			op.CurrentStep = lastCp.StepID - 1
		}
	}

	steps, _ := e.repo.ListMigrationSteps(ctx, operationID)
	completedStepCount := 0
	for _, s := range steps {
		if s.Status == "succeeded" {
			completedStepCount++
		}
	}

	op.Status = OperationStatusMigrating
	op.ErrorCode = ""
	op.ErrorMessage = ""
	e.repo.SaveMigrationOperation(ctx, op)
	e.repo.UpdateOperationStatus(ctx, operationID, OperationStatusMigrating, "", "")

	e.mu.Lock()
	e.inProgress[operationID] = op
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.inProgress, operationID)
		e.mu.Unlock()
	}()

	remainingSteps := op.MigrationPath.Steps[completedStepCount:]
	if len(remainingSteps) == 0 {
		finished := time.Now().UTC()
		op.FinishedAt = &finished
		op.Status = OperationStatusCompleted
		e.repo.SaveMigrationOperation(ctx, op)
		e.repo.UpdateOperationStatus(ctx, operationID, OperationStatusCompleted, "", "")
		return op, nil
	}

	defMap := make(map[string]MigrationDefinition)
	allDefs, _ := e.repo.ListMigrationDefinitions(ctx, op.ExtensionID)
	for _, d := range allDefs {
		defMap[d.MigrationID] = d
	}

	for i, step := range remainingSteps {
		select {
		case <-ctx.Done():
			e.failOperation(ctx, op, ctx.Err())
			return op, ctx.Err()
		default:
		}

		actualStepIdx := completedStepCount + i
		op.CurrentStep = actualStepIdx
		e.repo.SaveMigrationOperation(ctx, op)

		def, ok := defMap[step.MigrationID]
		if !ok {
			err := fmt.Errorf("migration: definition %s not found", step.MigrationID)
			e.failOperation(ctx, op, err)
			return op, err
		}

		var checkpoint *MigrationCheckpoint
		if e.checkpoint != nil {
			existing, _ := e.checkpoint.GetLatestCheckpoint(ctx, op.OperationID, actualStepIdx+1)
			if existing != nil {
				checkpoint = existing
			}
		}

		stepRecord := MigrationStepRecord{
			StepID:      actualStepIdx + 1,
			OperationID: op.OperationID,
			MigrationID: step.MigrationID,
			Status:      "running",
			InputHash:   op.FromDefinitionHash,
			StartedAt:   time.Now().UTC(),
		}
		e.repo.SaveMigrationStep(ctx, &stepRecord)

		output, err := handler(ctx, step, &def, checkpoint)
		finished := time.Now().UTC()
		stepRecord.FinishedAt = &finished

		if err != nil {
			stepRecord.Status = "failed"
			stepRecord.ErrorCode = "handler_error"
			stepRecord.ErrorMessage = err.Error()
			e.repo.SaveMigrationStep(ctx, &stepRecord)
			e.failOperation(ctx, op, fmt.Errorf("step %d (%s): %w", actualStepIdx+1, step.MigrationID, err))
			return op, fmt.Errorf("migration: step %d (%s) handler failed: %w", actualStepIdx+1, step.MigrationID, err)
		}

		if len(output) > 0 {
			stepRecord.OutputHash = fmt.Sprintf("sha256:%x", output[:min(len(output), 32)])
		}
		stepRecord.Status = "succeeded"
		if checkpoint != nil {
			stepRecord.CheckpointID = checkpoint.CheckpointID
		}
		e.repo.SaveMigrationStep(ctx, &stepRecord)
	}

	finished := time.Now().UTC()
	op.FinishedAt = &finished
	op.Status = OperationStatusCompleted
	e.repo.SaveMigrationOperation(ctx, op)
	e.repo.UpdateOperationStatus(ctx, operationID, OperationStatusCompleted, "", "")
	return op, nil
}

func (e *MigrationExecutor) GetMigrationStatus(ctx context.Context, operationID string) (*MigrationOperation, error) {
	e.mu.Lock()
	op, ok := e.inProgress[operationID]
	e.mu.Unlock()
	if ok {
		result := *op
		return &result, nil
	}
	if e.repo == nil {
		return nil, fmt.Errorf("migration: operation %s not found", operationID)
	}
	return e.repo.GetMigrationOperation(ctx, operationID)
}

func (e *MigrationExecutor) GetMigrationSteps(ctx context.Context, operationID string) ([]MigrationStepRecord, error) {
	if e.repo == nil {
		return nil, fmt.Errorf("migration: repository not available")
	}
	return e.repo.ListMigrationSteps(ctx, operationID)
}

func (e *MigrationExecutor) RestoreFromSnapshot(ctx context.Context, operationID string) error {
	if e.repo == nil {
		return fmt.Errorf("migration: repository not available")
	}
	op, err := e.repo.GetMigrationOperation(ctx, operationID)
	if err != nil {
		return fmt.Errorf("migration: get operation: %w", err)
	}
	if op.SnapshotID == "" {
		return fmt.Errorf("migration: operation %s has no snapshot", operationID)
	}
	if e.snapshot == nil {
		return fmt.Errorf("migration: snapshot manager not available")
	}
	if err := e.snapshot.RestoreSnapshot(ctx, op.SnapshotID); err != nil {
		return fmt.Errorf("migration: restore snapshot: %w", err)
	}
	return nil
}

func (e *MigrationExecutor) ListPendingOperations(ctx context.Context) ([]MigrationOperation, error) {
	if e.repo == nil {
		return nil, fmt.Errorf("migration: repository not available")
	}
	ops, err := e.repo.ListMigrationOperations(ctx, "")
	if err != nil {
		return nil, err
	}
	var pending []MigrationOperation
	for _, op := range ops {
		if op.Status == OperationStatusCreated || op.Status == OperationStatusSnapshotting ||
			op.Status == OperationStatusMigrating || op.Status == OperationStatusValidating ||
			op.Status == OperationStatusRecoveryRequired {
			pending = append(pending, op)
		}
	}
	return pending, nil
}

func (e *MigrationExecutor) VerifyOperationIntegrity(ctx context.Context, operationID string) (*ValidationResult, error) {
	if e.repo == nil {
		return nil, fmt.Errorf("migration: repository not available")
	}
	op, err := e.repo.GetMigrationOperation(ctx, operationID)
	if err != nil {
		return nil, fmt.Errorf("migration: get operation: %w", err)
	}
	result := &ValidationResult{
		Passed:   true,
		Errors:   []string{},
		Warnings: []string{},
	}

	if op.SnapshotID != "" && e.snapshot != nil {
		snapResult, err := e.snapshot.VerifySnapshot(ctx, op.SnapshotID)
		if err != nil {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("snapshot verify failed: %v", err))
		} else if !snapResult.Passed {
			result.Passed = false
			result.Errors = append(result.Errors, snapResult.Errors...)
		}
	}

	steps, err := e.repo.ListMigrationSteps(ctx, operationID)
	if err != nil {
		result.Passed = false
		result.Errors = append(result.Errors, fmt.Sprintf("list steps failed: %v", err))
	} else {
		expectedSteps := len(op.MigrationPath.Steps)
		succeededSteps := 0
		failedSteps := 0
		for _, s := range steps {
			switch s.Status {
			case "succeeded":
				succeededSteps++
			case "failed":
				failedSteps++
			}
		}
		if op.Status == OperationStatusCompleted && succeededSteps != expectedSteps {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("completed but only %d/%d steps succeeded", succeededSteps, expectedSteps))
		}
		if failedSteps > 0 && op.Status == OperationStatusCompleted {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("completed but %d steps failed", failedSteps))
		}
		if op.Status != OperationStatusCompleted && succeededSteps < expectedSteps {
			result.Warnings = append(result.Warnings, fmt.Sprintf("operation not completed: %d/%d steps succeeded", succeededSteps, expectedSteps))
		}
	}

	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
