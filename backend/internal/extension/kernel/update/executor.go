package update

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type UpdateExecutor struct {
	mu             sync.Mutex
	generations    *GenerationManager
	migrations     *MigrationExecutor
	points         *RollbackPointStore
	conflicts      *ConflictRegistry
	plans          map[string]UpdatePlan
	activeUpdates  map[string]string
}

func NewUpdateExecutor() *UpdateExecutor {
	return &UpdateExecutor{
		generations:   NewGenerationManager(),
		migrations:    NewMigrationExecutor(),
		points:        NewRollbackPointStore(),
		conflicts:     NewConflictRegistry(),
		plans:         make(map[string]UpdatePlan),
		activeUpdates: make(map[string]string),
	}
}

func (e *UpdateExecutor) Generations() *GenerationManager { return e.generations }
func (e *UpdateExecutor) Migrations() *MigrationExecutor { return e.migrations }
func (e *UpdateExecutor) Points() *RollbackPointStore { return e.points }
func (e *UpdateExecutor) Conflicts() *ConflictRegistry { return e.conflicts }

func (e *UpdateExecutor) CreatePlan(ctx context.Context, planID string, old, new DefinitionSnapshot, migrations []MigrationSnapshot) UpdatePlan {
	plan := BuildPlan(planID, old, new, migrations)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.plans[planID] = plan
	return plan
}

func (e *UpdateExecutor) GetPlan(ctx context.Context, planID string) (*UpdatePlan, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	plan, ok := e.plans[planID]
	if !ok {
		return nil, fmt.Errorf("update: plan %s not found", planID)
	}
	result := plan
	return &result, nil
}

type ExecuteRequest struct {
	PlanID         string
	ExtensionID    string
	UserConfirmed  bool
	MigrationHandler func(ctx context.Context, plan MigrationPlan) (string, error)
	StorageData    map[string][]byte
}

type ExecuteResult struct {
	Success         bool
	PlanID          string
	NewGenerationID string
	RollbackPointID string
	Reason          string
	Steps           []ExecuteStep
	MigrationRuns   []MigrationRun
}

type ExecuteStep struct {
	Name   string
	Status string
	Error  string
}

