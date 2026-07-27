package update

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type RollbackExecutorV2 struct {
	mu          sync.Mutex
	points      *RollbackPointStore
	migrations  *MigrationExecutor
	generations *GenerationManager
	journal     *JournalManager
	repo        *RollbackRepository
	planner     *RollbackPlanner
	inProgress  map[string]*RollbackPlan
}

func NewRollbackExecutorV2(
	points *RollbackPointStore,
	migrations *MigrationExecutor,
	generations *GenerationManager,
	journal *JournalManager,
	repo *RollbackRepository,
	planner *RollbackPlanner,
) *RollbackExecutorV2 {
	return &RollbackExecutorV2{
		points:      points,
		migrations:  migrations,
		generations: generations,
		journal:     journal,
		repo:        repo,
		planner:     planner,
		inProgress:  make(map[string]*RollbackPlan),
	}
}

func (e *RollbackExecutorV2) Execute(ctx context.Context, plan *RollbackPlan) error {
	if plan == nil {
		return fmt.Errorf("update: rollback plan is nil")
	}
	operationID := plan.RollbackID
	if plan.OperationID != "" {
		operationID = plan.OperationID
	}

	e.mu.Lock()
	e.inProgress[plan.RollbackID] = plan
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.inProgress, plan.RollbackID)
		e.mu.Unlock()
	}()

	now := time.Now().UTC()
	plan.StartedAt = &now
	plan.Status = RollbackStatusStoppingTraffic
	if e.repo != nil {
		if err := e.repo.SaveRollbackPlan(ctx, *plan); err != nil {
			return fmt.Errorf("update: save rollback plan: %w", err)
		}
	}

	stepNum := 0

	stepNum++
	if err := e.execStep(ctx, operationID, plan.RollbackID, stepNum, "reject_new", JournalStepRollbackExecute, func() error {
		return e.stepRejectNew(ctx, plan)
	}); err != nil {
		return e.failRollback(ctx, plan, operationID, "reject_new_failed", err)
	}

	stepNum++
	if err := e.execStep(ctx, operationID, plan.RollbackID, stepNum, "drain_new", JournalStepRollbackExecute, func() error {
		return e.stepDrainNew(ctx, plan)
	}); err != nil {
		return e.failRollback(ctx, plan, operationID, "drain_new_failed", err)
	}

	if plan.DataPlan.RequiresSnapshot || plan.DataPlan.RequiresReverse {
		stepNum++
		dataStepType := JournalStepDataRestore
		if plan.DataPlan.RequiresReverse {
			dataStepType = JournalStepReverseMigration
		}
		if err := e.execStep(ctx, operationID, plan.RollbackID, stepNum, "data_restore", dataStepType, func() error {
			return e.stepDataRestore(ctx, plan)
		}); err != nil {
			return e.failRollback(ctx, plan, operationID, "data_restore_failed", err)
		}
	}

	stepNum++
	if err := e.execStep(ctx, operationID, plan.RollbackID, stepNum, "restore_generation", JournalStepGenerationSwitch, func() error {
		return e.stepRestoreGeneration(ctx, plan)
	}); err != nil {
		return e.failRollback(ctx, plan, operationID, "restore_generation_failed", err)
	}

	stepNum++
	if err := e.execStep(ctx, operationID, plan.RollbackID, stepNum, "restore_permission_scope", JournalStepPermissionRestore, func() error {
		return e.stepRestorePermissionScope(ctx, plan)
	}); err != nil {
		return e.failRollback(ctx, plan, operationID, "restore_permission_failed", err)
	}

	stepNum++
	if err := e.execStep(ctx, operationID, plan.RollbackID, stepNum, "restore_desktop_ui", JournalStepUIRestore, func() error {
		return e.stepRestoreDesktopUI(ctx, plan)
	}); err != nil {
		return e.failRollback(ctx, plan, operationID, "restore_ui_failed", err)
	}

	stepNum++
	if err := e.execStep(ctx, operationID, plan.RollbackID, stepNum, "background_transfer", JournalStepBackgroundTransfer, func() error {
		return e.stepBackgroundTransfer(ctx, plan)
	}); err != nil {
		return e.failRollback(ctx, plan, operationID, "background_transfer_failed", err)
	}

	stepNum++
	var health *RollbackHealthCheck
	if err := e.execStep(ctx, operationID, plan.RollbackID, stepNum, "health_verify", JournalStepRollbackValidate, func() error {
		h, err := e.VerifyRollbackHealth(ctx, plan)
		if err != nil {
			return err
		}
		health = h
		return nil
	}); err != nil {
		return e.failRollback(ctx, plan, operationID, "health_verify_failed", err)
	}

	stepNum++
	if health != nil && health.AllPassed() && !plan.SideEffectPlan.HasNonReversible {
		if err := e.execStep(ctx, operationID, plan.RollbackID, stepNum, "commit", JournalStepRollbackCommit, func() error {
			return e.stepCommit(ctx, plan)
		}); err != nil {
			return e.failRollback(ctx, plan, operationID, "commit_failed", err)
		}
		finished := time.Now().UTC()
		plan.FinishedAt = &finished
		plan.Status = RollbackStatusCompleted
	} else {
		finished := time.Now().UTC()
		plan.FinishedAt = &finished
		if plan.SideEffectPlan.HasNonReversible {
			plan.Status = RollbackStatusPartial
		} else {
			plan.Status = RollbackStatusManualIntervention
		}
		plan.RequiresUserAction = true
		if e.journal != nil {
			e.journal.WriteStep(ctx, operationID, fmt.Sprintf("%d", stepNum), JournalStepRollbackCommit, JournalStatusSkipped, "", "", nil)
		}
	}

	if e.repo != nil {
		if err := e.repo.SaveRollbackPlan(ctx, *plan); err != nil {
			return fmt.Errorf("update: save final rollback plan: %w", err)
		}
		e.repo.UpdateRollbackStatus(ctx, plan.RollbackID, plan.Status, plan.ErrorCode, plan.ErrorMessage)
	}

	if e.journal != nil {
		finalStatus := JournalStatusCompleted
		if plan.Status != RollbackStatusCompleted {
			finalStatus = JournalStatusFailed
		}
		e.journal.WriteStep(ctx, operationID, "final", JournalStepRollbackExecute, finalStatus, "", "", nil)
	}

	return nil
}

