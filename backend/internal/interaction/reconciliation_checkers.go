package interaction

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/mindruntime"
)

type agentCheckerBase struct {
	processor   AgentReconciliationProcessor
	settleDelay time.Duration
	now         func() time.Time
}

func (b *agentCheckerBase) captureScope(req mindruntime.ReconciliationCheckRequest) ReconciliationCaptureScope {
	scope := ReconciliationCaptureScope{}
	if req.Scope != nil {
		scope.UserID = req.Scope.UserID
		scope.CharacterID = req.Scope.CharacterID
		scope.ConversationID = req.Scope.ConversationID
		scope.InteractionID = req.Scope.InteractionID
		scope.RequestID = req.Scope.RequestID
		scope.GoalIDs = req.Scope.GoalIDs
	}
	return scope
}

func (b *agentCheckerBase) settleReady(observedAt time.Time) bool {
	if observedAt.IsZero() {
		return false
	}
	cutoff := b.now().Add(-b.settleDelay)
	return !observedAt.After(cutoff)
}

type goalActionChecker struct {
	base agentCheckerBase
}

func NewGoalActionChecker(processor AgentReconciliationProcessor, settleDelay time.Duration) mindruntime.ReconciliationChecker {
	return &goalActionChecker{base: newAgentBase(processor, settleDelay)}
}

func (c *goalActionChecker) CheckReconciliation(ctx context.Context, req mindruntime.ReconciliationCheckRequest) ([]mindruntime.ReconciliationDiff, error) {
	if c == nil || c.base.processor == nil {
		return nil, nil
	}
	scope := c.base.captureScope(req)
	if scope.UserID == "" || scope.InteractionID == "" {
		return nil, nil
	}
	snap, err := c.base.processor.Capture(ctx, scope)
	if err != nil || snap == nil {
		return nil, err
	}
	now := c.base.now()
	diffs := make([]mindruntime.ReconciliationDiff, 0)
	for i := range snap.Goals {
		g := snap.Goals[i]
		if g.Status != decision.GoalStatusActive && g.Status != decision.GoalStatusPending {
			continue
		}
		if g.Trigger.InteractionID != scope.InteractionID {
			continue
		}
		hasAction := false
		for j := range snap.Observations {
			if snap.Observations[j].InteractionID == scope.InteractionID &&
				containsGoal(snap.Observations[j].GoalIDs, g.ID) {
				hasAction = true
				break
			}
		}
		if hasAction {
			continue
		}
		if !c.base.settleReady(g.CreatedAt) {
			continue
		}
		diffs = append(diffs, mindruntime.ReconciliationDiff{
			ScanID:         req.ScanID,
			Source:         "agent_goal",
			Target:         "agent_action",
			DiffType:       "missing_action_for_active_goal",
			SourceKey:      g.ID,
			TargetKey:      scope.InteractionID,
			Description:    "active goal has no matching action/observation within settle window",
			Severity:       "warning",
			AutoRepairable: false,
			RepairAction:   "manual_confirm",
			FoundAt:        now,
		})
	}
	return diffs, nil
}

type actionObservationChecker struct {
	base agentCheckerBase
}

func NewActionObservationChecker(processor AgentReconciliationProcessor, settleDelay time.Duration) mindruntime.ReconciliationChecker {
	return &actionObservationChecker{base: newAgentBase(processor, settleDelay)}
}

func (c *actionObservationChecker) CheckReconciliation(ctx context.Context, req mindruntime.ReconciliationCheckRequest) ([]mindruntime.ReconciliationDiff, error) {
	if c == nil || c.base.processor == nil {
		return nil, nil
	}
	scope := c.base.captureScope(req)
	if scope.InteractionID == "" {
		return nil, nil
	}
	snap, err := c.base.processor.Capture(ctx, scope)
	if err != nil || snap == nil {
		return nil, err
	}
	now := c.base.now()
	diffs := make([]mindruntime.ReconciliationDiff, 0)
	for i := range snap.Observations {
		obs := snap.Observations[i]
		if obs.InteractionID != scope.InteractionID {
			continue
		}
		if !c.base.settleReady(obs.ObservedAt) {
			continue
		}
		switch obs.Outcome {
		case decision.ObservationOutcomeSucceeded,
			decision.ObservationOutcomeFailed,
			decision.ObservationOutcomeCancelled,
			decision.ObservationOutcomeTimedOut,
			decision.ObservationOutcomeSkipped,
			decision.ObservationOutcomeNotMaterialized,
			decision.ObservationOutcomeNotDispatched:
			continue
		}
		diffs = append(diffs, mindruntime.ReconciliationDiff{
			ScanID:         req.ScanID,
			Source:         "agent_action",
			Target:         "agent_observation",
			DiffType:       "unfinished_observation_past_settle",
			SourceKey:      obs.ActionID,
			TargetKey:      obs.ID,
			Description:    "observation has not reached a terminal outcome past settle window",
			Severity:       "warning",
			AutoRepairable: false,
			RepairAction:   "manual_confirm",
			FoundAt:        now,
		})
	}
	return diffs, nil
}