func (e *UpdateExecutor) Execute(ctx context.Context, req ExecuteRequest) ExecuteResult {
	e.mu.Lock()
	plan, ok := e.plans[req.PlanID]
	if !ok {
		e.mu.Unlock()
		return ExecuteResult{PlanID: req.PlanID, Reason: "plan not found"}
	}
	if _, exists := e.activeUpdates[req.ExtensionID]; exists {
		e.mu.Unlock()
		return ExecuteResult{PlanID: req.PlanID, Reason: "update already in progress"}
	}
	e.activeUpdates[req.ExtensionID] = req.PlanID
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.activeUpdates, req.ExtensionID)
		e.mu.Unlock()
	}()

	result := ExecuteResult{PlanID: req.PlanID}

	if plan.RequiresUserConfirm && !req.UserConfirmed {
		result.Reason = "user confirmation required"
		return result
	}
	result.Steps = append(result.Steps, ExecuteStep{Name: "verify_confirmation", Status: "succeeded"})

	if e.conflicts.HasUnresolvedConflicts(req.ExtensionID) {
		result.Reason = "unresolved user asset conflicts"
		result.Steps = append(result.Steps, ExecuteStep{Name: "check_conflicts", Status: "failed", Error: "unresolved conflicts"})
		return result
	}
	result.Steps = append(result.Steps, ExecuteStep{Name: "check_conflicts", Status: "succeeded"})

	activeGen := e.generations.Active(ctx, req.ExtensionID)
	var rollbackPointID string
	if activeGen != nil {
		point := RollbackPoint{
			PointID:        fmt.Sprintf("rb-%s-%d", req.ExtensionID, time.Now().UnixNano()),
			ExtensionID:    req.ExtensionID,
			Version:        activeGen.Version,
			GenerationID:   activeGen.GenerationID,
			DefinitionHash: activeGen.DefinitionHash,
			CreatedAt:      time.Now().UTC(),
			ProtectionLevel: plan.RollbackLevel,
		}
		if plan.RollbackLevel == RollbackLevelDataSnapshotRequired && len(plan.Migrations) > 0 {
			var namespaces []string
			for _, m := range plan.Migrations {
				namespaces = append(namespaces, m.Namespaces...)
			}
			if req.StorageData != nil {
				snap := e.migrations.SnapshotNamespaces(ctx, req.ExtensionID, namespaces, req.StorageData)
				point.StorageSnapshotID = snap.SnapshotID
			}
		}
		if err := e.points.Save(point); err != nil {
			result.Reason = fmt.Sprintf("save rollback point failed: %v", err)
			result.Steps = append(result.Steps, ExecuteStep{Name: "save_rollback_point", Status: "failed", Error: err.Error()})
			return result
		}
		rollbackPointID = point.PointID
		result.Steps = append(result.Steps, ExecuteStep{Name: "save_rollback_point", Status: "succeeded"})
	}

	newGen := e.generations.Prepare(ctx, req.ExtensionID, plan.NewVersion, plan.Diff.NewPublisherID+plan.NewVersion)
	if err := e.generations.Transition(ctx, req.ExtensionID, newGen.GenerationID, GenerationStateValidated); err != nil {
		result.Reason = fmt.Sprintf("validate generation failed: %v", err)
		result.Steps = append(result.Steps, ExecuteStep{Name: "validate_generation", Status: "failed", Error: err.Error()})
		return result
	}
	result.Steps = append(result.Steps, ExecuteStep{Name: "validate_generation", Status: "succeeded"})

	if err := e.generations.Transition(ctx, req.ExtensionID, newGen.GenerationID, GenerationStateRuntimeReady); err != nil {
		result.Reason = fmt.Sprintf("runtime ready failed: %v", err)
		result.Steps = append(result.Steps, ExecuteStep{Name: "runtime_ready", Status: "failed", Error: err.Error()})
		return result
	}
	result.Steps = append(result.Steps, ExecuteStep{Name: "runtime_ready", Status: "succeeded"})

	if len(plan.Migrations) > 0 && req.MigrationHandler != nil {
		for _, mPlan := range plan.Migrations {
			if e.migrations.RequiresSnapshot(mPlan) && req.StorageData != nil {
				e.migrations.SnapshotNamespaces(ctx, req.ExtensionID, mPlan.Namespaces, req.StorageData)
			}
			run, err := e.migrations.ExecuteMigration(ctx, mPlan, req.ExtensionID, plan.OldVersion, plan.NewVersion, "", func(ctx context.Context) (string, error) {
				return req.MigrationHandler(ctx, mPlan)
			})
			result.MigrationRuns = append(result.MigrationRuns, run)
			if err != nil {
				result.Reason = fmt.Sprintf("migration %s failed: %v", mPlan.MigrationID, err)
				result.Steps = append(result.Steps, ExecuteStep{Name: "migration_" + mPlan.MigrationID, Status: "failed", Error: err.Error()})
				if rollbackPointID != "" {
					e.executeRollback(ctx, req.ExtensionID, rollbackPointID)
				}
				return result
			}
			result.Steps = append(result.Steps, ExecuteStep{Name: "migration_" + mPlan.MigrationID, Status: "succeeded"})
		}
	}

	if err := e.generations.Transition(ctx, req.ExtensionID, newGen.GenerationID, GenerationStateActive); err != nil {
		result.Reason = fmt.Sprintf("activate generation failed: %v", err)
		result.Steps = append(result.Steps, ExecuteStep{Name: "activate_generation", Status: "failed", Error: err.Error()})
		return result
	}
	result.Steps = append(result.Steps, ExecuteStep{Name: "activate_generation", Status: "succeeded"})

	if activeGen != nil {
		e.generations.Transition(ctx, req.ExtensionID, activeGen.GenerationID, GenerationStateDraining)
		e.generations.Transition(ctx, req.ExtensionID, activeGen.GenerationID, GenerationStateStopped)
		result.Steps = append(result.Steps, ExecuteStep{Name: "stop_old_generation", Status: "succeeded"})
	}

	result.Success = true
	result.NewGenerationID = newGen.GenerationID
	result.RollbackPointID = rollbackPointID
	return result
}

func (e *UpdateExecutor) executeRollback(ctx context.Context, extensionID, pointID string) {
	executor := NewRollbackExecutor(e.points, e.migrations, e.generations)
	executor.Execute(ctx, RollbackRequest{
		ExtensionID: extensionID,
		PointID:     pointID,
		Reason:      "automatic rollback after update failure",
	})
}

type RollbackExecutorAccessor struct {
	executor *UpdateExecutor
}

func NewRollbackExecutorAccessor(executor *UpdateExecutor) *RollbackExecutorAccessor {
	return &RollbackExecutorAccessor{executor: executor}
}

func (a *RollbackExecutorAccessor) Execute(ctx context.Context, req RollbackRequest) RollbackResult {
	executor := NewRollbackExecutor(a.executor.points, a.executor.migrations, a.executor.generations)
	return executor.Execute(ctx, req)
}

func (e *UpdateExecutor) ApplyRetentionPolicy(ctx context.Context, extensionID string, policy RetentionPolicy) []string {
	return e.points.ApplyRetentionPolicy(ctx, extensionID, policy)
}

var (
	ErrUpdateInProgress = errors.New("update: update already in progress")
	ErrPlanNotFound     = errors.New("update: plan not found")
)