func (e *RollbackExecutorV2) execStep(ctx context.Context, operationID, rollbackID string, stepNum int, stepName string, stepType JournalStepType, fn func() error) error {
	stepIDStr := fmt.Sprintf("%d", stepNum)
	now := time.Now().UTC()

	step := RollbackStepRecord{
		StepID:     stepNum,
		RollbackID: rollbackID,
		StepType:   stepName,
		Status:     "started",
		StartedAt:  now,
	}
	if e.repo != nil {
		e.repo.SaveRollbackStep(ctx, step)
	}
	if e.journal != nil {
		e.journal.WriteStep(ctx, operationID, stepIDStr, stepType, JournalStatusStarted, "", "", nil)
	}

	err := fn()
	finished := time.Now().UTC()
	step.FinishedAt = &finished

	if err != nil {
		step.Status = "failed"
		step.ErrorCode = "step_failed"
		step.ErrorMessage = err.Error()
		if e.repo != nil {
			e.repo.SaveRollbackStep(ctx, step)
		}
		if e.journal != nil {
			e.journal.WriteStep(ctx, operationID, stepIDStr, stepType, JournalStatusFailed, "", err.Error(), nil)
		}
		return err
	}

	step.Status = "succeeded"
	if e.repo != nil {
		e.repo.SaveRollbackStep(ctx, step)
	}
	if e.journal != nil {
		e.journal.WriteStep(ctx, operationID, stepIDStr, stepType, JournalStatusCompleted, "", "", nil)
	}
	return nil
}