type observationGoalChecker struct {
	base agentCheckerBase
}

func NewObservationGoalChecker(processor AgentReconciliationProcessor, settleDelay time.Duration) mindruntime.ReconciliationChecker {
	return &observationGoalChecker{base: newAgentBase(processor, settleDelay)}
}

func (c *observationGoalChecker) CheckReconciliation(ctx context.Context, req mindruntime.ReconciliationCheckRequest) ([]mindruntime.ReconciliationDiff, error) {
	if c == nil || c.base.processor == nil {
		return nil, nil
	}
	scope := c.base.captureScope(req)
	if scope.InteractionID == "" {
		return nil, nil
	}
	snap, err := c.base.processor.Capture(ctx, scope)
	if err != nil || snap == nil {
		return nil, err
	}
	goalByID := make(map[string]decision.Goal, len(snap.Goals))
	for i := range snap.Goals {
		goalByID[snap.Goals[i].ID] = snap.Goals[i]
	}
	now := c.base.now()
	diffs := make([]mindruntime.ReconciliationDiff, 0)
	for i := range snap.Observations {
		obs := snap.Observations[i]
		if obs.InteractionID != scope.InteractionID {
			continue
		}
		for _, gid := range obs.GoalIDs {
			g, ok := goalByID[gid]
			if !ok {
				diffs = append(diffs, mindruntime.ReconciliationDiff{
					ScanID:         req.ScanID,
					Source:         "agent_observation",
					Target:         "agent_goal",
					DiffType:       "observation_references_unknown_goal",
					SourceKey:      obs.ID,
					TargetKey:      gid,
					Description:    "observation goalIds references unknown goal",
					Severity:       "critical",
					AutoRepairable: false,
					RepairAction:   "manual_confirm",
					FoundAt:        now,
				})
				continue
			}
			if revokedGoalStatus(g.Status) {
				diffs = append(diffs, mindruntime.ReconciliationDiff{
					ScanID:         req.ScanID,
					Source:         "agent_observation",
					Target:         "agent_goal",
					DiffType:       "observation_references_revoked_goal",
					SourceKey:      obs.ID,
					TargetKey:      g.ID,
					Description:    "observation references a revoked/abandoned goal",
					Severity:       "warning",
					AutoRepairable: false,
					RepairAction:   "manual_confirm",
					FoundAt:        now,
				})
			}
		}
	}
	return diffs, nil
}

type taskConsistencyChecker struct {
	base agentCheckerBase
}

func NewTaskConsistencyChecker(processor AgentReconciliationProcessor, settleDelay time.Duration) mindruntime.ReconciliationChecker {
	return &taskConsistencyChecker{base: newAgentBase(processor, settleDelay)}
}

func (c *taskConsistencyChecker) CheckReconciliation(ctx context.Context, req mindruntime.ReconciliationCheckRequest) ([]mindruntime.ReconciliationDiff, error) {
	if c == nil || c.base.processor == nil {
		return nil, nil
	}
	scope := c.base.captureScope(req)
	if scope.InteractionID == "" {
		return nil, nil
	}
	snap, err := c.base.processor.Capture(ctx, scope)
	if err != nil || snap == nil {
		return nil, err
	}
	now := c.base.now()
	diffs := make([]mindruntime.ReconciliationDiff, 0)
	for i := range snap.Tasks {
		t := snap.Tasks[i]
		if t.Completed || t.Status == "" {
			continue
		}
		diffs = append(diffs, mindruntime.ReconciliationDiff{
			ScanID:         req.ScanID,
			Source:         "agent_task",
			Target:         "agent_runtime",
			DiffType:       "task_incomplete",
			SourceKey:      t.TaskRunID,
			TargetKey:      t.InvocationID,
			Description:    "task has not reached a completed state",
			Severity:       "warning",
			AutoRepairable: false,
			RepairAction:   "manual_confirm",
			FoundAt:        now,
		})
	}
	return diffs, nil
}