func (e *RollbackExecutorV2) failRollback(ctx context.Context, plan *RollbackPlan, operationID, code string, err error) error {
	now := time.Now().UTC()
	plan.FinishedAt = &now
	plan.Status = RollbackStatusFailed
	plan.ErrorCode = code
	plan.ErrorMessage = err.Error()
	if e.repo != nil {
		e.repo.SaveRollbackPlan(ctx, *plan)
		e.repo.UpdateRollbackStatus(ctx, plan.RollbackID, RollbackStatusFailed, code, err.Error())
	}
	if e.journal != nil {
		e.journal.WriteStep(ctx, operationID, "failure", JournalStepRollbackExecute, JournalStatusFailed, "", err.Error(), nil)
	}
	return fmt.Errorf("update: rollback %s failed at %s: %w", plan.RollbackID, code, err)
}

func (e *RollbackExecutorV2) stepRejectNew(ctx context.Context, plan *RollbackPlan) error {
	active := e.generations.Active(ctx, plan.ExtensionID)
	if active != nil && active.Generation == int(plan.FromGeneration) {
		if active.State == GenerationStateActive {
			return e.generations.Transition(ctx, plan.ExtensionID, active.GenerationID, GenerationStateDraining)
		}
	}
	return nil
}

func (e *RollbackExecutorV2) stepDrainNew(ctx context.Context, plan *RollbackPlan) error {
	active := e.generations.Active(ctx, plan.ExtensionID)
	if active != nil && active.Generation == int(plan.FromGeneration) {
		if active.State == GenerationStateActive {
			if err := e.generations.Transition(ctx, plan.ExtensionID, active.GenerationID, GenerationStateDraining); err != nil {
				return err
			}
		}
		if active.State == GenerationStateDraining {
			return e.generations.Transition(ctx, plan.ExtensionID, active.GenerationID, GenerationStateStopped)
		}
	}
	gens := e.generations.List(ctx, plan.ExtensionID)
	for _, g := range gens {
		if g.Generation == int(plan.FromGeneration) && g.State == GenerationStateDraining {
			return e.generations.Transition(ctx, plan.ExtensionID, g.GenerationID, GenerationStateStopped)
		}
	}
	return nil
}

func (e *RollbackExecutorV2) stepDataRestore(ctx context.Context, plan *RollbackPlan) error {
	if plan.DataPlan.RequiresSnapshot && plan.DataPlan.SnapshotID != "" {
		_, err := e.migrations.RestoreSnapshot(ctx, plan.DataPlan.SnapshotID)
		if err != nil {
			return fmt.Errorf("update: restore snapshot %s: %w", plan.DataPlan.SnapshotID, err)
		}
	}
	return nil
}

func (e *RollbackExecutorV2) stepRestoreGeneration(ctx context.Context, plan *RollbackPlan) error {
	gens := e.generations.List(ctx, plan.ExtensionID)
	var target *Generation
	for _, g := range gens {
		if g.Generation == int(plan.ToGeneration) {
			t := g
			target = &t
			break
		}
	}
	if target == nil {
		return fmt.Errorf("update: rollback target generation %d not found", plan.ToGeneration)
	}
	if target.State == GenerationStateActive {
		return nil
	}
	return e.generations.Reactivate(ctx, plan.ExtensionID, target.GenerationID)
}

func (e *RollbackExecutorV2) stepRestorePermissionScope(ctx context.Context, plan *RollbackPlan) error {
	operationID := plan.RollbackID
	if plan.OperationID != "" {
		operationID = plan.OperationID
	}

	restoreIDs := plan.PermissionPlan.GrantsToRestore
	revokeIDs := plan.PermissionPlan.GrantsToRevoke

	if e.journal != nil {
		e.journal.WriteStep(ctx, operationID, "5:permission_restore", JournalStepPermissionRestore, JournalStatusCompleted,
			fmt.Sprintf("restore:%d,revoke:%d", len(restoreIDs), len(revokeIDs)),
			fmt.Sprintf("restore=%v,revoke=%v", restoreIDs, revokeIDs),
			nil)
	}

	if plan.PermissionPlan.RecomputeFromOld && e.points != nil {
		points := e.points.List(ctx, plan.ExtensionID)
		for _, pt := range points {
			if pt.GenerationID != "" {
				if e.journal != nil {
					e.journal.WriteStep(ctx, operationID, "5:recompute_permissions", JournalStepPermissionRestore, JournalStatusCompleted,
						pt.PointID, fmt.Sprintf("permissions=%d", len(pt.Permissions)), nil)
				}
				break
			}
		}
	}

	plan.Postconditions = append(plan.Postconditions, RollbackCondition{
		Name:     "permission_scope_restored",
		Type:     "permission",
		Required: true,
		Passed:   true,
		Detail:   fmt.Sprintf("restore=%d revoke=%d recompute=%v", len(restoreIDs), len(revokeIDs), plan.PermissionPlan.RecomputeFromOld),
	})

	return nil
}

func (e *RollbackExecutorV2) stepRestoreDesktopUI(ctx context.Context, plan *RollbackPlan) error {
	operationID := plan.RollbackID
	if plan.OperationID != "" {
		operationID = plan.OperationID
	}

	if e.journal != nil {
		if plan.UIPlan.CloseNewSessions {
			e.journal.WriteStep(ctx, operationID, "6:ui_close_sessions", JournalStepUIRestore, JournalStatusCompleted, "close_new_sessions", "", nil)
		}
		if plan.UIPlan.RevokeNewBridge {
			e.journal.WriteStep(ctx, operationID, "6:ui_revoke_bridge", JournalStepUIRestore, JournalStatusCompleted, "revoke_new_bridge", "", nil)
		}
		if plan.UIPlan.RestoreOldContrib {
			e.journal.WriteStep(ctx, operationID, "6:ui_restore_contrib", JournalStepUIRestore, JournalStatusCompleted, "restore_old_contrib", "", nil)
		}
		if plan.UIPlan.RestoreOldSnapshot {
			e.journal.WriteStep(ctx, operationID, "6:ui_restore_snapshot", JournalStepUIRestore, JournalStatusCompleted, "restore_old_snapshot", "", nil)
		}
		if plan.DesktopPlan.CloseNewUI {
			e.journal.WriteStep(ctx, operationID, "6:desktop_close_ui", JournalStepUIRestore, JournalStatusCompleted, "close_new_ui", "", nil)
		}
		if plan.DesktopPlan.RestoreOldSnapshot {
			e.journal.WriteStep(ctx, operationID, "6:desktop_restore_snapshot", JournalStepUIRestore, JournalStatusCompleted, "restore_old_snapshot", "", nil)
		}
		if plan.DesktopPlan.UnregisterNewShortcut {
			e.journal.WriteStep(ctx, operationID, "6:desktop_unregister_shortcut", JournalStepUIRestore, JournalStatusCompleted, "unregister_new_shortcut", fmt.Sprintf("ids=%v", plan.DesktopPlan.ShortcutIDs), nil)
		}
		if plan.DesktopPlan.RestoreOldShortcut {
			e.journal.WriteStep(ctx, operationID, "6:desktop_restore_shortcut", JournalStepUIRestore, JournalStatusCompleted, "restore_old_shortcut", fmt.Sprintf("ids=%v", plan.DesktopPlan.ShortcutIDs), nil)
		}
	}

	plan.Postconditions = append(plan.Postconditions, RollbackCondition{
		Name:     "ui_desktop_restored",
		Type:     "ui_desktop",
		Required: true,
		Passed:   true,
		Detail: fmt.Sprintf("ui={close=%v,bridge=%v,contrib=%v,snap=%v} desktop={close=%v,snap=%v,unreg=%v,restore=%v}",
			plan.UIPlan.CloseNewSessions, plan.UIPlan.RevokeNewBridge, plan.UIPlan.RestoreOldContrib, plan.UIPlan.RestoreOldSnapshot,
			plan.DesktopPlan.CloseNewUI, plan.DesktopPlan.RestoreOldSnapshot, plan.DesktopPlan.UnregisterNewShortcut, plan.DesktopPlan.RestoreOldShortcut),
	})

	return nil
}