type workflowConsistencyChecker struct {
	base agentCheckerBase
}

func NewWorkflowConsistencyChecker(processor AgentReconciliationProcessor, settleDelay time.Duration) mindruntime.ReconciliationChecker {
	return &workflowConsistencyChecker{base: newAgentBase(processor, settleDelay)}
}

func (c *workflowConsistencyChecker) CheckReconciliation(ctx context.Context, req mindruntime.ReconciliationCheckRequest) ([]mindruntime.ReconciliationDiff, error) {
	if c == nil || c.base.processor == nil {
		return nil, nil
	}
	scope := c.base.captureScope(req)
	if scope.InteractionID == "" {
		return nil, nil
	}
	snap, err := c.base.processor.Capture(ctx, scope)
	if err != nil || snap == nil {
		return nil, err
	}
	now := c.base.now()
	diffs := make([]mindruntime.ReconciliationDiff, 0)
	for i := range snap.Workflows {
		w := snap.Workflows[i]
		if w.Completed || w.Status == "" {
			continue
		}
		if w.Attempts > 5 {
			diffs = append(diffs, mindruntime.ReconciliationDiff{
				ScanID:         req.ScanID,
				Source:         "agent_workflow",
				Target:         "agent_runtime",
				DiffType:       "workflow_excessive_attempts",
				SourceKey:      w.ExecutionID,
				TargetKey:      w.WorkflowID,
				Description:    "workflow has excessive retry attempts without completion",
				Severity:       "critical",
				AutoRepairable: false,
				RepairAction:   "manual_confirm",
				FoundAt:        now,
			})
		}
	}
	return diffs, nil
}

type runtimeInvocationChecker struct {
	base agentCheckerBase
}

func NewInvocationConsistencyChecker(processor AgentReconciliationProcessor, settleDelay time.Duration) mindruntime.ReconciliationChecker {
	return &runtimeInvocationChecker{base: newAgentBase(processor, settleDelay)}
}

func (c *runtimeInvocationChecker) CheckReconciliation(ctx context.Context, req mindruntime.ReconciliationCheckRequest) ([]mindruntime.ReconciliationDiff, error) {
	if c == nil || c.base.processor == nil {
		return nil, nil
	}
	scope := c.base.captureScope(req)
	if scope.InteractionID == "" {
		return nil, nil
	}
	snap, err := c.base.processor.Capture(ctx, scope)
	if err != nil || snap == nil {
		return nil, err
	}
	now := c.base.now()
	diffs := make([]mindruntime.ReconciliationDiff, 0)
	for i := range snap.Invocations {
		inv := snap.Invocations[i]
		if inv.Completed || inv.Status == "" {
			continue
		}
		diffs = append(diffs, mindruntime.ReconciliationDiff{
			ScanID:         req.ScanID,
			Source:         "agent_runtime",
			Target:         "agent_invocation",
			DiffType:       "invocation_incomplete",
			SourceKey:      inv.InvocationID,
			TargetKey:      inv.CapabilityID,
			Description:    "invocation has not reached a completed state",
			Severity:       "warning",
			AutoRepairable: false,
			RepairAction:   "manual_confirm",
			FoundAt:        now,
		})
	}
	return diffs, nil
}

func newAgentBase(processor AgentReconciliationProcessor, settleDelay time.Duration) agentCheckerBase {
	if settleDelay <= 0 {
		settleDelay = mindruntime.DefaultAgentFactSettleDelay()
	}
	return agentCheckerBase{
		processor:   processor,
		settleDelay: settleDelay,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func containsGoal(goals []string, goalID string) bool {
	for _, g := range goals {
		if g == goalID {
			return true
		}
	}
	return false
}

func revokedGoalStatus(s decision.GoalStatus) bool {
	switch s {
	case decision.GoalStatusAbandoned, decision.GoalStatusAchieved, decision.GoalStatusSuspended:
		return true
	}
	return false
}