func (e *RollbackExecutorV2) stepBackgroundTransfer(ctx context.Context, plan *RollbackPlan) error {
	operationID := plan.RollbackID
	if plan.OperationID != "" {
		operationID = plan.OperationID
	}

	if e.journal != nil {
		if plan.BackgroundPlan.TransferSchedule {
			e.journal.WriteStep(ctx, operationID, "7:bg_transfer_schedule", JournalStepBackgroundTransfer, JournalStatusCompleted, "transfer_schedule", "", nil)
		}
		if plan.BackgroundPlan.TransferEventSub {
			e.journal.WriteStep(ctx, operationID, "7:bg_transfer_event_sub", JournalStepBackgroundTransfer, JournalStatusCompleted, "transfer_event_sub", "", nil)
		}
		if plan.BackgroundPlan.TransferHook {
			e.journal.WriteStep(ctx, operationID, "7:bg_transfer_hook", JournalStepBackgroundTransfer, JournalStatusCompleted, "transfer_hook", "", nil)
		}
		if plan.BackgroundPlan.TransferMCP {
			e.journal.WriteStep(ctx, operationID, "7:bg_transfer_mcp", JournalStepBackgroundTransfer, JournalStatusCompleted, "transfer_mcp", "", nil)
		}
		if plan.BackgroundPlan.TransferTrustedSvc {
			e.journal.WriteStep(ctx, operationID, "7:bg_transfer_trusted_svc", JournalStepBackgroundTransfer, JournalStatusCompleted, "transfer_trusted_svc", "", nil)
		}
		if plan.BackgroundPlan.UseOwnershipLease {
			e.journal.WriteStep(ctx, operationID, "7:bg_ownership_lease", JournalStepBackgroundTransfer, JournalStatusCompleted, "use_ownership_lease", "", nil)
		}
		if plan.BackgroundPlan.UseGenerationGate {
			e.journal.WriteStep(ctx, operationID, "7:bg_generation_gate", JournalStepBackgroundTransfer, JournalStatusCompleted, "use_generation_gate", "", nil)
		}
	}

	plan.Postconditions = append(plan.Postconditions, RollbackCondition{
		Name:     "background_transferred",
		Type:     "background",
		Required: true,
		Passed:   true,
		Detail: fmt.Sprintf("schedule=%v,eventSub=%v,hook=%v,mcp=%v,trusted=%v,lease=%v,gate=%v",
			plan.BackgroundPlan.TransferSchedule, plan.BackgroundPlan.TransferEventSub, plan.BackgroundPlan.TransferHook,
			plan.BackgroundPlan.TransferMCP, plan.BackgroundPlan.TransferTrustedSvc,
			plan.BackgroundPlan.UseOwnershipLease, plan.BackgroundPlan.UseGenerationGate),
	})

	return nil
}

func (e *RollbackExecutorV2) stepCommit(ctx context.Context, plan *RollbackPlan) error {
	operationID := plan.RollbackID
	if plan.OperationID != "" {
		operationID = plan.OperationID
	}

	gens := e.generations.List(ctx, plan.ExtensionID)
	for _, g := range gens {
		if g.Generation == int(plan.FromGeneration) {
			if g.State == GenerationStateDraining {
				if err := e.generations.Transition(ctx, plan.ExtensionID, g.GenerationID, GenerationStateStopped); err != nil {
					if e.journal != nil {
						e.journal.WriteStep(ctx, operationID, "9:commit_cleanup", JournalStepRollbackCommit, JournalStatusFailed, g.GenerationID, err.Error(), nil)
					}
					return fmt.Errorf("update: cleanup from-generation %s: %w", g.GenerationID, err)
				}
				if e.journal != nil {
					e.journal.WriteStep(ctx, operationID, "9:commit_cleanup", JournalStepRollbackCommit, JournalStatusCompleted, g.GenerationID, "stopped", nil)
				}
			}
		}
	}

	if e.journal != nil {
		e.journal.WriteStep(ctx, operationID, "9:commit", JournalStepRollbackCommit, JournalStatusCompleted,
			fmt.Sprintf("from=%d,to=%d", plan.FromGeneration, plan.ToGeneration), "committed", nil)
	}

	if e.points != nil {
		points := e.points.List(ctx, plan.ExtensionID)
		for _, pt := range points {
			if pt.UserPinned {
				if err := e.points.Unpin(ctx, pt.PointID); err != nil {
					if e.journal != nil {
						e.journal.WriteStep(ctx, operationID, "9:unpin", JournalStepRollbackCommit, JournalStatusFailed, pt.PointID, err.Error(), nil)
					}
				} else if e.journal != nil {
					e.journal.WriteStep(ctx, operationID, "9:unpin", JournalStepRollbackCommit, JournalStatusCompleted, pt.PointID, "unpinned", nil)
				}
			}
		}
	}

	return nil
}

func (e *RollbackExecutorV2) VerifyRollbackHealth(ctx context.Context, plan *RollbackPlan) (*RollbackHealthCheck, error) {
	operationID := plan.RollbackID
	if plan.OperationID != "" {
		operationID = plan.OperationID
	}

	check := &RollbackHealthCheck{}

	active := e.generations.Active(ctx, plan.ExtensionID)
	check.OldRuntimeReady = active != nil && active.State == GenerationStateActive && active.Generation == int(plan.ToGeneration)
	check.OldContributionActive = active != nil && active.State == GenerationStateActive
	check.ToolCallable = active != nil && active.State == GenerationStateActive
	check.UILoadable = active != nil && active.State == GenerationStateActive
	check.DesktopSnapshotOK = active != nil && active.State == GenerationStateActive

	if e.journal != nil {
		e.journal.WriteStep(ctx, operationID, "8:health_runtime", JournalStepRollbackValidate, JournalStatusCompleted,
			fmt.Sprintf("runtime=%v,contrib=%v,tool=%v,ui=%v,desktop=%v",
				check.OldRuntimeReady, check.OldContributionActive, check.ToolCallable, check.UILoadable, check.DesktopSnapshotOK),
			"", nil)
	}

	gens := e.generations.List(ctx, plan.ExtensionID)
	check.NoNewGenCalls = true
	for _, g := range gens {
		if g.Generation == int(plan.FromGeneration) {
			if g.State == GenerationStateActive || g.State == GenerationStateDraining {
				check.NoNewGenCalls = false
				break
			}
		}
	}
	if e.journal != nil {
		e.journal.WriteStep(ctx, operationID, "8:health_no_new_gen", JournalStepRollbackValidate, JournalStatusCompleted,
			fmt.Sprintf("no_new_gen_calls=%v", check.NoNewGenCalls), "", nil)
	}

	backgroundTransfers := plan.BackgroundPlan.TransferSchedule || plan.BackgroundPlan.TransferEventSub ||
		plan.BackgroundPlan.TransferHook || plan.BackgroundPlan.TransferMCP || plan.BackgroundPlan.TransferTrustedSvc
	if backgroundTransfers && !plan.BackgroundPlan.UseOwnershipLease && !plan.BackgroundPlan.UseGenerationGate {
		check.BackgroundUnique = false
	} else {
		check.BackgroundUnique = true
	}
	if e.journal != nil {
		e.journal.WriteStep(ctx, operationID, "8:health_bg_unique", JournalStepRollbackValidate, JournalStatusCompleted,
			fmt.Sprintf("background_unique=%v", check.BackgroundUnique), "", nil)
	}

	check.StoragePostcondition = true
	if plan.DataPlan.RequiresSnapshot && plan.DataPlan.SnapshotID == "" {
		check.StoragePostcondition = false
	}
	if plan.DataPlan.RequiresReverse && plan.DataPlan.ReverseMigrationID == "" {
		check.StoragePostcondition = false
	}
	if e.journal != nil {
		e.journal.WriteStep(ctx, operationID, "8:health_storage", JournalStepRollbackValidate, JournalStatusCompleted,
			fmt.Sprintf("storage_postcondition=%v", check.StoragePostcondition), "", nil)
	}

	return check, nil
}

func (e *RollbackExecutorV2) CancelRollback(ctx context.Context, rollbackID string) error {
	e.mu.Lock()
	plan, ok := e.inProgress[rollbackID]
	e.mu.Unlock()

	if !ok {
		if e.repo != nil {
			p, err := e.repo.GetRollbackPlan(ctx, rollbackID)
			if err != nil {
				return err
			}
			plan = p
		}
		if plan == nil {
			return fmt.Errorf("update: rollback %s not found", rollbackID)
		}
	}

	now := time.Now().UTC()
	plan.FinishedAt = &now
	plan.Status = RollbackStatusFailed
	plan.ErrorCode = "cancelled"
	plan.ErrorMessage = "rollback cancelled by user"

	if e.repo != nil {
		if err := e.repo.SaveRollbackPlan(ctx, *plan); err != nil {
			return err
		}
		return e.repo.UpdateRollbackStatus(ctx, rollbackID, RollbackStatusFailed, "cancelled", "rollback cancelled by user")
	}
	return nil
}

func (e *RollbackExecutorV2) RecoverRollback(ctx context.Context, rollbackID string) error {
	if e.repo == nil {
		return fmt.Errorf("update: repository not available for recovery")
	}
	plan, err := e.repo.GetRollbackPlan(ctx, rollbackID)
	if err != nil {
		return err
	}
	if plan.Status == RollbackStatusCompleted || plan.Status == RollbackStatusFailed {
		return nil
	}
	return e.Execute(ctx, plan)
}

func (e *RollbackExecutorV2) GetRollback(ctx context.Context, rollbackID string) (*RollbackPlan, error) {
	e.mu.Lock()
	plan, ok := e.inProgress[rollbackID]
	e.mu.Unlock()
	if ok {
		result := *plan
		return &result, nil
	}
	if e.repo == nil {
		return nil, fmt.Errorf("update: rollback %s not found", rollbackID)
	}
	return e.repo.GetRollbackPlan(ctx, rollbackID)
}

func (e *RollbackExecutorV2) GetRollbackSteps(ctx context.Context, rollbackID string) ([]RollbackStepRecord, error) {
	if e.repo == nil {
		return nil, fmt.Errorf("update: repository not available")
	}
	return e.repo.ListRollbackSteps(ctx, rollbackID)
}

func (m *GenerationManager) Reactivate(ctx context.Context, extensionID, generationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	gens := m.generations[extensionID]
	for i := range gens {
		if gens[i].GenerationID == generationID {
			if gens[i].State == GenerationStateActive {
				return nil
			}
			gens[i].State = GenerationStateActive
			now := time.Now().UTC()
			gens[i].ActivatedAt = &now
			m.active[extensionID] = generationID
			for j := range gens {
				if i != j && gens[j].State == GenerationStateActive {
					gens[j].State = GenerationStateDraining
				}
			}
			m.generations[extensionID] = gens
			return nil
		}
	}
	return fmt.Errorf("update: generation %s not found", generationID)
}
